// Package macbundle assembles and validates the Gallery's private unsigned
// macOS bundle. It deliberately does not launch, install, sign, or modify OS
// trust metadata.
package macbundle

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	manifestSchema = 1
	bundleMode     = "private"
	wailsVersion   = "v3.0.0-beta.21"
	maxMetadata    = 64 << 10
	maxExecutable  = 256 << 20

	launcherPath = "Contents/MacOS/gallery-launcher"
	serverPath   = "Contents/Resources/gallery-server"
	nativePath   = "Contents/Resources/gallery-native"
	manifestPath = "Contents/Resources/bundle.json"
	plistPath    = "Contents/Info.plist"
)

var (
	hashPattern     = regexp.MustCompile(`^[a-f0-9]{64}$`)
	revisionPattern = regexp.MustCompile(`^(?:[a-f0-9]{40}|[a-f0-9]{64})$`)
)

type Executable struct {
	Path   string
	SHA256 string
}

type Inputs struct {
	Launcher    Executable
	Server      Executable
	Native      Executable
	KitRevision string
	MinMacOS    string
}

type FileRecord struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type Manifest struct {
	SchemaVersion int           `json:"schema_version"`
	Mode          string        `json:"mode"`
	KitRevision   string        `json:"kit_revision"`
	WailsVersion  string        `json:"wails_version"`
	MinMacOS      string        `json:"min_macos"`
	Files         [3]FileRecord `json:"files"`
}

type LaunchPlan struct {
	Executable string
	Args       []string
	Manifest   Manifest
}

type preparedExecutable struct {
	file   *os.File
	record FileRecord
}

func Assemble(destination string, input Inputs) (Manifest, error) {
	if err := validateDestination(destination); err != nil {
		return Manifest{}, err
	}
	if !revisionPattern.MatchString(input.KitRevision) {
		return Manifest{}, fmt.Errorf("kit revision must be a full 40- or 64-character lowercase hex revision")
	}
	if err := validateMinMacOS(input.MinMacOS); err != nil {
		return Manifest{}, err
	}

	launcher, err := prepareExecutable(input.Launcher, launcherPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("launcher: %w", err)
	}
	defer launcher.file.Close()
	server, err := prepareExecutable(input.Server, serverPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("server: %w", err)
	}
	defer server.file.Close()
	native, err := prepareExecutable(input.Native, nativePath)
	if err != nil {
		return Manifest{}, fmt.Errorf("native: %w", err)
	}
	defer native.file.Close()

	manifest := Manifest{
		SchemaVersion: manifestSchema,
		Mode:          bundleMode,
		KitRevision:   input.KitRevision,
		WailsVersion:  wailsVersion,
		MinMacOS:      input.MinMacOS,
		Files:         [3]FileRecord{launcher.record, server.record, native.record},
	}
	manifestBytes, err := canonicalManifest(manifest)
	if err != nil {
		return Manifest{}, err
	}
	plistBytes := canonicalPlist(input.MinMacOS)

	// All inputs, metadata, pins, quarantine state, and the destination parent
	// are validated before this first output mutation. Failures after this point
	// intentionally leave a recognizable private partial bundle for inspection.
	if err := os.Mkdir(destination, 0o700); err != nil {
		return Manifest{}, fmt.Errorf("create bundle: %w", err)
	}
	if err := os.Mkdir(filepath.Join(destination, "Contents"), 0o700); err != nil {
		return Manifest{}, fmt.Errorf("create Contents: %w", err)
	}
	if err := os.Mkdir(filepath.Join(destination, "Contents", "MacOS"), 0o700); err != nil {
		return Manifest{}, fmt.Errorf("create MacOS: %w", err)
	}
	if err := os.Mkdir(filepath.Join(destination, "Contents", "Resources"), 0o700); err != nil {
		return Manifest{}, fmt.Errorf("create Resources: %w", err)
	}
	for _, prepared := range []*preparedExecutable{launcher, server, native} {
		if err := copyPrepared(destination, prepared); err != nil {
			return Manifest{}, err
		}
	}
	if err := writeExclusive(filepath.Join(destination, filepath.FromSlash(manifestPath)), manifestBytes, 0o600); err != nil {
		return Manifest{}, fmt.Errorf("write manifest: %w", err)
	}
	if err := writeExclusive(filepath.Join(destination, filepath.FromSlash(plistPath)), plistBytes, 0o600); err != nil {
		return Manifest{}, fmt.Errorf("write Info.plist: %w", err)
	}
	if _, err := Plan(filepath.Join(destination, filepath.FromSlash(launcherPath))); err != nil {
		return Manifest{}, fmt.Errorf("validate assembled bundle: %w", err)
	}
	return manifest, nil
}

