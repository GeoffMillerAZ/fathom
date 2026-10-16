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

func dataAssetsAdaptMenu(args []string) error {
	fs := flag.NewFlagSet("assets-adapt-menu", flag.ContinueOnError)
	root := fs.String("root", defaultDesignAssetsRoot, "private archive/catalog directory")
	repair := fs.Bool("repair", false, "explicit eight-file menu geometry/focus repair variant; default six-file delivery is unchanged")
	out := fs.String("out", "", "export private sources to a NEW directory under an existing real parent")
	check := fs.String("check", "", "read-only exact drift check of a private export (macOS/Linux)")
	if err := fs.Parse(args); err != nil {
		return command.Exit(2, err)
	}
	var outSet, checkSet bool
	fs.Visit(func(f *flag.Flag) { outSet = outSet || f.Name == "out"; checkSet = checkSet || f.Name == "check" })
	if fs.NArg() != 0 || *root == "" || outSet && checkSet || outSet && *out == "" || checkSet && *check == "" {
		return command.Exit(2, fmt.Errorf("usage: appkit data assets-adapt-menu [-root PRIVATE_DIRECTORY] [-repair] [-out NEW_PRIVATE_DIRECTORY | -check PRIVATE_DIRECTORY]"))
	}
	original, err := designassets.ReadFile(*root, designadapt.MenuArchive, designadapt.MenuMember)
	if err != nil {
		return command.Exit(2, err)
	}
	source := designconvert.Source{ArchiveID: original.ArchiveID, Path: original.Path, SHA256: original.SHA256, Size: len(original.Content)}
	body := []byte(original.Content)
	var artifact any
	var writeDelivery, checkDelivery func(string) error
	if *repair {
		a, err := designadapt.AdaptMenuRepair(source, body)
		if err != nil {
			return command.Exit(2, err)
		}
		artifact = a
		writeDelivery = func(dir string) error { return designadapt.WriteMenuRepairDelivery(dir, a, body) }
		checkDelivery = func(dir string) error { return designadapt.CheckMenuRepairDelivery(dir, a, body) }
	} else {
		a, err := designadapt.AdaptMenu(source, body)
		if err != nil {
			return command.Exit(2, err)
		}
		artifact = a
		writeDelivery = func(dir string) error { return designadapt.WriteMenuDelivery(dir, a, body) }
		checkDelivery = func(dir string) error { return designadapt.CheckMenuDelivery(dir, a, body) }
	}
	if outSet || checkSet {
		if outSet {
			err = writeDelivery(*out)
		} else {
			err = checkDelivery(*check)
		}
		if errors.Is(err, designadapt.ErrMenuDeliveryDrift) || errors.Is(err, designadapt.ErrMenuRepairDeliveryDrift) {
			return command.Exit(1, err)
		}
		if err != nil {
			return command.Exit(2, err)
		}
		return nil
	}
	if err := json.NewEncoder(os.Stdout).Encode(artifact); err != nil {
		return command.Exit(2, err)
	}
	return nil
}
