package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

func init() {
	offlineHarnesses["oracle"] = offlineHarnessFactory{
		help: "verified gold patch or gold files, no spend",
		new: func(cfg offlineHarnessConfig) scenariobench.Harness {
			var source scenariobench.SourcePin
			// Multiple pins cannot be guessed. FetchGoldPatch refuses an absent pin on F2.
			if len(cfg.Corpus.Sources) == 1 {
				source = cfg.Corpus.Sources[0]
			}
			var fetch scenariobench.GoldPatchFetcher
			if cfg.PatchDir != "" {
				fetch = func(_ context.Context, _ scenariobench.SourcePin, id string) ([]byte, error) {
					for _, task := range cfg.Corpus.Tasks {
						if task.ID == id {
							// Validate the digest before using it as a local file name.
							if len(task.Gold.PatchSHA) != 64 {
								return nil, fmt.Errorf("invalid patch sha256")
							}
							if _, err := hex.DecodeString(task.Gold.PatchSHA); err != nil {
								return nil, fmt.Errorf("invalid patch sha256")
							}
							return os.ReadFile(filepath.Join(cfg.PatchDir, task.Gold.PatchSHA+".patch"))
						}
					}
					return nil, fmt.Errorf("unknown task %q", id)
				}
			}
			return scenariobench.NewOracleHarness(source, cfg.CacheDir, fetch)
		}}
}
