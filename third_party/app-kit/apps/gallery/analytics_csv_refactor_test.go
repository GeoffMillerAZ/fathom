package gallery

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"testing"
)

func TestBuildAnalyticsCSVAllSelectionsDigest(t *testing.T) {
	h := sha256.New()
	count := 0
	add := func(body []byte) {
		// ADR-0056 appends role identity/meaning. Verify those independently, then
		// preserve the old 144-export digest over every established column.
		records, err := csv.NewReader(bytes.NewReader(body)).ReadAll()
		if err != nil || len(records) == 0 || len(records[0]) != 33 || records[0][31] != "role" || records[0][32] != "role_label" {
			t.Fatal("expected the reviewed 33-column export")
		}
		for i, row := range records {
			if i > 0 {
				wantRole, wantLabel := "accent", "API services"
				if row[23] == "report-worker" {
					wantRole, wantLabel = "info", "Worker services"
				}
				if row[31] != wantRole || row[32] != wantLabel {
					t.Fatalf("lost service identity/meaning: %v", row)
				}
			}
			records[i] = row[:31]
		}
		var legacy bytes.Buffer
		w := csv.NewWriter(&legacy)
		w.WriteAll(records)
		if err := w.Error(); err != nil {
			t.Fatal(err)
		}
		body = legacy.Bytes()
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(body)))
		_, _ = h.Write(size[:])
		_, _ = h.Write(body)
		count++
	}
	for _, period := range []analyticsPeriod{analyticsPeriodAugust, analyticsPeriodSeptember} {
		for _, compare := range []analyticsComparison{analyticsCompareNone, analyticsComparePrevious} {
			for _, category := range []analyticsCategory{analyticsCategoryAll, analyticsCategoryAPI, analyticsCategoryWorker} {
				for _, state := range []analyticsState{analyticsStateComplete, analyticsStateMissing, analyticsStateStale, analyticsStatePartial} {
					j, err := buildAnalyticsJourney(analyticsSelection{period, compare, category, state})
					if err != nil {
						t.Fatal(err)
					}
					body, err := buildAnalyticsCSV(j, "")
					if err != nil {
						t.Fatal(err)
					}
					add(body)
					for _, detail := range j.Details {
						body, err = buildAnalyticsCSV(analyticsServiceExport(j, detail), detail.ID)
						if err != nil {
							t.Fatal(err)
						}
						add(body)
					}
				}
			}
		}
	}
	got := fmt.Sprintf("%x", h.Sum(nil))
	const want = "f8747c6ec1f964bb8cb6d9af9a6def1ee94612c1f3ae4edcbd3b3962c9279afe"
	if count != 144 || got != want {
		t.Fatalf("analytics CSV corpus changed: count=%d digest=%s", count, got)
	}
}