func Plan(executable string) (LaunchPlan, error) {
	root, err := bundleRoot(executable)
	if err != nil {
		return LaunchPlan{}, err
	}
	if err := validateStagingParent(root); err != nil {
		return LaunchPlan{}, err
	}
	rootHandle, err := os.Open(root)
	if err != nil {
		return LaunchPlan{}, fmt.Errorf("bundle root: %w", err)
	}
	if err := rejectQuarantined(rootHandle); err != nil {
		rootHandle.Close()
		return LaunchPlan{}, fmt.Errorf("bundle root: %w", err)
	}
	if err := rootHandle.Close(); err != nil {
		return LaunchPlan{}, fmt.Errorf("bundle root: close: %w", err)
	}
	if err := validateLayout(root); err != nil {
		return LaunchPlan{}, err
	}
	manifestBytes, err := readBoundedRegular(filepath.Join(root, filepath.FromSlash(manifestPath)), maxMetadata)
	if err != nil {
		return LaunchPlan{}, fmt.Errorf("manifest: %w", err)
	}
	manifest, err := decodeManifest(manifestBytes)
	if err != nil {
		return LaunchPlan{}, fmt.Errorf("manifest: %w", err)
	}
	if err := validateManifest(manifest); err != nil {
		return LaunchPlan{}, fmt.Errorf("manifest: %w", err)
	}
	plistBytes, err := readBoundedRegular(filepath.Join(root, filepath.FromSlash(plistPath)), maxMetadata)
	if err != nil {
		return LaunchPlan{}, fmt.Errorf("Info.plist: %w", err)
	}
	if !bytes.Equal(plistBytes, canonicalPlist(manifest.MinMacOS)) {
		return LaunchPlan{}, fmt.Errorf("Info.plist does not match fixed bundle metadata")
	}
	for _, record := range manifest.Files {
		prepared, err := prepareExecutable(Executable{
			Path: filepath.Join(root, filepath.FromSlash(record.Path)), SHA256: record.SHA256,
		}, record.Path)
		if err != nil {
			return LaunchPlan{}, fmt.Errorf("%s: %w", record.Path, err)
		}
		if prepared.record.Size != record.Size {
			prepared.file.Close()
			return LaunchPlan{}, fmt.Errorf("%s: size mismatch", record.Path)
		}
		if err := prepared.file.Close(); err != nil {
			return LaunchPlan{}, fmt.Errorf("%s: close: %w", record.Path, err)
		}
	}
	native := filepath.Join(root, filepath.FromSlash(nativePath))
	server := filepath.Join(root, filepath.FromSlash(serverPath))
	return LaunchPlan{
		Executable: server,
		Args:       []string{"-native-host", native, "-native-host-sha256", manifest.Files[2].SHA256},
		Manifest:   manifest,
	}, nil
}

