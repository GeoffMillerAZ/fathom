package lang

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

// levels.json is generated from docs/bench/readings/lang/*.json by
// levels_gen.go (go generate); TestLevelsTableMatchesReadings holds the two
// together.
//
//go:embed levels.json
var levelsJSON []byte

type levelRow struct {
	ID    string `json:"id"`
	Level string `json:"level"`
	Grade string `json:"grade"`
}

var (
	levelsOnce sync.Once
	levelsByID map[string]levelRow
	levelsErr  error
)

func loadLevels() (map[string]levelRow, error) {
	levelsOnce.Do(func() {
		var rows []levelRow
		if err := json.Unmarshal(levelsJSON, &rows); err != nil {
			levelsErr = fmt.Errorf("lang: measured-level table: %w", err)
			return
		}
		levelsByID = make(map[string]levelRow, len(rows))
		for _, r := range rows {
			levelsByID[r.ID] = r
		}
	})
	return levelsByID, levelsErr
}

// levelOf is the measured level and grade of a dialect, LevelUnmeasured when
// no reading exists.
func levelOf(levels map[string]levelRow, id string) (level, grade string) {
	if r, ok := levels[id]; ok {
		return r.Level, r.Grade
	}
	return LevelUnmeasured, ""
}
