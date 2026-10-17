package facts

import (
	"context"
	"time"
)

func fetch(ctx context.Context, k string) error { return ctx.Err() }

func fetchNoCtx(k string) error { return nil }

type getter interface {
	Get(ctx context.Context) error
}

// want: unchecked=no wrapped=- executes_only=- locked=- threaded=yes
// A context derived from the parameter is the parameter threaded on.
func threads(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	return fetch(ctx, "a")
}

// want: unchecked=no wrapped=- executes_only=- locked=- threaded=no
func detaches(ctx context.Context) error {
	return fetch(context.Background(), "a")
}

// want: unchecked=no wrapped=- executes_only=- locked=- threaded=no
// One of two calls drops it for context.TODO().
func partly(ctx context.Context) error {
	if err := fetch(ctx, "a"); err != nil {
		return err
	}
	return fetch(context.TODO(), "b")
}

// want: unchecked=no wrapped=- executes_only=- locked=- threaded=-
func noParam() error {
	return fetch(context.Background(), "a")
}

// want: unchecked=no wrapped=- executes_only=- locked=- threaded=-
// It has a context but calls nothing that accepts one.
func unused(ctx context.Context) error {
	return fetchNoCtx("a")
}

// want: unchecked=no wrapped=- executes_only=- locked=- threaded=yes
func inClosure(ctx context.Context) func() error {
	return func() error { return fetch(ctx, "x") }
}

// want: unchecked=no wrapped=- executes_only=- locked=- threaded=no
// Tricky negative: a local from context.Background() is not the parameter.
func shadows(ctx context.Context) error {
	bg := context.Background()
	return fetch(bg, "a")
}

// want: unchecked=no wrapped=- executes_only=- locked=- threaded=yes
func viaInterface(ctx context.Context, g getter) error {
	return g.Get(ctx)
}

// want: unchecked=no wrapped=- executes_only=- locked=- threaded=no
// An unnamed context parameter cannot be passed on.
func unnamedParam(_ context.Context, g getter) error {
	return g.Get(context.Background())
}
