// Package runtimex holds small process-lifecycle helpers shared by every
// service entrypoint (signal handling, concurrent run groups).
package runtimex

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// SignalContext returns a context cancelled on SIGINT or SIGTERM, enabling
// graceful shutdown across all services.
func SignalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// RunGroup runs several long-lived functions concurrently and returns the first
// non-nil error. All functions share the same context; callers should cancel it
// to stop the group.
func RunGroup(ctx context.Context, fns ...func(context.Context) error) error {
	var (
		wg       sync.WaitGroup
		once     sync.Once
		firstErr error
	)
	for _, fn := range fns {
		wg.Add(1)
		go func(f func(context.Context) error) {
			defer wg.Done()
			if err := f(ctx); err != nil && err != context.Canceled {
				once.Do(func() { firstErr = err })
			}
		}(fn)
	}
	wg.Wait()
	return firstErr
}
