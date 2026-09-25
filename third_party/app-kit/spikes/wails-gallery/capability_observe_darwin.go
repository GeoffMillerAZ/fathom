//go:build darwin && !server && capabilityprobe && !workflowprobe && appkit_contained

package main

/*
#cgo CFLAGS: -mmacosx-version-min=10.13 -x objective-c -fblocks
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#include <stdlib.h>
char *capabilityObserve(void *nativeWindow, const char *origin, int preflight);
int capabilityObservationWireContract(void);
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// This observation is read from the actual native delegate, never page-authored
// state. Preflight calls only the denial method, with no OS/device request.
type capabilityNativeObservation struct {
	Available         bool      `json:"available"`
	OriginConfigured  bool      `json:"origin_configured"`
	Denials           [3]uint64 `json:"denials"`
	OriginMatched     bool      `json:"origin_matched"`
	PreflightDenied   [3]bool   `json:"preflight_denied"`
	PreflightCalls    [3]uint64 `json:"preflight_calls"`
	PreflightResults  [3]int64  `json:"preflight_results"`
	WindowVisible     bool      `json:"window_visible"`
	WindowKey         bool      `json:"window_key"`
	ApplicationActive bool      `json:"application_active"`
}

func capabilityWireContract() bool { return C.capabilityObservationWireContract() == 1 }

func observeCapability(ctx context.Context, window *application.WebviewWindow, origin string, preflight bool) (capabilityNativeObservation, error) {
	type result struct {
		observation capabilityNativeObservation
		err         error
	}
	finished := make(chan result, 1)
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	application.InvokeAsync(func() {
		if waitCtx.Err() != nil {
			return
		}
		cOrigin := C.CString(origin)
		defer C.free(unsafe.Pointer(cOrigin))
		flag := C.int(0)
		if preflight {
			flag = 1
		}
		value := C.capabilityObserve(window.NativeWindow(), cOrigin, flag)
		if value == nil {
			finished <- result{err: errors.New("native capability observation unavailable")}
			return
		}
		defer C.free(unsafe.Pointer(value))
		var r result
		r.err = json.Unmarshal([]byte(C.GoString(value)), &r.observation)
		finished <- r
	})
	select {
	case r := <-finished:
		return r.observation, r.err
	case <-waitCtx.Done():
		return capabilityNativeObservation{}, errors.New("native capability observation timed out")
	}
}
