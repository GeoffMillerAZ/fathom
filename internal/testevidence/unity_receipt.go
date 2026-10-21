package testevidence

// Native Unity evidence is admitted only with a producer-generated test-kit
// record. The receipt is a local assertion plus integrity/freshness binding,
// not a signature and not proof against a malicious producer or filesystem.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/scip"
	"github.com/geoffmilleraz/fathom/internal/testrun"
)

const UnityReceiptSchema = "fathom.unity.nunit/v1"
const unityJSONLimit = 32 << 20
const unityInputLimit = 100000
const unityInputBytesLimit int64 = 8 << 30

var ErrUnityEvidence = errors.New("Unity NUnit evidence refused")
var unitySHA = regexp.MustCompile(`^[0-9a-f]{64}$`)

type UnityFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type UnityProducer struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
}
type UnityEditor struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}
type UnityInventory struct {
	Root  string   `json:"root"`
	Paths []string `json:"paths"`
}
type UnitySource struct {
	BeforeSHA256 string           `json:"before_sha256"`
	AfterSHA256  string           `json:"after_sha256"`
	Inputs       []UnityFile      `json:"inputs"`
	Inventories  []UnityInventory `json:"inventories"`
}
type UnitySymbols struct {
	SCIPPath            string `json:"scip_path"`
	SCIPSHA256          string `json:"scip_sha256"`
	SourceReceiptSHA256 string `json:"source_receipt_sha256"`
}
type UnityReceipt struct {
	Schema      string        `json:"schema"`
	Status      string        `json:"status"`
	RunID       string        `json:"run_id"`
	Root        string        `json:"root"`
	Project     string        `json:"project"`
	Mode        string        `json:"mode"`
	Target      string        `json:"target"`
	Filter      string        `json:"filter"`
	StartedAt   string        `json:"started_at"`
	FinishedAt  string        `json:"finished_at"`
	Producer    UnityProducer `json:"producer"`
	Editor      UnityEditor   `json:"editor"`
	Command     []string      `json:"command"`
	ExitCode    *int          `json:"exit_code"`
	XML         UnityFile     `json:"xml"`
	Source      UnitySource   `json:"source"`
	Attestation UnityFile     `json:"attestation"`
	Symbols     *UnitySymbols `json:"symbols,omitempty"`
}

// UnityEvidence is a validated native parse and its standard record. A missing
// typed identity leaves cases unjoined; parsing alone never writes test_runs.
type UnityEvidence struct {
	Receipt                    UnityReceipt
	Native                     testrun.NUnitRun
	Run                        Run
	ReceiptPath, ReceiptSHA256 string
}

