package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"
)

func TestSourceRefProjectionSealAndFailure(t *testing.T) {
	ctx := context.Background()
	s, err := NewSQLiteStore(filepath.Join(t.TempDir(), "refs.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	meta := ServiceMeta{ServiceID: "svc", Path: "/root", IndexedCommit: "head", IndexedAt: 1}
	h := sha256.Sum256([]byte("raw"))
	row := SourceRefFact{Path: "a.txt", SourceHash: hex.EncodeToString(h[:]), ExtractorVersion: SourceRefExtractorVersion, Refs: []string{}}
	p := &SourceRefProjection{Policy: "policy", ExtractorVersion: SourceRefExtractorVersion, ProducedAt: 1, Complete: true, Rows: []SourceRefFact{row}}
	if err := s.WriteServiceSummary(ctx, meta, ServiceSummary{RefProjection: p}, []FileStat{{Path: "a.txt"}}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.ReadSourceRefProjection(ctx, "svc")
	if err != nil || !ok || !got.Complete || got.Rows[0].Refs == nil {
		t.Fatalf("sealed projection=%+v ok=%v err=%v", got, ok, err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE source_ref_facts SET extractor_version='older' WHERE service_id='svc'`); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.ReadSourceRefProjection(ctx, "svc"); err != nil || ok {
		t.Fatalf("mixed fact version visible: ok=%v err=%v", ok, err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE source_ref_facts SET extractor_version=? WHERE service_id='svc'`, SourceRefExtractorVersion); err != nil {
		t.Fatal(err)
	}
	if err := s.SetIngestState(ctx, IngestState{ServiceID: "svc", State: IngestInProgress}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.ReadSourceRefProjection(ctx, "svc"); err != nil || ok {
		t.Fatalf("mixed ingest visible: ok=%v err=%v", ok, err)
	}
	if err := s.DeleteIngestState(ctx, "svc"); err != nil {
		t.Fatal(err)
	}
	bad := &SourceRefProjection{Policy: "policy", ExtractorVersion: SourceRefExtractorVersion, Complete: true, Rows: []SourceRefFact{{Path: "b.txt", ExtractorVersion: SourceRefExtractorVersion}}}
	meta.IndexedAt = 2
	if err := s.WriteServiceSummary(ctx, meta, ServiceSummary{RefProjection: bad}, nil); err == nil {
		t.Fatal("invalid transaction accepted")
	}
	got, ok, err = s.ReadSourceRefProjection(ctx, "svc")
	if err != nil || !ok || got.Watermark != WatermarkOf(ServiceMeta{IndexedCommit: "head", IndexedAt: 1}) {
		t.Fatalf("failed transaction changed seal: %+v ok=%v err=%v", got, ok, err)
	}
	if err := s.SetServiceMeta(ctx, meta); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.ReadSourceRefProjection(ctx, "svc"); err != nil || ok {
		t.Fatalf("meta-only fallback exposed stale seal: ok=%v err=%v", ok, err)
	}
}

func TestSourceRefProjectionRefusesTamperedSealAndExclusions(t *testing.T) {
	ctx := context.Background()
	s, err := NewSQLiteStore(filepath.Join(t.TempDir(), "refs.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h := sha256.Sum256([]byte("[FR-1]"))
	meta := ServiceMeta{ServiceID: "svc", Path: "/root", IndexedCommit: "head", IndexedAt: 1}
	rows := []SourceRefFact{
		{Path: "a.go", SourceHash: hex.EncodeToString(h[:]), ExtractorVersion: SourceRefExtractorVersion, Refs: []string{"[FR-1]"}},
		{Path: "empty.go", SourceHash: hex.EncodeToString(h[:]), ExtractorVersion: SourceRefExtractorVersion, Refs: []string{}},
		{Path: "large.dat", ExtractorVersion: SourceRefExtractorVersion, Excluded: "oversize"},
	}
	p := &SourceRefProjection{Policy: "policy", ExtractorVersion: SourceRefExtractorVersion, Rows: rows}
	files := []FileStat{{Path: "a.go"}, {Path: "empty.go"}, {Path: "large.dat", Excluded: "oversize"}}
	if err := s.WriteServiceSummary(ctx, meta, ServiceSummary{RefProjection: p}, files); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := s.ReadSourceRefProjection(ctx, "svc"); err != nil || !ok || got.Complete || got.Rows[1].Refs == nil {
		t.Fatalf("valid excluded projection missing: %+v ok=%v err=%v", got, ok, err)
	}
	for _, tc := range []struct{ name, update, restore string }{
		{"root", `UPDATE source_ref_seals SET root='/elsewhere'`, `UPDATE source_ref_seals SET root='/root'`},
		{"watermark", `UPDATE source_ref_seals SET watermark='stale'`, `UPDATE source_ref_seals SET watermark='head@1'`},
		{"policy", `UPDATE source_ref_seals SET policy=''`, `UPDATE source_ref_seals SET policy='policy'`},
		{"extractor", `UPDATE source_ref_seals SET extractor_version='old'`, `UPDATE source_ref_seals SET extractor_version='sdd-default-refs-v1'`},
		{"membership", `UPDATE source_ref_seals SET membership_hash='bad'`, `UPDATE source_ref_seals SET membership_hash='` + SourceRefMembershipHash(rows) + `'`},
		{"raw hash", `UPDATE source_ref_facts SET source_hash='bad' WHERE path='a.go'`, `UPDATE source_ref_facts SET source_hash='` + rows[0].SourceHash + `' WHERE path='a.go'`},
		{"file stats exclusion", `UPDATE file_stats SET excluded='' WHERE path='large.dat'`, `UPDATE file_stats SET excluded='oversize' WHERE path='large.dat'`},
		{"file stats membership", `DELETE FROM file_stats WHERE path='empty.go'`, `INSERT INTO file_stats(service_id,path,size,mtime_ns,ino,content_hash,lang,excluded) VALUES('svc','empty.go',0,0,0,'','','')`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.db.ExecContext(ctx, tc.update); err != nil {
				t.Fatal(err)
			}
			if _, ok, err := s.ReadSourceRefProjection(ctx, "svc"); err != nil || ok {
				t.Fatalf("tampered projection visible: ok=%v err=%v", ok, err)
			}
			if _, err := s.db.ExecContext(ctx, tc.restore); err != nil {
				t.Fatal(err)
			}
		})
	}
}
