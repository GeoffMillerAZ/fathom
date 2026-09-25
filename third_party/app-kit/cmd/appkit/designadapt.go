package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/geoffmilleraz/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/app-kit/internal/designadapt"
	"github.com/geoffmilleraz/app-kit/internal/designassets"
	"github.com/geoffmilleraz/app-kit/internal/designconvert"
)

// Explicit generation for one frozen case. JSON remains the read-only default;
// opt-in private delivery never creates a certified catalog entry.
func dataAssetsAdaptPricing(args []string) error {
	fs := flag.NewFlagSet("assets-adapt-pricing", flag.ContinueOnError)
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
	if fs.NArg() != 0 || *root == "" || (outSet && checkSet) || (outSet && *out == "") || (checkSet && *check == "") {
		return command.Exit(2, fmt.Errorf("usage: appkit data assets-adapt-pricing [-root PRIVATE_DIRECTORY] [-out NEW_PRIVATE_DIRECTORY | -check PRIVATE_DIRECTORY]"))
	}
	original, err := designassets.ReadFile(*root, designadapt.PricingArchive, designadapt.PricingMember)
	if err != nil {
		return command.Exit(2, err)
	}
	result, err := designadapt.AdaptPricing(designconvert.Source{ArchiveID: original.ArchiveID, Path: original.Path, SHA256: original.SHA256, Size: len(original.Content)}, []byte(original.Content))
	if err != nil {
		return command.Exit(2, err)
	}
	if outSet || checkSet {
		if outSet {
			err = designadapt.WritePricingDelivery(*out, result, []byte(original.Content))
		} else {
			err = designadapt.CheckPricingDelivery(*check, result, []byte(original.Content))
		}
		if errors.Is(err, designadapt.ErrPricingDeliveryDrift) {
			return command.Exit(1, err)
		}
		if err != nil {
			return command.Exit(2, err)
		}
		// Success is silent, like other check/build commands; never print paid
		// bodies or imply runtime certification from source delivery.
		return nil
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		return command.Exit(2, err)
	}
	return nil
}

// Directory is a second source-specific pilot, not a generic table language.
// JSON stays the default; explicit private delivery does not promote the catalog.
func dataAssetsAdaptDirectory(args []string) error {
	fs := flag.NewFlagSet("assets-adapt-directory", flag.ContinueOnError)
	root := fs.String("root", defaultDesignAssetsRoot, "private archive/catalog directory")
	enhanced := fs.Bool("enhanced", false, "emit the opt-in scoped usability enhancement envelope")
	out := fs.String("out", "", "export private sources to a NEW directory under an existing real parent")
	check := fs.String("check", "", "read-only exact drift check of the selected variant (macOS/Linux)")
	if err := fs.Parse(args); err != nil {
		return command.Exit(2, err)
	}
	var outSet, checkSet bool
	fs.Visit(func(f *flag.Flag) {
		outSet = outSet || f.Name == "out"
		checkSet = checkSet || f.Name == "check"
	})
	if fs.NArg() != 0 || *root == "" || (outSet && checkSet) || (outSet && *out == "") || (checkSet && *check == "") {
		return command.Exit(2, fmt.Errorf("usage: appkit data assets-adapt-directory [-root PRIVATE_DIRECTORY] [-enhanced] [-out NEW_PRIVATE_DIRECTORY | -check PRIVATE_DIRECTORY]"))
	}
	original, err := designassets.ReadFile(*root, designadapt.DirectoryArchive, designadapt.DirectoryMember)
	if err != nil {
		return command.Exit(2, err)
	}
	source := designconvert.Source{ArchiveID: original.ArchiveID, Path: original.Path, SHA256: original.SHA256, Size: len(original.Content)}
	if *enhanced {
		result, err := designadapt.AdaptDirectoryEnhanced(source, []byte(original.Content))
		if err != nil {
			return command.Exit(2, err)
		}
		if err := designadapt.VerifyDirectoryEnhancedReplay(result, []byte(original.Content)); err != nil {
			return command.Exit(2, err)
		}
		if outSet || checkSet {
			if outSet {
				err = designadapt.WriteDirectoryEnhancedDelivery(*out, result, []byte(original.Content))
			} else {
				err = designadapt.CheckDirectoryEnhancedDelivery(*check, result, []byte(original.Content))
			}
			return directoryDeliveryExit(err)
		}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			return command.Exit(2, err)
		}
		return nil
	}
	result, err := designadapt.AdaptDirectory(source, []byte(original.Content))
	if err != nil {
		return command.Exit(2, err)
	}
	if outSet || checkSet {
		if outSet {
			err = designadapt.WriteDirectoryDelivery(*out, result, []byte(original.Content))
		} else {
			err = designadapt.CheckDirectoryDelivery(*check, result, []byte(original.Content))
		}
		return directoryDeliveryExit(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		return command.Exit(2, err)
	}
	return nil
}

// Success is silent; only observed P04 delivery drift is exit 1. Never emit a
// private body on export/check or classify failed source verification as drift.
func directoryDeliveryExit(err error) error {
	if errors.Is(err, designadapt.ErrDirectoryDeliveryDrift) {
		return command.Exit(1, err)
	}
	if err != nil {
		return command.Exit(2, err)
	}
	return nil
}
