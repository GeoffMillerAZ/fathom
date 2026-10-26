package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

type locatedEntity struct {
	Kind  string `json:"kind"`
	Name  string `json:"name"`
	Scope string `json:"scope,omitempty"`
	Line  int    `json:"line"`
	End   int    `json:"end"`
}

// locatedWritten supplies C positions only, never the native population.
func locatedWritten(source []byte) ([]locatedEntity, error) {
	if len(source) > 1<<20 {
		return nil, fmt.Errorf("whole-input location source cap exceeded")
	}
	var root yaml.Node
	if err := yaml.Unmarshal(source, &root); err != nil {
		return nil, err
	}
	rows := []locatedEntity{}
	if len(root.Content) == 0 {
		return rows, nil
	}
	entities, _ := walkWritten(root.Content[0])
	for _, e := range entities {
		rows = append(rows, locatedEntity{e.kind, e.name, e.scope, e.line, e.end})
	}
	return rows, nil
}
func runLocations(sourcePath string) error {
	input, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer input.Close()
	source, err := io.ReadAll(io.LimitReader(input, (1<<20)+1))
	if err != nil {
		return err
	}
	rows, err := locatedWritten(source)
	if err != nil {
		return err
	}
	output := struct {
		Tier     string          `json:"tier"`
		Entities []locatedEntity `json:"entities"`
	}{"C", rows}
	data, err := json.Marshal(output)
	if err != nil {
		return err
	}
	if len(data) > 8<<20 {
		return fmt.Errorf("whole-input location output cap exceeded")
	}
	_, err = os.Stdout.Write(append(data, '\n'))
	return err
}
