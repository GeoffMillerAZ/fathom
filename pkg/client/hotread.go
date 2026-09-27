package client

import (
	"errors"

	"context"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/geoffmilleraz/fathom/internal/corpus"
)

// HotPathReadCap is the largest file a request-path read in this package
// loads (docs/design/responsible-tenant.md §4.2–§4.3, invariant I38): a
// file over it is stat'ed, never read, and the skip is named to the caller
// as a partial answer, never passed off as absent or fresh.
const HotPathReadCap int64 = 2 << 20

// readObserver, when set, is told every path readFileCapped actually
// opens for reading — the tests' proof that a skipped file was never read
// and that a whole-tree walk read no contents. nil in production.
var readObserver atomic.Pointer[func(path string)]

// readFileCapped reads path through internal/corpus (RT-66, I48): stat
// first, read only a regular file of at most max bytes that does not
// sniff as binary, and charge every byte read to ctx's admission ticket.
// skipped names why a file was not read (and data is nil); err is a stat
// or read failure (an unreadable file's own error, never a skip — I38), or
// the governor or meter refusing the read. A file that grew past max
// between the stat and the read is skipped too, never truncated.
func readFileCapped(ctx context.Context, path string, max int64) (data []byte, info os.FileInfo, skipped string, err error) {
	var onOpen func(string)
	if obs := readObserver.Load(); obs != nil {
		onOpen = *obs
	}
	data, info, ex, err := corpus.Read(ctx, path, max, onOpen)
	if err != nil {
		return nil, info, "", err
	}
	if ex != nil {
		switch ex.Reason {
		case corpus.ReasonUnreadable:
			return nil, info, "", ex.Err
		case corpus.ReasonNotRegular:
			return nil, info, "not a regular file", nil
		case corpus.ReasonOversize:
			return nil, info, oversizeNote(ex.Size, max), nil
		case corpus.ReasonBinary:
			return nil, info, "binary file, not read past its first " + humanBytes(corpus.SniffSize), nil
		default:
			return nil, info, ex.Reason, nil
		}
	}
	return data, info, "", nil
}

func oversizeNote(size, max int64) string {
	return fmt.Sprintf("file is %s, over the %s read cap", humanBytes(size), humanBytes(max))
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// flightDerive runs derive once among concurrent callers of key on this
// client (RT-95, I51): the obligations hot path's read-and-remember steps
// (the edited file's hash and tokens, a layer guide's content) each
// re-check their memo inside the flight, read on a miss and store what
// they derived before the flight ends, so a cold stampede reads each file
// once — charged to the ticket of the caller that read it — and a caller
// arriving after the flight finds the memo filled. A waiter whose own
// context is live when the shared derive failed on the reader's cancelled
// context derives for itself.
func (c *Client) flightDerive(ctx context.Context, key string, derive func() (any, error)) (any, error) {
	v, err, _ := c.hotReads.Do(key, derive)
	if err != nil && ctx.Err() == nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		return derive()
	}
	return v, err
}
