//go:build !server && (windows || (darwin && !appkit_contained))

package main

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestUncontainedNativeLaunchFailsBeforeApplicationCreation(t *testing.T) {
	// A nil host is deliberate: no launch token or native app may be touched.
	err := runNative(context.Background(), nil, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "-browser") {
		t.Fatalf("missing safe recovery: %v", err)
	}
}
