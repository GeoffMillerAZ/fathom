package client

import (
	"strings"
	"sync"
	"time"
)

// ObserveReadsUnder records every path under prefix that readFileCapped
// opens until the returned stop func is called, which returns them in
// order. Paths outside prefix (a parallel test's temp dir) are ignored.
func ObserveReadsUnder(prefix string) (stop func() []string) {
	var mu sync.Mutex
	var got []string
	fn := func(path string) {
		if !strings.HasPrefix(path, prefix) {
			return
		}
		mu.Lock()
		got = append(got, path)
		mu.Unlock()
	}
	readObserver.Store(&fn)
	return func() []string {
		readObserver.Store(nil)
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), got...)
	}
}

// ObserveReadsUnderSlow is ObserveReadsUnder with every observed read held
// for delay before it opens: concurrent callers pile up behind the first.
func ObserveReadsUnderSlow(prefix string, delay time.Duration) (stop func() []string) {
	var mu sync.Mutex
	var got []string
	fn := func(path string) {
		if !strings.HasPrefix(path, prefix) {
			return
		}
		mu.Lock()
		got = append(got, path)
		mu.Unlock()
		time.Sleep(delay)
	}
	readObserver.Store(&fn)
	return func() []string {
		readObserver.Store(nil)
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), got...)
	}
}
