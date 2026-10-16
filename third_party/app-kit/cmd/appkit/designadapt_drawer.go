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

func dataAssetsAdaptDrawer(args []string) error {
	fs := flag.NewFlagSet("assets-adapt-drawer", flag.ContinueOnError)
	root := fs.String("root", defaultDesignAssetsRoot, "private archive/catalog directory")
	repair := fs.Bool("repair", false, "select the explicit CSS-only focus repair and six-file delivery")
	out := fs.String("out", "", "export private sources to a NEW directory under an existing real parent")
	check := fs.String("check", "", "read-only exact drift check of a private export (macOS/Linux)")
	if err := fs.Parse(args); err != nil {
		return command.Exit(2, err)
	}
	var outSet, checkSet bool
	fs.Visit(func(f *flag.Flag) { outSet = outSet || f.Name == "out"; checkSet = checkSet || f.Name == "check" })
	if fs.NArg() != 0 || *root == "" || outSet && checkSet || outSet && *out == "" || checkSet && *check == "" {
		return command.Exit(2, fmt.Errorf("usage: appkit data assets-adapt-drawer [-root PRIVATE_DIRECTORY] [-repair] [-out NEW_PRIVATE_DIRECTORY | -check PRIVATE_DIRECTORY]"))
	}
	original, err := designassets.ReadFile(*root, designadapt.DrawerArchive, designadapt.DrawerMember)
	if err != nil {
		return command.Exit(2, err)
	}
	body := []byte(original.Content)
	source := designconvert.Source{ArchiveID: original.ArchiveID, Path: original.Path, SHA256: original.SHA256, Size: len(body)}
	if *repair {
		return dataAssetsAdaptDrawerFocus(source, body, *out, *check, outSet, checkSet)
	}
	artifact, err := designadapt.AdaptDrawer(source, body)
	if err != nil {
		return command.Exit(2, err)
	}
	if outSet || checkSet {
		if outSet {
			err = designadapt.WriteDrawerDelivery(*out, artifact, body)
		} else {
			err = designadapt.CheckDrawerDelivery(*check, artifact, body)
		}
		if errors.Is(err, designadapt.ErrDrawerDeliveryDrift) {
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

func dataAssetsAdaptDrawerFocus(source designconvert.Source, body []byte, out, check string, outSet, checkSet bool) error {
	artifact, err := designadapt.AdaptDrawerFocus(source, body)
	if err != nil {
		return command.Exit(2, err)
	}
	if outSet || checkSet {
		if outSet {
			err = designadapt.WriteDrawerFocusDelivery(out, artifact, body)
		} else {
			err = designadapt.CheckDrawerFocusDelivery(check, artifact, body)
		}
		if errors.Is(err, designadapt.ErrDrawerFocusDeliveryDrift) {
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
