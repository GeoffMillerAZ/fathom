package apikey_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision/internal/apikey"
)

// The fake keys below are deliberately short and shaped like no vendor's
// key.
const (
	fileKey = "fake-file-key"
	envKey  = "fake-env-key"
)

// writeKey writes content to a file named name in a fresh directory with
// mode, and returns its path.
func writeKey(t *testing.T, name, content string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}

// env is an injected environment.
func env(vars map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		v, ok := vars[name]
		return v, ok
	}
}

// noKeyIn fails t when err's text contains any of keys.
func noKeyIn(t *testing.T, err error, keys ...string) {
	t.Helper()
	for _, k := range keys {
		if err != nil && strings.Contains(err.Error(), k) {
			t.Errorf("error %q contains a key", err.Error())
		}
	}
}

func TestResolveReadsFileOrEnvAndNamesTheSource(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes do not map onto Windows ACLs; the mode check is skipped there")
	}
	vars := map[string]string{"DECISION_TEST_KEY": "  " + envKey + "\n", "DECISION_TEST_EMPTY": " \n"}
	r := apikey.Resolver{LookupEnv: env(vars)}

	t.Run("a 0600 file is read and trimmed", func(t *testing.T) {
		path := writeKey(t, "token", "  "+fileKey+"\n", 0o600)
		got, err := r.Resolve(apikey.Source{File: path})
		if err != nil || got != fileKey {
			t.Fatalf("Resolve() = %q, %v; want the trimmed file key", got, err)
		}
	})

	t.Run("a 0400 file is read", func(t *testing.T) {
		path := writeKey(t, "token", fileKey, 0o400)
		if got, err := r.Resolve(apikey.Source{File: path}); err != nil || got != fileKey {
			t.Fatalf("Resolve() = %q, %v; want the file key", got, err)
		}
	})

	for _, mode := range []os.FileMode{0o644, 0o640, 0o604, 0o660} {
		t.Run("a "+mode.String()+" file is refused with the fix", func(t *testing.T) {
			path := writeKey(t, "token", fileKey, mode)
			_, err := r.Resolve(apikey.Source{File: path})
			if err == nil {
				t.Fatal("Resolve() error = nil for a key file other users can access")
			}
			if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "chmod 600 "+path) {
				t.Errorf("error %q does not name the path and the fix", err)
			}
			noKeyIn(t, err, fileKey)
		})
	}

	t.Run("a file wins over an environment variable", func(t *testing.T) {
		path := writeKey(t, "token", fileKey, 0o600)
		if got, err := r.Resolve(apikey.Source{File: path, Env: "DECISION_TEST_KEY"}); err != nil || got != fileKey {
			t.Fatalf("Resolve() = %q, %v; want the file key", got, err)
		}
	})

	t.Run("a value set in code wins over both", func(t *testing.T) {
		path := writeKey(t, "token", fileKey, 0o600)
		if got, err := r.Resolve(apikey.Source{Value: "code-key", File: path, Env: "DECISION_TEST_KEY"}); err != nil || got != "code-key" {
			t.Fatalf("Resolve() = %q, %v; want the value", got, err)
		}
	})

	t.Run("an environment variable is read and trimmed", func(t *testing.T) {
		if got, err := r.Resolve(apikey.Source{Env: "DECISION_TEST_KEY"}); err != nil || got != envKey {
			t.Fatalf("Resolve() = %q, %v; want the trimmed env key", got, err)
		}
	})

	t.Run("~ expands to the home directory", func(t *testing.T) {
		home := t.TempDir()
		if err := os.MkdirAll(filepath.Join(home, "secrets", "provider"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, "secrets", "provider", "token"), []byte(fileKey), 0o600); err != nil {
			t.Fatal(err)
		}
		r := apikey.Resolver{HomeDir: func() (string, error) { return home, nil }}
		if got, err := r.Resolve(apikey.Source{File: "~/secrets/provider/token"}); err != nil || got != fileKey {
			t.Fatalf("Resolve() = %q, %v; want the key under the home directory", got, err)
		}
	})

	errorCases := []struct {
		name string
		src  func(t *testing.T) apikey.Source
		want []string
	}{
		{"no source", func(*testing.T) apikey.Source { return apikey.Source{} }, []string{"no API key configured"}},
		{"a missing file", func(t *testing.T) apikey.Source {
			return apikey.Source{File: filepath.Join(t.TempDir(), "absent")}
		}, []string{"absent", "no such file"}},
		{"an empty file", func(t *testing.T) apikey.Source {
			return apikey.Source{File: writeKey(t, "token", " \n", 0o600)}
		}, []string{"token", "is empty"}},
		{"a file with two lines", func(t *testing.T) apikey.Source {
			return apikey.Source{File: writeKey(t, "token", fileKey+"\n"+envKey+"\n", 0o600)}
		}, []string{"token", "exactly one key"}},
		{"a directory", func(t *testing.T) apikey.Source { return apikey.Source{File: t.TempDir()} }, []string{"not a regular file"}},
		{"an unset variable", func(*testing.T) apikey.Source { return apikey.Source{Env: "DECISION_TEST_UNSET"} }, []string{`"DECISION_TEST_UNSET" is not set`}},
		{"an empty variable", func(*testing.T) apikey.Source { return apikey.Source{Env: "DECISION_TEST_EMPTY"} }, []string{`"DECISION_TEST_EMPTY" is empty`}},
		{"an unreadable home", func(*testing.T) apikey.Source { return apikey.Source{File: "~/token"} }, []string{`"~/token"`, "expanding ~"}},
	}
	for _, c := range errorCases {
		t.Run(c.name+" is an error naming the source", func(t *testing.T) {
			r := r
			r.HomeDir = func() (string, error) { return "", errors.New("no home") }
			_, err := r.Resolve(c.src(t))
			if err == nil {
				t.Fatal("Resolve() error = nil")
			}
			for _, w := range c.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
			noKeyIn(t, err, fileKey, envKey)
		})
	}
}

