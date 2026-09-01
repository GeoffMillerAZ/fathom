package main

import "testing"

func TestLoopbackAddr(t *testing.T) {
	cases := []struct {
		name        string
		addr        string
		allowPublic bool
		want        string
		wantErr     bool
	}{
		{"bare port defaults to loopback", ":8765", false, "127.0.0.1:8765", false},
		{"explicit loopback ipv4", "127.0.0.1:8765", false, "127.0.0.1:8765", false},
		{"localhost name is loopback", "localhost:8765", false, "localhost:8765", false},
		{"loopback ipv6", "[::1]:8765", false, "[::1]:8765", false},
		{"public bind refused by default", "0.0.0.0:8765", false, "", true},
		{"private lan bind refused by default", "192.168.1.10:9000", false, "", true},
		{"public bind allowed with opt-in", "0.0.0.0:8765", true, "0.0.0.0:8765", false},
		{"private bind allowed with opt-in", "192.168.1.10:9000", true, "192.168.1.10:9000", false},
		{"missing port is an error", "127.0.0.1", false, "", true},
		{"empty is an error", "", false, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := loopbackAddr(c.addr, c.allowPublic)
			if c.wantErr {
				if err == nil {
					t.Fatalf("loopbackAddr(%q, %v) = %q, want error", c.addr, c.allowPublic, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("loopbackAddr(%q, %v): unexpected error %v", c.addr, c.allowPublic, err)
			}
			if got != c.want {
				t.Errorf("loopbackAddr(%q, %v) = %q, want %q", c.addr, c.allowPublic, got, c.want)
			}
		})
	}
}

func TestIsLoopbackHost(t *testing.T) {
	loopback := []string{"localhost", "127.0.0.1", "127.0.0.53", "::1"}
	for _, h := range loopback {
		if !isLoopbackHost(h) {
			t.Errorf("isLoopbackHost(%q) = false, want true", h)
		}
	}
	public := []string{"0.0.0.0", "192.168.1.10", "10.0.0.1", "example.com", "::"}
	for _, h := range public {
		if isLoopbackHost(h) {
			t.Errorf("isLoopbackHost(%q) = true, want false", h)
		}
	}
}
