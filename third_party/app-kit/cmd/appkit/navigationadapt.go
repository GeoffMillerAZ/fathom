package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designadapt"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designconvert"
)

func dataAssetsAdaptNavigation(args []string) error {
	fs := flag.NewFlagSet("assets-adapt-navigation", flag.ContinueOnError)
	root := fs.String("root", defaultDesignAssetsRoot, "private archive/catalog directory")
	out := fs.String("out", "", "export private sources to a NEW directory under an existing real parent")
	check := fs.String("check", "", "read-only exact drift check of a private export (macOS/Linux)")
	if err := fs.Parse(args); err != nil {
		return command.Exit(2, err)
	}
	var outSet, checkSet bool
	fs.Visit(func(f *flag.Flag) {
		outSet = outSet || f.Name == "out"
		checkSet = checkSet || f.Name == "check"
	})
	if fs.NArg() != 0 || *root == "" || outSet && checkSet || outSet && *out == "" || checkSet && *check == "" {
		return command.Exit(2, fmt.Errorf("usage: appkit data assets-adapt-navigation [-root PRIVATE_DIRECTORY] [-out NEW_PRIVATE_DIRECTORY | -check PRIVATE_DIRECTORY]"))
	}
	original, err := designassets.ReadFile(*root, designadapt.NavigationArchive, designadapt.NavigationMember)
	if err != nil {
		return command.Exit(2, err)
	}
	result, err := designadapt.AdaptNavigation(designconvert.Source{ArchiveID: original.ArchiveID, Path: original.Path, SHA256: original.SHA256, Size: len(original.Content)}, []byte(original.Content))
	if err != nil {
		return command.Exit(2, err)
	}
	if outSet || checkSet {
		if outSet {
			err = designadapt.WriteNavigationDelivery(*out, result, []byte(original.Content))
		} else {
			err = designadapt.CheckNavigationDelivery(*check, result, []byte(original.Content))
		}
		if errors.Is(err, designadapt.ErrNavigationDeliveryDrift) {
			return command.Exit(1, err)
		}
		if err != nil {
			return command.Exit(2, err)
		}
		return nil
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		return command.Exit(2, err)
	}
	return nil
}
