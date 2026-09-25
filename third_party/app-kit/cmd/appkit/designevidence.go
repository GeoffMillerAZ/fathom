package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/geoffmilleraz/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/app-kit/internal/designevidence"
	"github.com/geoffmilleraz/app-kit/internal/designreceipts"
)

func dataAssetsEvidence(args []string) error {
	fs := flag.NewFlagSet("assets-evidence", flag.ContinueOnError)
	root := fs.String("root", defaultDesignAssetsRoot, "private original archive/catalog directory")
	archive := fs.String("archive", "", "filter by exact source archive ID")
	subject := fs.String("subject", "", "filter by exact evidence subject ID")
	profile := fs.String("profile", "", "filter by exact target profile")
	cursor := fs.String("cursor", "", "continuation cursor from the same query and catalog revision")
	limit := fs.Int("limit", 10, "maximum evidence entries per page")
	if err := fs.Parse(args); err != nil {
		return command.Exit(2, err)
	}
	if fs.NArg() != 0 {
		return command.Exit(2, fmt.Errorf("usage: appkit data assets-evidence [-archive ARCHIVE_ID] [-subject SUBJECT_ID] [-profile PROFILE] [-limit N] [-cursor CURSOR] [-root PRIVATE_DIRECTORY]"))
	}
	catalog, err := designevidence.Load(*root)
	if err != nil {
		return command.Exit(2, err)
	}
	page, err := catalog.Search(designevidence.Query{ArchiveID: *archive, SubjectID: *subject, Profile: *profile, Cursor: *cursor, Limit: *limit})
	if err != nil {
		return command.Exit(2, err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(page); err != nil {
		return command.Exit(2, err)
	}
	return nil
}

func dataAssetsEvidenceGet(args []string) error {
	fs := flag.NewFlagSet("assets-evidence-get", flag.ContinueOnError)
	root := fs.String("root", defaultDesignAssetsRoot, "private original archive/catalog directory")
	id := fs.String("id", "", "exact evidence@SHA256 ID")
	if err := fs.Parse(args); err != nil {
		return command.Exit(2, err)
	}
	if *id == "" || fs.NArg() != 0 {
		return command.Exit(2, fmt.Errorf("usage: appkit data assets-evidence-get -id EVIDENCE_ID [-root PRIVATE_DIRECTORY]"))
	}
	detail, err := designevidence.Read(*root, *id)
	if err != nil {
		return command.Exit(2, err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(detail); err != nil {
		return command.Exit(2, err)
	}
	return nil
}

func dataAssetsEvidenceSave(args []string) error {
	fs := flag.NewFlagSet("assets-evidence-save", flag.ContinueOnError)
	root := fs.String("root", defaultDesignAssetsRoot, "private original archive/catalog directory")
	bundle := fs.String("bundle", "", "canonical private evidence bundle JSON file")
	contractSHA := fs.String("contract-sha", "", "independently selected expected contract SHA-256")
	sourceSHA := fs.String("source-sha", "", "independently selected expected source revision SHA-256")
	artifactSHA := fs.String("artifact-sha", "", "independently selected expected artifact SHA-256")
	if err := fs.Parse(args); err != nil {
		return command.Exit(2, err)
	}
	if *bundle == "" || *contractSHA == "" || *sourceSHA == "" || *artifactSHA == "" || fs.NArg() != 0 {
		return command.Exit(2, fmt.Errorf("usage: appkit data assets-evidence-save -bundle FILE -contract-sha SHA256 -source-sha SHA256 -artifact-sha SHA256 [-root PRIVATE_DIRECTORY]"))
	}
	body, err := readEvidenceBundle(*bundle)
	if err != nil {
		return command.Exit(2, err)
	}
	entry, err := designevidence.PublishBundle(*root, body, designreceipts.Expected{
		ContractSHA256: *contractSHA,
		SourceRevision: *sourceSHA,
		ArtifactSHA256: *artifactSHA,
	})
	if err != nil {
		return command.Exit(2, err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(entry); err != nil {
		return command.Exit(2, err)
	}
	return nil
}

// readEvidenceBundle accepts one bounded regular file. Bundle strings remain
// inert JSON data: this command never treats embedded paths or commands as IO.
func readEvidenceBundle(name string) ([]byte, error) {
	before, err := os.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("inspect evidence bundle: %w", err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, fmt.Errorf("evidence bundle must be a regular non-symlink file")
	}
	if before.Size() > designevidence.MaxBundleBytes {
		return nil, fmt.Errorf("evidence bundle exceeds %d-byte limit", designevidence.MaxBundleBytes)
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("open evidence bundle: %w", err)
	}
	defer f.Close()
	after, err := f.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		return nil, fmt.Errorf("evidence bundle changed while opening")
	}
	body, err := io.ReadAll(io.LimitReader(f, designevidence.MaxBundleBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read evidence bundle: %w", err)
	}
	if len(body) > designevidence.MaxBundleBytes {
		return nil, fmt.Errorf("evidence bundle exceeds %d-byte limit", designevidence.MaxBundleBytes)
	}
	return body, nil
}