func TestSourceStringNamesTheSourceNotTheKey(t *testing.T) {
	for _, c := range []struct {
		src  apikey.Source
		want string
	}{
		{apikey.Source{Value: "code-key", File: "f"}, "key set in code"},
		{apikey.Source{File: "~/secrets/provider/token", Env: "E"}, `key file "~/secrets/provider/token"`},
		{apikey.Source{Env: "PROVIDER_KEY"}, `environment variable "PROVIDER_KEY"`},
		{apikey.Source{}, "no key source"},
	} {
		if got := c.src.String(); got != c.want {
			t.Errorf("%+v.String() = %q, want %q", c.src, got, c.want)
		}
		if strings.Contains(c.src.String(), "code-key") {
			t.Errorf("String() contains the key: %q", c.src.String())
		}
		if got := c.src.Configured(); got != (c.want != "no key source") {
			t.Errorf("%+v.Configured() = %v", c.src, got)
		}
	}
}

func TestLazyReadsAtFirstUseAndRetriesAFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes do not map onto Windows ACLs")
	}
	path := writeKey(t, "token", fileKey, 0o644)
	l := apikey.NewLazy(apikey.Source{File: path}, apikey.Resolver{})
	if l.Source().File != path {
		t.Errorf("Source() = %+v", l.Source())
	}
	if _, err := l.Key(); err == nil || !strings.Contains(err.Error(), "chmod 600") {
		t.Fatalf("Key() error = %v, want the mode refusal", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := l.Key(); err != nil || got != fileKey {
		t.Fatalf("Key() after chmod 600 = %q, %v; want the key without a restart", got, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got, err := l.Key(); err != nil || got != fileKey {
				t.Errorf("Key() = %q, %v; want the key kept from first use", got, err)
			}
		}()
	}
	wg.Wait()
}

func TestRedactRemovesKeysAndAuthorizationEchoes(t *testing.T) {
	for _, c := range []struct {
		name, in string
		keys     []string
		gone     []string
	}{
		{"a known key", `{"error":"key fake-key-0001 rejected"}`, []string{"fake-key-0001"}, []string{"fake-key-0001"}},
		{"an echoed Authorization header", `{"error":{"message":"bad header Authorization: Bearer fake-key-0001"}}`, nil, []string{"fake-key-0001"}},
		{"an echoed JSON authorization field", `{"headers":{"authorization":"Bearer fake-key-0001"}}`, nil, []string{"fake-key-0001"}},
		{"a bare bearer token", `token was Bearer fake-key-0001.`, nil, []string{"fake-key-0001"}},
		{"a masked key echo", `Incorrect API key provided: sk-fake***0001. See the docs.`, nil, []string{"sk-fake", "0001"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := apikey.Redact(c.in, c.keys...)
			for _, g := range c.gone {
				if strings.Contains(got, g) {
					t.Errorf("Redact(%q) = %q, still contains %q", c.in, got, g)
				}
			}
			if !strings.Contains(got, apikey.Redacted) {
				t.Errorf("Redact(%q) = %q, want the redaction marker", c.in, got)
			}
		})
	}
	if got := apikey.Redact("plain provider text", ""); got != "plain provider text" {
		t.Errorf("Redact changed text with no secret: %q", got)
	}
}
