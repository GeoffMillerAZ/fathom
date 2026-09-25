package hostwire

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

const testOrigin = "http://127.0.0.1:41237"

var testToken = base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0xa5}, 32))

func testLaunch() string { return testOrigin + launchMarker + testToken }

func allMessages() []Message {
	return []Message{
		{Version: 1, Kind: "init", Origin: testOrigin, Launch: testLaunch()},
		{Version: 1, Kind: "launch", Launch: testLaunch()},
		{Version: 1, Kind: "ready"},
		{Version: 1, Kind: "launch-request"},
		{Version: 1, Kind: "browser"},
		{Version: 1, Kind: "browser-ok"},
		{Version: 1, Kind: "browser-failed"},
		{Version: 1, Kind: "closed"},
	}
}

func TestRoundTripEveryKind(t *testing.T) {
	left, right := net.Pipe()
	writer, reader := New(left), New(right)
	t.Cleanup(func() { _ = writer.Close(); _ = reader.Close() })

	for _, want := range allMessages() {
		want := want
		t.Run(want.Kind, func(t *testing.T) {
			done := make(chan error, 1)
			go func() { done <- writer.Write(want) }()
			got, err := reader.Read(time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("got %#v, want %#v", got, want)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReadRejectsInvalidJSONMessages(t *testing.T) {
	validLaunch := testLaunch()
	tests := []struct {
		name    string
		payload string
	}{
		{"unknown field", `{"version":1,"kind":"ready","extra":true}`},
		{"duplicate field", `{"version":1,"version":1,"kind":"ready"}`},
		{"wrong version", `{"version":2,"kind":"ready"}`},
		{"unknown kind", `{"version":1,"kind":"future"}`},
		{"noncanonical whitespace", `{"version": 1,"kind":"ready"}`},
		{"noncanonical order", `{"kind":"ready","version":1}`},
		{"unexpected origin", `{"version":1,"kind":"ready","origin":"` + testOrigin + `"}`},
		{"unexpected launch", `{"version":1,"kind":"closed","launch":"` + validLaunch + `"}`},
		{"init missing launch", `{"version":1,"kind":"init","origin":"` + testOrigin + `"}`},
		{"launch includes origin", `{"version":1,"kind":"launch","origin":"` + testOrigin + `","launch":"` + validLaunch + `"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := readRaw(t, []byte(test.payload)); err == nil {
				t.Fatal("accepted invalid message")
			}
		})
	}
}

func TestReadRejectsInvalidFrames(t *testing.T) {
	t.Run("oversized", func(t *testing.T) {
		left, right := net.Pipe()
		defer left.Close()
		defer right.Close()
		go func() {
			var header [4]byte
			binary.BigEndian.PutUint32(header[:], maxFrameSize+1)
			_, _ = left.Write(header[:])
		}()
		if _, err := New(right).Read(time.Second); err == nil {
			t.Fatal("accepted oversized frame")
		}
	})
	t.Run("truncated header", func(t *testing.T) {
		left, right := net.Pipe()
		go func() { _, _ = left.Write([]byte{0, 0}); _ = left.Close() }()
		defer right.Close()
		if _, err := New(right).Read(time.Second); err == nil {
			t.Fatal("accepted truncated header")
		}
	})
	t.Run("truncated payload", func(t *testing.T) {
		left, right := net.Pipe()
		go func() {
			var header [4]byte
			binary.BigEndian.PutUint32(header[:], 100)
			_, _ = left.Write(header[:])
			_, _ = left.Write([]byte(`{"version":1}`))
			_ = left.Close()
		}()
		defer right.Close()
		if _, err := New(right).Read(time.Second); err == nil {
			t.Fatal("accepted truncated payload")
		}
	})
}

func TestOriginAndLaunchValidation(t *testing.T) {
	validOrigins := []string{"http://127.0.0.1:1", testOrigin, "http://127.0.0.1:65535"}
	for _, origin := range validOrigins {
		if err := ValidateOrigin(origin); err != nil {
			t.Fatalf("rejected valid origin %q", origin)
		}
	}
	invalidOrigins := []string{"", "http://127.0.0.1", "http://127.0.0.1:0", "http://127.0.0.1:65536", "http://127.0.0.1:01", "http://127.0.0.1:+1", "http://localhost:80", "https://127.0.0.1:80", "http://[::1]:80", testOrigin + "/", testOrigin + "?x=1", "http://user@127.0.0.1:80"}
	for _, origin := range invalidOrigins {
		if err := ValidateOrigin(origin); err == nil {
			t.Fatalf("accepted invalid origin %q", origin)
		}
	}

	if err := ValidateLaunch(testOrigin, testLaunch()); err != nil {
		t.Fatal("rejected valid launch")
	}
	bad := []string{
		testOrigin + "/auth/launch", testOrigin + "/auth/launch?token=", testOrigin + "/auth/launch?token=" + strings.Repeat("A", 42),
		testOrigin + "/auth/launch?token=" + strings.Repeat("A", 44), testLaunch() + "=", testLaunch() + "&extra=1",
		testOrigin + "/auth/launch?token=" + strings.Repeat("+", 43), strings.Replace(testLaunch(), "token=", "%74oken=", 1),
		"http://127.0.0.1:41238/auth/launch?token=" + testToken,
	}
	for _, launch := range bad {
		if err := ValidateLaunch(testOrigin, launch); err == nil {
			t.Fatal("accepted invalid launch")
		}
	}
}

func TestReadTimeout(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	started := time.Now()
	if _, err := New(right).Read(20 * time.Millisecond); err == nil {
		t.Fatal("read did not time out")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("timeout took %v", elapsed)
	}
}

func TestConcurrentWritesRemainFramed(t *testing.T) {
	left, right := net.Pipe()
	writer, reader := New(left), New(right)
	defer writer.Close()
	defer reader.Close()
	messages := allMessages()
	const repeats = 25

	var writes sync.WaitGroup
	for i := 0; i < repeats; i++ {
		for _, message := range messages {
			message := message
			writes.Add(1)
			go func() {
				defer writes.Done()
				if err := writer.Write(message); err != nil {
					t.Errorf("write: %v", err)
				}
			}()
		}
	}
	for i := 0; i < repeats*len(messages); i++ {
		if _, err := reader.Read(time.Second); err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
	}
	writes.Wait()
}

func TestErrorsDoNotExposeSecrets(t *testing.T) {
	secret := strings.Repeat("s", 43)
	payload := []byte(`{"version":1,"kind":"init","origin":"http://attacker.invalid/` + secret + `","launch":"http://attacker.invalid/` + secret + `"}`)
	_, err := readRaw(t, payload)
	if err == nil || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "attacker") || strings.Contains(err.Error(), string(payload)) {
		t.Fatalf("error exposed message material: %q", err)
	}
	message := Message{Version: 1, Kind: "init", Origin: "http://attacker.invalid/" + secret, Launch: secret}
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	err = New(left).Write(message)
	if err == nil || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "attacker") {
		t.Fatalf("write error exposed message material: %q", err)
	}
}

func TestWriteRejectsInvalidMessages(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	for _, message := range []Message{
		{}, {Version: 2, Kind: "ready"}, {Version: 1, Kind: "unknown"},
		{Version: 1, Kind: "init", Origin: testOrigin},
		{Version: 1, Kind: "launch", Launch: "invalid"},
		{Version: 1, Kind: "browser", Origin: testOrigin},
	} {
		if err := New(left).Write(message); err == nil {
			t.Fatalf("accepted invalid message %#v", message)
		}
	}
}

func readRaw(t *testing.T, payload []byte) (Message, error) {
	t.Helper()
	left, right := net.Pipe()
	go func() {
		defer left.Close()
		var header [4]byte
		binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
		_, _ = left.Write(header[:])
		_, _ = io.Copy(left, bytes.NewReader(payload))
	}()
	defer right.Close()
	return New(right).Read(time.Second)
}
