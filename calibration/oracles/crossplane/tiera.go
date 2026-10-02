package main

// tiera.go — the tier-A definitions (wave ox, LB-F157).
//
// `crossplane resource validate` (crossplane CLI 2.5.0; `crossplane beta
// validate` before 2.x) validates each Crossplane file offline against
// Crossplane's own CRDs (cluster/crds of crossplane/crossplane at the
// subject's tag), with the Kubernetes API server's validation library and
// unknown-field detection; --crossplane-image="" keeps it from pulling
// Crossplane's package from a registry, and a dead proxy proves nothing else
// is fetched. A file is accepted when no resource is invalid and every
// Crossplane object of the file validates; a missing schema is tolerated
// only for a pipeline step's function input (validate reports it as a
// resource of the function's group: not one of the file's objects). A file
// refused (an invalid object, an object with no schema here such as package
// metadata) contributes nothing and is counted. The entities of an accepted file are its schema-valid objects'
// (the object, an XRD's composite and claim kinds and versions, a pipeline
// step), read from the fields the schema validated. A composed resource
// (`element`, a function input's resources[]) sits in a function's input,
// which Crossplane's CRDs leave to the function's own schema: it is
// unevidenced, counted, and in no truth. References and cross_references stay
// the walk's, tier C: validate checks no compositeTypeRef or functionRef.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// report is validate's -o json output.
type report struct {
	Resources []struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Name       string `json:"name"`
		Status     string `json:"status"`
	} `json:"resources"`
}

// verdict reads validate's JSON report for a file whose Crossplane objects
// are objs (Kind/[namespace/]name): accepted when no resource is invalid and
// every object is valid. inputs counts the missing schemas tolerated (what is
// not one of the file's objects: a function's input).
func verdict(b []byte, objs []string) (ok bool, why string, inputs int) {
	var r report
	if err := json.Unmarshal(b, &r); err != nil {
		return false, "unreadable report: " + err.Error(), 0
	}
	valid := map[string]bool{}
	for _, x := range r.Resources {
		switch x.Status {
		case "valid":
			valid[x.Kind+"|"+x.Name] = true
		case "missingSchema":
			inputs++
		default:
			return false, fmt.Sprintf("%s %s %s: %s", x.APIVersion, x.Kind, x.Name, x.Status), 0
		}
	}
	for _, o := range objs {
		parts := strings.Split(o, "/")
		if !valid[parts[0]+"|"+parts[len(parts)-1]] {
			return false, "no valid result for " + o, 0
		}
	}
	// every object is valid, so each missing schema is something else: a
	// function's input
	return true, "", inputs
}

// validate runs crossplane resource validate on one file, offline, and
// returns how many function inputs it could not validate.
func validate(bin, crds, cache, file string, objs []string) (int, error) {
	var last error
	for i := 0; i < 6; i++ {
		cmd := exec.Command(bin, "resource", "validate", "--crossplane-image=", "--cache-dir", cache, "-o", "json", crds, file)
		cmd.Env = append(os.Environ(), "HTTPS_PROXY=http://127.0.0.1:9", "HTTP_PROXY=http://127.0.0.1:9", "KUBECONFIG=/dev/null")
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		_ = cmd.Run()
		ok, why, inputs := verdict(stdout.Bytes(), objs)
		if ok {
			return inputs, nil
		}
		last = fmt.Errorf("%s %s", why, strings.Join(strings.Fields(stderr.String()), " "))
		if !strings.Contains(strings.ToLower(stdout.String()+stderr.String()), "operation not permitted") {
			return 0, last
		}
		if c := os.Getenv("ORACLE_TCC_RESET"); c != "" {
			_ = exec.Command("sh", "-c", c).Run()
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	return 0, last
}

// firstMark is validate's first failure or missing-schema line.
func firstMark(out string) string {
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "[x]") || strings.HasPrefix(l, "[!]") {
			return l
		}
	}
	return strings.TrimSpace(out)
}

// writeTierA writes the tier-A definitions to stdout, the walk's tier-C
// references and cross_references to refsOut, the accepted files to filesOut.
func writeTierA(o out, files []string, subject, bin, crds, scratch, refsOut, filesOut string) error {
	ver, err := exec.Command(bin, "version", "--client").Output()
	if err != nil {
		return fmt.Errorf("crossplane version: %w", err)
	}
	cache := filepath.Join(scratch, "xp-cache")
	accepted := map[string]bool{}
	var acc []string
	reasons := map[string]int{}
	objsOf := map[string][]string{}
	for _, d := range o.Definitions {
		if d.Kind == "resource" {
			objsOf[d.Path] = append(objsOf[d.Path], d.Name)
		}
	}
	inputs := 0
	for _, rel := range files {
		n, err := validate(bin, crds, cache, filepath.Join(subject, filepath.FromSlash(rel)), objsOf[rel])
		if err != nil {
			why := err.Error()
			switch {
			case strings.Contains(why, "no valid result"):
				reasons["object-without-schema"]++
			case strings.Contains(why, "invalid"):
				reasons["invalid"]++
			default:
				reasons["other"]++
			}
			fmt.Fprintf(os.Stderr, "crossplane-oracle: validate refuses %s: %.240s\n", rel, why)
			continue
		}
		inputs += n
		accepted[rel] = true
		acc = append(acc, rel)
	}
	var defs []def
	elements := 0
	for _, d := range o.Definitions {
		if !accepted[d.Path] {
			continue
		}
		if d.Kind == "element" {
			elements++
			continue
		}
		defs = append(defs, d)
	}
	fmt.Fprintf(os.Stderr, "crossplane-oracle: tier A (%s): %d crossplane files, %d validated, refused %v; %d function inputs without a schema (tolerated: not the file's objects); %d definitions; %d composed resources in function inputs unevidenced (the function's schema needs a registry)\n",
		strings.Join(strings.Fields(string(ver)), " "), len(files), len(acc), reasons, inputs, len(defs), elements)
	if defs == nil {
		defs = []def{}
	}
	type defsOut struct {
		Tool        string `json:"tool"`
		Version     string `json:"version"`
		Tier        string `json:"tier"`
		Definitions []def  `json:"definitions"`
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", " ")
	if err := enc.Encode(defsOut{
		Tool:        "crossplane resource validate, offline, against crossplane's own CRDs (cluster/crds at the subject's tag); lines: gopkg.in/yaml.v3",
		Version:     strings.Join(strings.Fields(string(ver)), " "),
		Tier:        "A",
		Definitions: defs,
	}); err != nil {
		return err
	}
	o.Definitions = []def{}
	var buf bytes.Buffer
	renc := json.NewEncoder(&buf)
	renc.SetIndent("", " ")
	if err := renc.Encode(o); err != nil {
		return err
	}
	if err := os.WriteFile(refsOut, buf.Bytes(), 0o644); err != nil {
		return err
	}
	if filesOut != "" {
		return os.WriteFile(filesOut, []byte(strings.Join(acc, "\n")+"\n"), 0o644)
	}
	return nil
}
