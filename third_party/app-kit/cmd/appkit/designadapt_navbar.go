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

func dataAssetsAdaptNavbar(args []string) error {
	fs := flag.NewFlagSet("assets-adapt-navbar", flag.ContinueOnError)
	root := fs.String("root", defaultDesignAssetsRoot, "private archive/catalog directory")
	repair := fs.Bool("repair", false, "explicit nine-file navbar repair variant; default five-file output is unchanged")
	runtimePath := fs.String("runtime", "", "absolute original pinned Elements ESM path; required only with -repair")
	out := fs.String("out", "", "export private sources to a NEW directory under an existing real parent")
	check := fs.String("check", "", "read-only exact drift check of a private export (macOS/Linux)")
	if err := fs.Parse(args); err != nil {
		return command.Exit(2, err)
	}
	var outSet, checkSet, runtimeSet bool
	fs.Visit(func(f *flag.Flag) {
		outSet = outSet || f.Name == "out"
		checkSet = checkSet || f.Name == "check"
		runtimeSet = runtimeSet || f.Name == "runtime"
	})
	if fs.NArg() != 0 || *root == "" || outSet && checkSet || outSet && *out == "" || checkSet && *check == "" || *repair && *runtimePath == "" || !*repair && runtimeSet {
		return command.Exit(2, fmt.Errorf("usage: appkit data assets-adapt-navbar [-root PRIVATE_DIRECTORY] [-repair -runtime ABSOLUTE_ORIGINAL_ESM] [-out NEW_PRIVATE_DIRECTORY | -check PRIVATE_DIRECTORY]"))
	}
	original, err := designassets.ReadFile(*root, designadapt.NavbarArchive, designadapt.NavbarMember)
	if err != nil {
		return command.Exit(2, err)
	}
	body := []byte(original.Content)
	source := designconvert.Source{ArchiveID: original.ArchiveID, Path: original.Path, SHA256: original.SHA256, Size: len(body)}
	var artifact any
	var writeDelivery, checkDelivery func(string) error
	if *repair {
		runtime, err := designadapt.ReadNavbarRuntimeOriginal(*runtimePath)
		if err != nil {
			return command.Exit(2, err)
		}
		a, err := designadapt.AdaptNavbarRepairDelivery(source, body, runtime)
		if err != nil {
			return command.Exit(2, err)
		}
		artifact = a
		writeDelivery = func(dir string) error { return designadapt.WriteNavbarRepairDelivery(dir, a, body, runtime) }
		checkDelivery = func(dir string) error { return designadapt.CheckNavbarRepairDelivery(dir, a, body, runtime) }
	} else {
		a, err := designadapt.AdaptNavbar(source, body)
		if err != nil {
			return command.Exit(2, err)
		}
		artifact = a
		writeDelivery = func(dir string) error { return designadapt.WriteNavbarDelivery(dir, a, body) }
		checkDelivery = func(dir string) error { return designadapt.CheckNavbarDelivery(dir, a, body) }
	}
	if outSet || checkSet {
		if outSet {
			err = writeDelivery(*out)
		} else {
			err = checkDelivery(*check)
		}
		if errors.Is(err, designadapt.ErrNavbarDeliveryDrift) || errors.Is(err, designadapt.ErrNavbarRepairDeliveryDrift) {
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
