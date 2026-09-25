package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/geoffmilleraz/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/app-kit/internal/designassets"
	"github.com/geoffmilleraz/app-kit/internal/designconvert"
)

// Elements has a separate opt-in receipt family. It neither mutates the legacy
// syntax store nor installs the declared runtime or binds any application action.
func dataAssetsTranslateElements(args []string) error {
	fs := flag.NewFlagSet("assets-translate-elements", flag.ContinueOnError)
	root := fs.String("root", defaultDesignAssetsRoot, "private original archive/catalog directory")
	id := fs.String("id", "", "exact original archive ID from assets-list")
	member := fs.String("path", "", "exact HTML member; selected Elements grammar only")
	if err := fs.Parse(args); err != nil {
		return command.Exit(2, err)
	}
	if *id == "" || *member == "" || fs.NArg() != 0 {
		return command.Exit(2, fmt.Errorf("usage: appkit data assets-translate-elements -id ARCHIVE_ID -path MEMBER [-root PRIVATE_DIRECTORY]; emits private syntax JSON only, no save or runtime installation"))
	}
	original, err := designassets.ReadFile(*root, *id, *member)
	if err != nil {
		return command.Exit(2, err)
	}
	result, err := designconvert.TranslateElements(designconvert.Source{
		ArchiveID: original.ArchiveID, Path: original.Path, SHA256: original.SHA256, Size: len(original.Content),
	}, []byte(original.Content))
	if err != nil {
		return command.Exit(2, err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		return command.Exit(2, err)
	}
	if result.Status == "unsupported" {
		return command.Exit(1, fmt.Errorf("source is unsupported by the Elements syntax rule; see JSON diagnostics, original remains available"))
	}
	return nil
}
