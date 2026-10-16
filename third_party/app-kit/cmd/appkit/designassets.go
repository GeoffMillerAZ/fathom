package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
)

const defaultDesignAssetsRoot = "data/design-assets"

func dataAssetsImport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("assets-import", flag.ContinueOnError)
	src := fs.String("src", "", "directory of official Tailwind Plus ZIP downloads; originals remain unchanged")
	root := fs.String("root", defaultDesignAssetsRoot, "private archive/catalog directory; never installs or executes projects")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *src == "" || fs.NArg() != 0 {
		return fmt.Errorf("usage: appkit data assets-import -src DOWNLOAD_DIRECTORY [-root PRIVATE_DIRECTORY]")
	}
	c, err := designassets.Import(ctx, *src, *root)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Revision string `json:"revision"`
		Archives int    `json:"archives"`
		Root     string `json:"root"`
	}{c.Revision, len(c.List()), *root})
}

func dataAssetsList(args []string) error {
	fs := flag.NewFlagSet("assets-list", flag.ContinueOnError)
	root := fs.String("root", defaultDesignAssetsRoot, "private archive/catalog directory")
	query := fs.String("query", "", "words in archive metadata and member paths")
	kind := fs.String("kind", "", "template, ui-kit, or ui-blocks")
	family := fs.String("family", "", "exact family from a prior result")
	limit := fs.Int("limit", 10, "page size, 1–50")
	cursor := fs.String("cursor", "", "next_cursor from the same query")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("assets-list accepts flags only")
	}
	c, err := designassets.Load(*root)
	if err != nil {
		return err
	}
	page, err := c.Search(designassets.Query{Text: *query, Kind: *kind, Family: *family, Limit: *limit, Cursor: *cursor})
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(page)
}

func dataAssetsFiles(args []string) error {
	fs := flag.NewFlagSet("assets-files", flag.ContinueOnError)
	root := fs.String("root", defaultDesignAssetsRoot, "private archive/catalog directory")
	id := fs.String("id", "", "exact archive ID from assets-list")
	prefix := fs.String("prefix", "", "archive-relative member path prefix")
	kind := fs.String("kind", "", "exact file kind from a prior result")
	limit := fs.Int("limit", 10, "page size, 1–50")
	cursor := fs.String("cursor", "", "next_cursor from the same query")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" || fs.NArg() != 0 {
		return fmt.Errorf("usage: appkit data assets-files -id ARCHIVE_ID [flags]")
	}
	c, err := designassets.Load(*root)
	if err != nil {
		return err
	}
	page, err := c.FilesPage(designassets.FileQuery{ID: *id, Prefix: *prefix, Kind: *kind, Limit: *limit, Cursor: *cursor})
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(page)
}

func dataAssetsGet(args []string) error {
	fs := flag.NewFlagSet("assets-get", flag.ContinueOnError)
	root := fs.String("root", defaultDesignAssetsRoot, "private archive/catalog directory")
	id := fs.String("id", "", "exact archive ID from assets-list")
	path := fs.String("path", "", "exact text member path from assets-files; omit for verified project metadata")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" || fs.NArg() != 0 {
		return fmt.Errorf("usage: appkit data assets-get -id ARCHIVE_ID [-path MEMBER] [flags]")
	}
	if *path != "" {
		f, err := designassets.ReadFile(*root, *id, *path)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(f)
	}
	a, err := designassets.Verify(*root, *id)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Asset    designassets.Summary   `json:"asset"`
		Projects []designassets.Project `json:"projects"`
	}{designassets.Summarize(a), a.Projects})
}
