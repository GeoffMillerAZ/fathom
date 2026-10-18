package readout

import (
	"os"
	"path/filepath"
)

// LoadFile loads one rule file alone, without the built-ins or the user's
// files, held to the same schema and contract as LoadRules: what `fathom
// readout test FILE` and `readout replay --rules FILE` read. A fixture's
// input_file resolves beside the file. The error is operational only (the
// file unreadable); a file that does not parse is a Refusal with no ID.
func LoadFile(path string) (*RuleSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	rs := &RuleSet{}
	rs.addFile(path, data, os.DirFS(filepath.Dir(path)))
	return rs, nil
}