func prepareExecutable(input Executable, fixedPath string) (*preparedExecutable, error) {
	if !hashPattern.MatchString(input.SHA256) {
		return nil, fmt.Errorf("expected SHA-256 must be 64 lowercase hex characters")
	}
	if err := validateAbsoluteRealPath(input.Path); err != nil {
		return nil, err
	}
	pathInfo, err := os.Lstat(input.Path)
	if err != nil {
		return nil, err
	}
	if err := validateExecutableInfo(pathInfo); err != nil {
		return nil, err
	}
	file, err := os.Open(input.Path)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			file.Close()
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(pathInfo, info) {
		return nil, fmt.Errorf("executable changed while opening")
	}
	if err := validateExecutableInfo(info); err != nil {
		return nil, err
	}
	if err := rejectQuarantined(file); err != nil {
		return nil, err
	}
	if err := rejectQuarantinedAncestors(input.Path); err != nil {
		return nil, err
	}
	h := sha256.New()
	size, err := io.Copy(h, io.LimitReader(file, maxExecutable+1))
	if err != nil {
		return nil, err
	}
	if size > maxExecutable {
		return nil, fmt.Errorf("executable exceeds %d bytes", maxExecutable)
	}
	if size != info.Size() {
		return nil, fmt.Errorf("executable size changed while hashing")
	}
	actual := fmt.Sprintf("%x", h.Sum(nil))
	if actual != input.SHA256 {
		return nil, fmt.Errorf("SHA-256 mismatch: got %s", actual)
	}
	if size <= 0 {
		return nil, fmt.Errorf("empty executable refused")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	ok = true
	return &preparedExecutable{file: file, record: FileRecord{Path: fixedPath, SHA256: actual, Size: size}}, nil
}

// An executable can inherit its download's quarantine context from a containing
// directory. Refuse those markers too; copying a leaf must not bypass that context.
// This is a read-only check, not protection against hostile concurrent mutation.
func rejectQuarantinedAncestors(path string) error {
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		file, err := os.Open(dir)
		if err != nil {
			return fmt.Errorf("open source ancestor: %w", err)
		}
		quarantineErr := rejectQuarantined(file)
		closeErr := file.Close()
		if quarantineErr != nil {
			return fmt.Errorf("source ancestor %s: %w", dir, quarantineErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close source ancestor: %w", closeErr)
		}
		if filepath.Dir(dir) == dir {
			return nil
		}
	}
}

func validateExecutableInfo(info os.FileInfo) error {
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file")
	}
	if info.Size() <= 0 {
		return fmt.Errorf("empty executable refused")
	}
	if info.Size() > maxExecutable {
		return fmt.Errorf("executable exceeds %d bytes", maxExecutable)
	}
	if info.Mode().Perm()&0o100 == 0 {
		return fmt.Errorf("owner executable bit is required")
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("group/world-writable executable refused")
	}
	return nil
}

func copyPrepared(root string, prepared *preparedExecutable) error {
	destination := filepath.Join(root, filepath.FromSlash(prepared.record.Path))
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		return fmt.Errorf("create %s: %w", prepared.record.Path, err)
	}
	h := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(out, h), io.LimitReader(prepared.file, prepared.record.Size+1))
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("copy %s: %w", prepared.record.Path, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %s: %w", prepared.record.Path, closeErr)
	}
	if written > prepared.record.Size {
		return fmt.Errorf("%s grew while assembling", prepared.record.Path)
	}
	if written != prepared.record.Size || fmt.Sprintf("%x", h.Sum(nil)) != prepared.record.SHA256 {
		return fmt.Errorf("%s changed while assembling", prepared.record.Path)
	}
	return nil
}

func validateDestination(destination string) error {
	if !filepath.IsAbs(destination) || filepath.Clean(destination) != destination {
		return fmt.Errorf("destination must be a clean absolute path")
	}
	if filepath.Ext(destination) != ".app" || filepath.Base(destination) == ".app" {
		return fmt.Errorf("destination must be a named .app path")
	}
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("destination already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return validateStagingParent(destination)
}

func validateStagingParent(destination string) error {
	parent := filepath.Dir(destination)
	if err := validateAbsoluteRealPath(parent); err != nil {
		return fmt.Errorf("destination parent: %w", err)
	}
	info, err := os.Stat(parent)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("destination parent is not a directory")
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("bundle parent is group/world writable")
	}
	return nil
}

func validateAbsoluteRealPath(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("path must be clean and absolute")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if resolved != path {
		return fmt.Errorf("symlink path or ancestor refused")
	}
	return nil
}

func bundleRoot(executable string) (string, error) {
	if err := validateAbsoluteRealPath(executable); err != nil {
		return "", fmt.Errorf("launcher path: %w", err)
	}
	if filepath.Base(executable) != "gallery-launcher" || filepath.Base(filepath.Dir(executable)) != "MacOS" {
		return "", fmt.Errorf("launcher is not at %s", launcherPath)
	}
	contents := filepath.Dir(filepath.Dir(executable))
	if filepath.Base(contents) != "Contents" {
		return "", fmt.Errorf("launcher is not at %s", launcherPath)
	}
	root := filepath.Dir(contents)
	if filepath.Ext(root) != ".app" {
		return "", fmt.Errorf("launcher is not inside a .app bundle")
	}
	return root, nil
}

