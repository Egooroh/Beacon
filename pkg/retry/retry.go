// Package retry provides a simple fixed-delay retry helper.
package retry

import (
	"context"
	"time"
)

// Do calls fn up to attempts times, pausing delay between retries.
// It stops immediately when ctx is cancelled or fn returns nil.
func Do(ctx context.Context, attempts int, delay time.Duration, fn func() error) error {
	var err error
	for i := range attempts {
		if err = fn(); err == nil {
			return nil
		}
		if i < attempts-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return err
}
