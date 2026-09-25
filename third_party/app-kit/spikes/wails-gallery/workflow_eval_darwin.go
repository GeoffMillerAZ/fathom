//go:build darwin && !server && (workflowprobe || capabilityprobe) && appkit_contained

package main

/*
#cgo CFLAGS: -mmacosx-version-min=10.13 -x objective-c -fblocks
#cgo LDFLAGS: -framework Cocoa -framework WebKit

#include <stdint.h>
#include <stdlib.h>

int workflowEvaluateJavaScript(void *nativeWindow, const char *script, uint64_t callbackID);
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	workflowEvalMaxBytes = 64 << 10
	workflowEvalTimeout  = 5 * time.Second
)

type workflowEvalResult struct {
	value string
	err   error
}

var workflowEvalRegistry = struct {
	sync.Mutex
	next  atomic.Uint64
	calls map[uint64]chan workflowEvalResult
}{calls: make(map[uint64]chan workflowEvalResult)}

// workflowNativeEval is test-only instrumentation. The caller must pass a live,
// unclosed macOS window and fixed compiled JavaScript that returns a JSON string.
func workflowNativeEval(ctx context.Context, window *application.WebviewWindow, script string) (string, error) {
	if ctx == nil || window == nil {
		return "", errors.New("native workflow evaluation requires a context and live window")
	}
	if script == "" || len(script) > workflowEvalMaxBytes || !utf8.ValidString(script) || strings.IndexByte(script, 0) >= 0 {
		return "", errors.New("native workflow evaluation script is invalid")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	callID := workflowEvalRegistry.next.Add(1)
	if callID == 0 {
		callID = workflowEvalRegistry.next.Add(1)
	}
	completed := make(chan workflowEvalResult, 1)
	workflowEvalRegistry.Lock()
	workflowEvalRegistry.calls[callID] = completed
	workflowEvalRegistry.Unlock()
	remove := func() {
		workflowEvalRegistry.Lock()
		delete(workflowEvalRegistry.calls, callID)
		workflowEvalRegistry.Unlock()
	}

	waitCtx, cancel := context.WithTimeout(ctx, workflowEvalTimeout)
	defer cancel()
	// Include main-thread dispatch in the timeout. The queued closure owns its C
	// allocation and checks cancellation on the main thread before it can touch
	// the window; a delayed task therefore cannot evaluate against a later DOM.
	application.InvokeAsync(func() {
		if err := waitCtx.Err(); err != nil {
			workflowEvalDeliver(callID, workflowEvalResult{err: err})
			return
		}
		cScript := C.CString(script)
		defer C.free(unsafe.Pointer(cScript))
		if status := C.workflowEvaluateJavaScript(window.NativeWindow(), cScript, C.uint64_t(callID)); status != 0 {
			workflowEvalDeliver(callID, workflowEvalResult{err: workflowEvalScheduleError(int(status))})
		}
	})
	select {
	case result := <-completed:
		if result.err == nil && !json.Valid([]byte(result.value)) {
			return "", errors.New("native workflow evaluation did not return valid JSON")
		}
		return result.value, result.err
	case <-waitCtx.Done():
		remove()
		return "", fmt.Errorf("native workflow evaluation did not complete: %w", waitCtx.Err())
	}
}

func workflowEvalScheduleError(status int) error {
	switch status {
	case 1:
		return errors.New("native workflow window is unavailable")
	case 2:
		return errors.New("native workflow WKWebView is missing")
	case 3:
		return errors.New("native workflow window contains multiple WKWebViews")
	case 4:
		return errors.New("native workflow JavaScript could not be decoded")
	default:
		return errors.New("native workflow evaluation could not be scheduled")
	}
}

//export workflowEvalComplete
func workflowEvalComplete(callID C.uint64_t, value *C.char, valueLen C.size_t, domain *C.char, code C.long, status C.int) {
	id := uint64(callID)
	result := workflowEvalResult{}
	switch int(status) {
	case 0:
		if value == nil || uint64(valueLen) > workflowEvalMaxBytes {
			result.err = errors.New("native workflow evaluation returned invalid data")
		} else {
			result.value = C.GoStringN(value, C.int(valueLen))
		}
	case 1:
		result.err = fmt.Errorf("native workflow JavaScript failed (%s/%d)", workflowEvalErrorDomain(domain), int64(code))
	case 2:
		result.err = errors.New("native workflow JavaScript did not return a string")
	case 3:
		result.err = errors.New("native workflow JavaScript result exceeded 64 KiB")
	default:
		result.err = errors.New("native workflow JavaScript evaluation failed")
	}
	workflowEvalDeliver(id, result)
}

func workflowEvalDeliver(id uint64, result workflowEvalResult) {
	workflowEvalRegistry.Lock()
	completed := workflowEvalRegistry.calls[id]
	if completed != nil {
		delete(workflowEvalRegistry.calls, id)
	}
	workflowEvalRegistry.Unlock()
	if completed == nil {
		return // Cancellation, timeout, or a duplicate/late native completion.
	}
	select {
	case completed <- result:
	default:
	}
}

func workflowEvalErrorDomain(value *C.char) string {
	if value == nil {
		return "unknown"
	}
	domain := C.GoString(value)
	if domain == "" || len(domain) > 128 {
		return "unknown"
	}
	for _, r := range domain {
		if r > 127 || !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_') {
			return "unknown"
		}
	}
	return domain
}
