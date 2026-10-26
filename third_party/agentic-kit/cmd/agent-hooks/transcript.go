package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

// maxTranscriptBytes bounds the size of a subagent transcript that report
// will scan for token usage; maxTranscriptLine bounds one record in it.
const (
	maxTranscriptBytes = 64 << 20
	maxTranscriptLine  = 16 << 20
)

// transcriptUsage is the token total and model a subagent transcript records.
type transcriptUsage struct {
	Model  string
	Tokens int64
}

// readTranscriptUsage totals a finished subagent's tokens from its
// transcript, at report time. Async Claude Code agents report nothing at
// launch, so this is the only place their spend is visible. The path came
// from the host payload: only an absolute path to a regular file within the size cap is
// read, and only to sum numbers. ok is false when nothing usable was found.
func readTranscriptUsage(host, path string) (transcriptUsage, bool) {
	if path == "" || !filepath.IsAbs(path) {
		return transcriptUsage{}, false
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxTranscriptBytes {
		return transcriptUsage{}, false
	}
	f, err := os.Open(path)
	if err != nil {
		return transcriptUsage{}, false
	}
	defer f.Close()
	var u transcriptUsage
	var found bool
	var readErr error
	if host == hostCodex {
		u, found, readErr = codexUsage(f)
	} else {
		u, found, readErr = claudeUsage(f)
	}
	if readErr != nil {
		return transcriptUsage{}, false // a partial total must not pass for a complete one
	}
	return u, found
}

// claudeUsage sums usage once per assistant message id: Claude Code writes
// one transcript line per content block, each repeating the message usage.
func claudeUsage(r io.Reader) (transcriptUsage, bool, error) {
	type usage struct {
		Input         int64 `json:"input_tokens"`
		CacheCreation int64 `json:"cache_creation_input_tokens"`
		CacheRead     int64 `json:"cache_read_input_tokens"`
		Output        int64 `json:"output_tokens"`
	}
	var line struct {
		Message struct {
			ID    string `json:"id"`
			Model string `json:"model"`
			Usage *usage `json:"usage"`
		} `json:"message"`
	}
	perMessage := map[string]int64{}
	var u transcriptUsage
	err := eachLine(r, maxTranscriptLine, func(b []byte) {
		line.Message.ID, line.Message.Model, line.Message.Usage = "", "", nil
		if json.Unmarshal(b, &line) != nil || line.Message.Usage == nil {
			return
		}
		us := line.Message.Usage
		total := us.Input + us.CacheCreation + us.CacheRead + us.Output
		if total > perMessage[line.Message.ID] {
			perMessage[line.Message.ID] = total // streaming lines only grow
		}
		if line.Message.Model != "" {
			u.Model = line.Message.Model
		}
	})
	for _, t := range perMessage {
		u.Tokens += t
	}
	return u, len(perMessage) > 0, err
}

// codexUsage takes the last cumulative token_count and the last model.
func codexUsage(r io.Reader) (transcriptUsage, bool, error) {
	var line struct {
		Type    string `json:"type"`
		Payload struct {
			Type  string `json:"type"`
			Model string `json:"model"`
			Info  *struct {
				Total struct {
					Tokens int64 `json:"total_tokens"`
				} `json:"total_token_usage"`
			} `json:"info"`
		} `json:"payload"`
	}
	var u transcriptUsage
	found := false
	err := eachLine(r, maxTranscriptLine, func(b []byte) {
		line.Type, line.Payload.Type, line.Payload.Info, line.Payload.Model = "", "", nil, ""
		if json.Unmarshal(b, &line) != nil {
			return
		}
		switch {
		case line.Type == "turn_context" && line.Payload.Model != "":
			u.Model = line.Payload.Model
		case line.Payload.Type == "token_count" && line.Payload.Info != nil:
			u.Tokens, found = line.Payload.Info.Total.Tokens, true
		}
	})
	return u, found, err
}