// UnitySourceBytes is the documented canonical input representation. Validation
// rejects duplicate/noncanonical inputs before using this representation.
func UnitySourceBytes(source UnitySource) []byte {
	var b bytes.Buffer
	b.WriteString("fathom.unity.inputs/v1\n")
	trees := append([]UnityInventory(nil), source.Inventories...)
	sort.Slice(trees, func(i, j int) bool { return trees[i].Root < trees[j].Root })
	for _, tree := range trees {
		b.WriteString("tree\x00" + tree.Root + "\n")
	}
	inputs := append([]UnityFile(nil), source.Inputs...)
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].Path < inputs[j].Path })
	for _, input := range inputs {
		b.WriteString("file\x00" + input.Path + "\x00" + input.SHA256 + "\n")
	}
	return b.Bytes()
}
func unityDigest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
func unityError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrUnityEvidence, fmt.Sprintf(format, args...))
}
func unitySafeText(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}
func unityPath(root, path string, absolute bool) (string, error) {
	if path == "" || !unitySafeText(path) || strings.Contains(path, "\\") {
		return "", fmt.Errorf("unsafe path")
	}
	if filepath.IsAbs(path) {
		if !absolute || filepath.Clean(path) != path {
			return "", fmt.Errorf("unexpected/noncanonical absolute path")
		}
		return path, nil
	}
	if !fs.ValidPath(path) || path == "." {
		return "", fmt.Errorf("noncanonical relative path")
	}
	return filepath.Join(root, filepath.FromSlash(path)), nil
}
func unityRegular(path string) (fs.FileInfo, error) {
	for at := path; ; at = filepath.Dir(at) {
		info, err := os.Lstat(at)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlink path")
		}
		if at == path && !info.Mode().IsRegular() {
			return nil, fmt.Errorf("not a regular file")
		}
		if filepath.Dir(at) == at {
			break
		}
	}
	return os.Stat(path)
}
func unityRead(path string, limit int64) ([]byte, error) {
	info, err := unityRegular(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("file exceeds %d bytes", limit)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("file grew beyond byte limit")
	}
	return raw, nil
}
func unityHash(ctx context.Context, path string, limit int64, buffer []byte) (string, int64, error) {
	info, err := unityRegular(path)
	if err != nil {
		return "", 0, err
	}
	if info.Size() > limit {
		return "", 0, fmt.Errorf("input byte budget exceeded")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	bufferSize := info.Size() + 1
	if bufferSize > 128<<10 {
		bufferSize = 128 << 10
	}
	buf := buffer[:int(bufferSize)]
	var count int64
	for {
		if err := ctx.Err(); err != nil {
			return "", 0, err
		}
		n, e := f.Read(buf)
		if n > 0 {
			count += int64(n)
			if count > limit {
				return "", 0, fmt.Errorf("input grew beyond budget")
			}
			h.Write(buf[:n])
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return "", 0, e
		}
	}
	if count != info.Size() {
		return "", 0, fmt.Errorf("input size changed during read")
	}
	return hex.EncodeToString(h.Sum(nil)), count, nil
}

// Duplicate JSON members are refused before struct or CUE decoding. Encoding/json
// otherwise accepts the last member, silently hiding contradictory provenance.
func unityUniqueJSON(raw []byte) error {
	if len(raw) == 0 || !utf8.Valid(raw) {
		return fmt.Errorf("empty/invalid UTF8 JSON")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 64 {
			return fmt.Errorf("JSON depth exceeded")
		}
		t, err := d.Token()
		if err != nil {
			return err
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				k, ok := key.(string)
				if !ok || seen[k] {
					return fmt.Errorf("duplicate/invalid JSON member %q", k)
				}
				seen[k] = true
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unexpected JSON delimiter")
		}
		_, err = d.Token()
		return err
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}
func unityDecode(raw []byte, out any) error {
	if err := unityUniqueJSON(raw); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}
func unityFileBytes(root string, file UnityFile, limit int64) ([]byte, error) {
	if !unitySHA.MatchString(file.SHA256) {
		return nil, fmt.Errorf("invalid SHA256")
	}
	path, err := unityPath(root, file.Path, true)
	if err != nil {
		return nil, err
	}
	raw, err := unityRead(path, limit)
	if err != nil {
		return nil, err
	}
	if unityDigest(raw) != file.SHA256 {
		return nil, fmt.Errorf("file hash changed: %s", file.Path)
	}
	return raw, nil
}
func unityValidateSource(ctx context.Context, root string, r UnityReceipt) error {
	s := r.Source
	if !unitySHA.MatchString(s.BeforeSHA256) || s.BeforeSHA256 != s.AfterSHA256 || unityDigest(UnitySourceBytes(s)) != s.BeforeSHA256 {
		return fmt.Errorf("source before/after/canonical digests disagree")
	}
	if len(s.Inputs) == 0 || len(s.Inputs) > unityInputLimit || len(s.Inventories) != 3 {
		return fmt.Errorf("source input/inventory count outside native bounds")
	}
	inputs := map[string]string{}
	buffer := make([]byte, 128<<10)
	var bytes int64
	for _, input := range s.Inputs {
		path, err := unityPath(root, input.Path, true)
		if err != nil {
			return err
		}
		if _, ok := inputs[path]; ok {
			return fmt.Errorf("duplicate source input")
		}
		if !unitySHA.MatchString(input.SHA256) {
			return fmt.Errorf("invalid source hash")
		}
		hash, size, err := unityHash(ctx, path, unityInputBytesLimit-bytes, buffer)
		if err != nil {
			return err
		}
		bytes += size
		if hash != input.SHA256 {
			return fmt.Errorf("source input changed: %s", input.Path)
		}
		inputs[path] = hash
	}
	required := map[string]bool{}
	for _, name := range []string{"Assets", "Packages", "ProjectSettings"} {
		required[r.Project+"/"+name] = true
	}
	for _, tree := range s.Inventories {
		if !required[tree.Root] {
			return fmt.Errorf("missing/duplicate/unexpected Unity inventory")
		}
		delete(required, tree.Root)
		path, err := unityPath(root, tree.Root, false)
		if err != nil {
			return err
		}
		info, statErr := os.Lstat(path)
		if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("Unity inventory root is not a regular directory")
		}
		if tree.Paths == nil || len(tree.Paths) > unityInputLimit || !sort.StringsAreSorted(tree.Paths) {
			return fmt.Errorf("inventory membership must be bounded/sorted")
		}
		members := []string{}
		visited := 0
		err = filepath.WalkDir(path, func(at string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			visited++
			if visited > unityInputLimit {
				return fmt.Errorf("inventory entry budget exceeded")
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("symlink inventory member")
			}
			if entry.IsDir() {
				return nil
			}
			if !entry.Type().IsRegular() {
				return fmt.Errorf("nonregular inventory member")
			}
			relative, err := filepath.Rel(root, at)
			if err != nil {
				return err
			}
			key := filepath.ToSlash(relative)
			if _, err = unityPath(root, key, false); err != nil {
				return err
			}
			if _, ok := inputs[at]; !ok {
				return fmt.Errorf("inventory member has no input hash")
			}
			members = append(members, key)
			if len(members) > unityInputLimit {
				return fmt.Errorf("inventory member budget exceeded")
			}
			return nil
		})
		if err != nil {
			return err
		}
		sort.Strings(members)
		if !slices.Equal(members, tree.Paths) {
			return fmt.Errorf("inventory membership changed: %s", tree.Root)
		}
	}

	for _, entry := range []struct {
		name string
		file UnityFile
	}{{"producer", UnityFile{r.Producer.Path, r.Producer.SHA256}}, {"Editor", UnityFile{r.Editor.Path, r.Editor.SHA256}}} {
		path, err := unityPath(root, entry.file.Path, true)
		if err != nil {
			return fmt.Errorf("%s path is not canonical: %w", entry.name, err)
		}
		if inputs[path] != entry.file.SHA256 {
			return fmt.Errorf("%s missing from source closure", entry.name)
		}
	}

	return nil
}
func unityValidateCommand(root string, r UnityReceipt) error {
	if len(r.Command) < 2 || len(r.Command) > 64 {
		return fmt.Errorf("invalid Editor argv length")
	}
	editor, err := unityPath(root, r.Editor.Path, true)
	if err != nil {
		return err
	}
	command, err := unityPath(root, r.Command[0], true)
	if err != nil || command != editor {
		return fmt.Errorf("command executable differs from Editor")
	}
	pairs := map[string]string{}
	flags := map[string]bool{}
	for i := 1; i < len(r.Command); i++ {
		a := r.Command[i]
		if !unitySafeText(a) {
			return fmt.Errorf("unsafe Editor argv")
		}
		switch a {
		case "-batchmode", "-runTests", "-nographics":
			if flags[a] {
				return fmt.Errorf("duplicate Editor flag")
			}
			flags[a] = true
		case "-projectPath", "-buildTarget", "-testPlatform", "-testResults", "-testFilter", "-logFile":
			if _, ok := pairs[a]; ok || i+1 == len(r.Command) {
				return fmt.Errorf("duplicate/missing Editor argument")
			}
			i++
			if !unitySafeText(r.Command[i]) {
				return fmt.Errorf("unsafe Editor argument")
			}
			pairs[a] = r.Command[i]
		default:
			return fmt.Errorf("unsupported Editor argument at position %d", i)
		}
	}
	project, _ := unityPath(root, r.Project, false)
	xmlPath, err := unityPath(root, r.XML.Path, true)
	if err != nil {
		return err
	}
	recordedProject, err := unityPath(root, pairs["-projectPath"], true)
	if err != nil || recordedProject != project {
		return fmt.Errorf("Editor project differs")
	}
	recordedXML, err := unityPath(root, pairs["-testResults"], true)
	if err != nil || recordedXML != xmlPath {
		return fmt.Errorf("Editor XML path differs")
	}
	if !flags["-runTests"] || pairs["-buildTarget"] != r.Target || pairs["-testPlatform"] != r.Mode || pairs["-testFilter"] != r.Filter {
		return fmt.Errorf("Editor mode/target/filter/run flags differ")
	}
	return nil
}

// LoadUnityNUnitEvidence reads only local artifacts. A complete receipt without
// a matching standard test-kit envelope is refused, never promoted from XML.
func LoadUnityNUnitEvidence(ctx context.Context, root, xmlPath, receiptPath string) (UnityEvidence, error) {
	refuse := func(err error) (UnityEvidence, error) { return UnityEvidence{}, unityError("%v", err) }
	actualRoot, err := filepath.Abs(root)
	if err != nil {
		return refuse(err)
	}
	actualRoot = filepath.Clean(actualRoot)
	if !unitySafeText(actualRoot) {
		return refuse(fmt.Errorf("unsafe target root"))
	}
	raw, err := unityRead(receiptPath, unityJSONLimit)
	if err != nil {
		return refuse(err)
	}
	var required map[string]json.RawMessage
	if err = json.Unmarshal(raw, &required); err != nil {
		return refuse(err)
	}
	for _, key := range []string{"schema", "status", "run_id", "root", "project", "mode", "target", "filter", "started_at", "finished_at", "producer", "editor", "command", "exit_code", "xml", "source", "attestation"} {
		if _, present := required[key]; !present {
			return refuse(fmt.Errorf("complete receipt missing %s", key))
		}
	}
	if filter := bytes.TrimSpace(required["filter"]); len(filter) == 0 || filter[0] != '"' {
		return refuse(fmt.Errorf("filter must be an explicit string"))
	}
	var r UnityReceipt
	if err = unityDecode(raw, &r); err != nil {
		return refuse(err)
	}
	if r.Schema != UnityReceiptSchema || r.Status != "complete" {
		return refuse(fmt.Errorf("receipt schema/status is not complete native v1"))
	}
	if r.Root != actualRoot || !filepath.IsAbs(r.Root) {
		return refuse(fmt.Errorf("receipt root differs from target root"))
	}
	if _, err = unityPath(actualRoot, r.Project, false); err != nil {
		return refuse(err)
	}
	expectedActive := filepath.Join(actualRoot, ".artifacts", "unity-tests", "active", filepath.FromSlash(r.Project), r.Mode+".json")
	requestedReceipt, err := filepath.Abs(receiptPath)
	if err != nil || filepath.Clean(requestedReceipt) != expectedActive {
		return refuse(fmt.Errorf("execution admission requires the active project/mode receipt"))
	}
	if r.RunID == "" || len(r.RunID) > 256 || !unitySafeText(r.RunID) || strings.ContainsAny(r.RunID, "/\\") {
		return refuse(fmt.Errorf("invalid native run id"))
	}
	if r.Mode != "EditMode" && r.Mode != "PlayMode" {
		return refuse(fmt.Errorf("unsupported Unity mode"))
	}
	if !slices.Contains([]string{"StandaloneOSX", "StandaloneWindows64", "StandaloneLinux64", "WebGL"}, r.Target) {
		return refuse(fmt.Errorf("unsupported selected target"))
	}
	if !unitySafeText(r.Filter) || r.ExitCode == nil {
		return refuse(fmt.Errorf("missing exit code/unsafe filter"))
	}
	if r.Producer.Name != "unity-local-runner" || r.Producer.Version != "1" || r.Editor.Version == "" {
		return refuse(fmt.Errorf("unsupported/missing producer or Editor version"))
	}
	if err = unityValidateCommand(actualRoot, r); err != nil {
		return refuse(err)
	}
	versionPath := filepath.Join(actualRoot, filepath.FromSlash(r.Project), "ProjectSettings", "ProjectVersion.txt")
	version, versionErr := unityRead(versionPath, 1<<20)
	if versionErr != nil || !slices.Contains(strings.Split(string(version), "\n"), "m_EditorVersion: "+r.Editor.Version) {
		return refuse(fmt.Errorf("declared Editor differs from the project version"))
	}
	if err = unityValidateSource(ctx, actualRoot, r); err != nil {
		return refuse(err)
	}
	xmlRaw, err := unityFileBytes(actualRoot, r.XML, 16<<20)
	if err != nil {
		return refuse(err)
	}
	resolvedXML, _ := unityPath(actualRoot, r.XML.Path, true)
	requestedXML, err := unityPath(actualRoot, xmlPath, true)
	if err != nil || filepath.Clean(requestedXML) != resolvedXML {
		return refuse(fmt.Errorf("requested XML differs from receipt"))
	}
	native, err := testrun.ParseUnityNUnitXML(xmlRaw)
	if err != nil {
		return refuse(err)
	}
	if native.Total == 0 {
		return refuse(fmt.Errorf("native producer record contains no executed/discovered cases"))
	}

	start, err := time.Parse(time.RFC3339Nano, r.StartedAt)
	if err != nil {
		return refuse(err)
	}
	finish, err := time.Parse(time.RFC3339Nano, r.FinishedAt)
	if err != nil || finish.Before(start) {
		return refuse(fmt.Errorf("invalid producer interval"))
	}
	nativeStart, _ := testrun.NUnitTime(native.StartedAt)
	nativeEnd, _ := testrun.NUnitTime(native.FinishedAt)
	if nativeStart.Before(start.Add(-time.Second)) || nativeEnd.After(finish.Add(time.Second)) {
		return refuse(fmt.Errorf("native XML lies outside producer interval"))
	}
	if native.Result == "Passed" && *r.ExitCode != 0 {
		return refuse(fmt.Errorf("passing XML contradicts nonzero Editor exit"))
	}
	envelope, err := unityFileBytes(actualRoot, r.Attestation, unityJSONLimit)
	if err != nil {
		return refuse(fmt.Errorf("mandatory test-kit envelope: %w", err))
	}
	run, err := unityValidateEnvelope(envelope, r, native)
	if err != nil {
		return refuse(err)
	}
	if r.Symbols != nil {
		symbols := r.Symbols
		indexRaw, err := unityFileBytes(actualRoot, UnityFile{symbols.SCIPPath, symbols.SCIPSHA256}, 256<<20)
		if err != nil {
			return refuse(err)
		}
		indexPath, _ := unityPath(actualRoot, symbols.SCIPPath, true)
		if _, err = unityFileBytes(actualRoot, UnityFile{symbols.SCIPPath + ".receipt.json", symbols.SourceReceiptSHA256}, 8<<20); err != nil {
			return refuse(err)
		}
		if _, err = scip.ValidateSourceReceipt(ctx, actualRoot, indexPath, indexRaw, true); err != nil {
			return refuse(err)
		}
	}
	receiptAbsolute, err := filepath.Abs(receiptPath)
	if err != nil {
		return refuse(err)
	}
	return UnityEvidence{Receipt: r, Native: native, Run: run, ReceiptPath: receiptAbsolute, ReceiptSHA256: unityDigest(raw)}, nil
}