func validateLayout(root string) error {
	expected := map[string][]string{
		"":                   {"Contents"},
		"Contents":           {"Info.plist", "MacOS", "Resources"},
		"Contents/MacOS":     {"gallery-launcher"},
		"Contents/Resources": {"bundle.json", "gallery-native", "gallery-server"},
	}
	for rel, names := range expected {
		dir := filepath.Join(root, filepath.FromSlash(rel))
		if err := validateAbsoluteRealPath(dir); err != nil {
			return fmt.Errorf("layout %s: %w", rel, err)
		}
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("layout %s is not a directory", rel)
		}
		if info.Mode().Perm()&0o022 != 0 {
			return fmt.Errorf("layout %s is group/world writable", rel)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		if len(entries) != len(names) {
			return fmt.Errorf("layout %s has unexpected entries", rel)
		}
		for i, entry := range entries {
			if entry.Name() != names[i] {
				return fmt.Errorf("layout %s has unexpected entry %s", rel, entry.Name())
			}
		}
	}
	return nil
}

func readBoundedRegular(path string, limit int64) ([]byte, error) {
	if err := validateAbsoluteRealPath(path); err != nil {
		return nil, err
	}
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if err := validateMetadataInfo(pathInfo, limit); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(pathInfo, info) {
		return nil, fmt.Errorf("metadata changed while opening")
	}
	if err := validateMetadataInfo(info, limit); err != nil {
		return nil, err
	}
	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("metadata exceeds %d bytes", limit)
	}
	if int64(len(body)) != info.Size() {
		return nil, fmt.Errorf("metadata size changed while reading")
	}
	return body, nil
}

func validateMetadataInfo(info os.FileInfo, limit int64) error {
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file")
	}
	if info.Size() > limit {
		return fmt.Errorf("metadata exceeds %d bytes", limit)
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("group/world-writable metadata refused")
	}
	return nil
}

func writeExclusive(path string, body []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(body); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func canonicalManifest(manifest Manifest) ([]byte, error) {
	body, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

func decodeManifest(body []byte) (Manifest, error) {
	if len(body) == 0 || len(body) > maxMetadata {
		return Manifest{}, fmt.Errorf("manifest size is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, err
	}
	if err := expectJSONEOF(decoder); err != nil {
		return Manifest{}, err
	}
	canonical, err := canonicalManifest(manifest)
	if err != nil {
		return Manifest{}, err
	}
	if !bytes.Equal(body, canonical) {
		return Manifest{}, fmt.Errorf("manifest is not canonical JSON")
	}
	return manifest, nil
}

func expectJSONEOF(decoder *json.Decoder) error {
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}

func validateManifest(manifest Manifest) error {
	if manifest.SchemaVersion != manifestSchema || manifest.Mode != bundleMode || manifest.WailsVersion != wailsVersion {
		return fmt.Errorf("fixed bundle metadata mismatch")
	}
	if !revisionPattern.MatchString(manifest.KitRevision) {
		return fmt.Errorf("invalid kit revision")
	}
	if err := validateMinMacOS(manifest.MinMacOS); err != nil {
		return err
	}
	paths := [3]string{launcherPath, serverPath, nativePath}
	for i, record := range manifest.Files {
		if record.Path != paths[i] || !hashPattern.MatchString(record.SHA256) || record.Size <= 0 || record.Size > maxExecutable {
			return fmt.Errorf("invalid executable record %d", i)
		}
	}
	return nil
}

func validateMinMacOS(value string) error {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return fmt.Errorf("minimum macOS must be canonical major.minor")
	}
	major, errMajor := strconv.Atoi(parts[0])
	minor, errMinor := strconv.Atoi(parts[1])
	if errMajor != nil || errMinor != nil || major < 10 || major > 99 || minor < 0 || minor > 99 || value != fmt.Sprintf("%d.%d", major, minor) {
		return fmt.Errorf("minimum macOS must be canonical major.minor")
	}
	return nil
}

func canonicalPlist(minMacOS string) []byte {
	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleIdentifier</key><string>local.appkit.gallery.private</string>
  <key>CFBundleName</key><string>App Kit Gallery</string>
  <key>CFBundleExecutable</key><string>gallery-launcher</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>0.0.0</string>
  <key>CFBundleVersion</key><string>1</string>
  <key>LSMinimumSystemVersion</key><string>%s</string>
  <key>NSHighResolutionCapable</key><true/>
  <key>NSAppTransportSecurity</key>
  <dict><key>NSAllowsLocalNetworking</key><true/></dict>
</dict>
</plist>
`, minMacOS))
}
