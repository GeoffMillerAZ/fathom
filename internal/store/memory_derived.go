package store

// memory_derived.go — the memory backend's derived facts (RT-76, RT-77,
// RT-160; derived.go is the SQLite design). The call index is computed
// from the observations on every CallersOfKeys (a test fake has no table
// to keep in step), so every service is always call-indexed here.

import (
	"context"
	"sort"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// WriteServiceSummary records meta, replaces file_stats and writes the
// summary with counts taken from the stored observations, atomically
// under the store lock.
func (m *MemoryStore) WriteServiceSummary(_ context.Context, meta ServiceMeta, sum ServiceSummary, files []FileStat) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.serviceMeta[meta.ServiceID] = meta
	if m.fileStats == nil {
		m.fileStats = map[string][]FileStat{}
	}
	if len(files) > 0 {
		fs := append([]FileStat(nil), files...)
		sort.Slice(fs, func(i, j int) bool { return fs[i].Path < fs[j].Path })
		m.fileStats[meta.ServiceID] = fs
	} else {
		delete(m.fileStats, meta.ServiceID)
	}
	sum.ServiceID = meta.ServiceID
	sum.Watermark = WatermarkOf(meta)
	sum.Source = SummaryFromIngest
	if sum.WrittenAt == 0 {
		sum.WrittenAt = time.Now().UnixNano()
	}
	sum.SymbolCount, sum.SourceKinds = m.symbolAggregateLocked(meta.ServiceID)
	m.putSummaryLocked(sum)
	return nil
}

// RecountServiceSummary recounts the summary's symbols in place.
func (m *MemoryStore) RecountServiceSummary(_ context.Context, serviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sum, ok := m.summaries[serviceID]
	if !ok {
		return nil
	}
	sum.SymbolCount, sum.SourceKinds = m.symbolAggregateLocked(serviceID)
	m.summaries[serviceID] = sum
	return nil
}

func (m *MemoryStore) putSummaryLocked(sum ServiceSummary) {
	if m.summaries == nil {
		m.summaries = map[string]ServiceSummary{}
	}
	sum.HasFileStats = false
	m.summaries[sum.ServiceID] = sum
}

func (m *MemoryStore) symbolAggregateLocked(serviceID string) (int, []core.SourceKind) {
	kinds := map[core.SourceKind]bool{}
	n := 0
	for _, o := range m.observations {
		if o.ServiceID == serviceID {
			n++
			kinds[o.SourceKind] = true
		}
	}
	var out []core.SourceKind
	for k := range kinds {
		out = append(out, k)
	}
	sortKinds(out)
	return n, out
}

func sortKinds(k []core.SourceKind) {
	sort.Slice(k, func(i, j int) bool { return k[i] < k[j] })
}

func (m *MemoryStore) serviceSummaryRaw(serviceID string) (ServiceSummary, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sum, ok := m.summaries[serviceID]
	if !ok {
		return ServiceSummary{}, false
	}
	sum.SourceKinds = append([]core.SourceKind(nil), sum.SourceKinds...)
	sum.HasFileStats = len(m.fileStats[serviceID]) > 0
	return sum, true
}

func (m *MemoryStore) fileStatsRaw(serviceID string) []FileStat {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]FileStat(nil), m.fileStats[serviceID]...)
}

func (m *MemoryStore) callersOfKeysRaw(keys []string) []CallerRow {
	want := map[string]bool{}
	for _, k := range keys {
		if k != "" {
			want[k] = true
		}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []CallerRow
	for _, o := range m.observations {
		for _, c := range o.Calls {
			if want[c] || want[CalleeName(c)] {
				out = append(out, CallerRow{ServiceID: o.ServiceID, Symbol: o.Symbol, Package: o.Package, SourceRef: o.SourceRef, Line: o.Line, CalleeKey: c})
			}
		}
	}
	sortCallerRows(out)
	return dedupeCallerRows(out)
}

// DerivedState reports the memory backend's derived facts; calls are
// always indexed (computed per call).
func (m *MemoryStore) DerivedState(_ context.Context, serviceID string) (DerivedState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sum, ok := m.summaries[serviceID]
	st := DerivedState{HasSummary: ok, SummaryWatermark: sum.Watermark, CallsIndexed: true, HasFileStats: len(m.fileStats[serviceID]) > 0}
	if meta, found := m.serviceMeta[serviceID]; found && ok {
		st.SummaryCurrent = sum.Watermark == WatermarkOf(meta)
	}
	return st, nil
}

// BackfillDerivedTables derives a missing or stale summary from the stored
// observations and fingerprinted paths (names only).
func (m *MemoryStore) BackfillDerivedTables(_ context.Context, serviceID string, opts BackfillOptions) (BackfillReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var rep BackfillReport
	if !opts.Summary {
		return rep, nil
	}
	if is, ok := m.ingestState[serviceID]; ok && is.State == IngestInProgress {
		rep.IngestInProgress = true
		return rep, nil
	}
	meta, found := m.serviceMeta[serviceID]
	if !found {
		return rep, nil
	}
	cur := WatermarkOf(meta)
	if sum, ok := m.summaries[serviceID]; ok && sum.Watermark == cur {
		return rep, nil
	}
	sum := ServiceSummary{ServiceID: serviceID, Watermark: cur, Source: SummaryFromBackfill, Bytes: -1, WrittenAt: time.Now().UnixNano()}
	sum.SymbolCount, sum.SourceKinds = m.symbolAggregateLocked(serviceID)
	for k := range m.fingerprints {
		if k.serviceID != serviceID {
			continue
		}
		sum.FileCount++
		if opts.Classify != nil {
			if lang, ok := opts.Classify(k.sourceRef); ok {
				if sum.UnsupportedLangs == nil {
					sum.UnsupportedLangs = map[string]int{}
				}
				sum.UnsupportedLangs[lang]++
			}
		}
	}
	rep.Derived = &sum
	if opts.NoWrite {
		return rep, nil
	}
	m.putSummaryLocked(sum)
	rep.Summary = true
	return rep, nil
}

// dropDerivedLocked forgets serviceID's derived rows; reports whether any
// existed.
func (m *MemoryStore) dropDerivedLocked(serviceID string) bool {
	_, a := m.summaries[serviceID]
	_, b := m.fileStats[serviceID]
	_, c := m.serviceRefs[serviceID]
	delete(m.summaries, serviceID)
	delete(m.fileStats, serviceID)
	delete(m.serviceRefs, serviceID)
	return a || b || c
}

func (m *MemoryStore) renameDerivedLocked(oldID, newID string) {
	if sr, ok := m.serviceRefs[oldID]; ok {
		delete(m.serviceRefs, oldID)
		m.serviceRefs[newID] = sr
	}
	if sum, ok := m.summaries[oldID]; ok {
		delete(m.summaries, oldID)
		sum.ServiceID = newID
		m.summaries[newID] = sum
	}
	if fs, ok := m.fileStats[oldID]; ok {
		delete(m.fileStats, oldID)
		m.fileStats[newID] = fs
	}
}
