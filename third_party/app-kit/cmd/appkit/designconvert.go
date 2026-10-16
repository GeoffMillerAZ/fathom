package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designattempts"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designconvert"
)

type savedTranslationOutput struct {
	ID          string                    `json:"id"`
	Translation designconvert.Translation `json:"translation"`
}

// dataAssetsTranslate assesses one exact original. It is read-only unless -save
// explicitly persists an attempt; originals are never edited. Unsupported emits
// JSON and exit 1; usage/operational failures carry exit 2.
func dataAssetsTranslate(args []string) error {
	fs := flag.NewFlagSet("assets-translate", flag.ContinueOnError)
	root := fs.String("root", defaultDesignAssetsRoot, "private original archive/catalog directory")
	id := fs.String("id", "", "exact original archive ID from assets-list")
	member := fs.String("path", "", "exact member path; only bounded HTML syntax is currently translated")
	target := fs.String("target", "go-templ", "syntax target (go-templ only); never installs or executes vendor projects")
	save := fs.Bool("save", false, "persist this private syntax attempt and emit its ID with the translation; never target certification")
	if err := fs.Parse(args); err != nil {
		return command.Exit(2, err)
	}
	if *id == "" || *member == "" || *target != "go-templ" || fs.NArg() != 0 {
		return command.Exit(2, fmt.Errorf("usage: appkit data assets-translate -id ARCHIVE_ID -path MEMBER [-target go-templ] [-save] [-root PRIVATE_DIRECTORY]"))
	}
	original, err := designassets.ReadFile(*root, *id, *member)
	if err != nil {
		return command.Exit(2, err)
	}
	result, err := designconvert.Translate(designconvert.Source{
		ArchiveID: original.ArchiveID, Path: original.Path, SHA256: original.SHA256, Size: len(original.Content),
	}, []byte(original.Content))
	if err != nil {
		return command.Exit(2, err)
	}
	var output any = result
	if *save {
		entry, err := designattempts.Publish(*root, result)
		if err != nil {
			return command.Exit(2, err)
		}
		output = savedTranslationOutput{ID: entry.ID, Translation: result}
	}
	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		return command.Exit(2, err)
	}
	if result.Status == "unsupported" {
		return command.Exit(1, fmt.Errorf("source is unsupported by the selected syntax rule; see JSON diagnostics, original remains available"))
	}
	return nil
}

func dataAssetsConversions(args []string) error {
	fs := flag.NewFlagSet("assets-conversions", flag.ContinueOnError)
	root := fs.String("root", defaultDesignAssetsRoot, "private original archive/catalog directory")
	id := fs.String("id", "", "filter by exact original archive ID")
	member := fs.String("path", "", "filter by exact original member path")
	status := fs.String("status", "", "filter syntax status (translated or unsupported)")
	limit := fs.Int("limit", 10, "maximum attempts per page")
	cursor := fs.String("cursor", "", "continuation cursor from the same query and catalog revision")
	if err := fs.Parse(args); err != nil {
		return command.Exit(2, err)
	}
	if fs.NArg() != 0 {
		return command.Exit(2, fmt.Errorf("usage: appkit data assets-conversions [-id ARCHIVE_ID] [-path MEMBER] [-status STATUS] [-limit N] [-cursor CURSOR] [-root PRIVATE_DIRECTORY]"))
	}
	catalog, err := designattempts.Load(*root)
	if err != nil {
		return command.Exit(2, err)
	}
	page, err := catalog.Search(designattempts.Query{ArchiveID: *id, Path: *member, Status: *status, Limit: *limit, Cursor: *cursor})
	if err != nil {
		return command.Exit(2, err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(page); err != nil {
		return command.Exit(2, err)
	}
	return nil
}

func dataAssetsConversion(args []string) error {
	fs := flag.NewFlagSet("assets-conversion", flag.ContinueOnError)
	root := fs.String("root", defaultDesignAssetsRoot, "private original archive/catalog directory")
	id := fs.String("id", "", "exact saved attempt ID (syntax@SHA256)")
	if err := fs.Parse(args); err != nil {
		return command.Exit(2, err)
	}
	if *id == "" || fs.NArg() != 0 {
		return command.Exit(2, fmt.Errorf("usage: appkit data assets-conversion -id ATTEMPT_ID [-root PRIVATE_DIRECTORY]"))
	}
	detail, err := designattempts.Read(*root, *id)
	if err != nil {
		return command.Exit(2, err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(detail); err != nil {
		return command.Exit(2, err)
	}
	return nil
}
