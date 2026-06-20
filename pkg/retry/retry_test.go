package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Egooroh/beacon/pkg/retry"
)

func TestDo_SucceedsImmediately(t *testing.T) {
	calls := 0
	err := retry.Do(context.Background(), 3, time.Millisecond, func() error {
		calls++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestDo_RetriesOnError(t *testing.T) {
	calls := 0
	sentinel := errors.New("transient")
	err := retry.Do(context.Background(), 3, time.Millisecond, func() error {
		calls++
		if calls < 3 {
			return sentinel
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestDo_ExhaustsAttempts(t *testing.T) {
	sentinel := errors.New("permanent")
	err := retry.Do(context.Background(), 3, time.Millisecond, func() error {
		return sentinel
	})
	assert.ErrorIs(t, err, sentinel)
}

func TestDo_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	err := retry.Do(ctx, 5, time.Second, func() error {
		calls++
		return errors.New("fail")
	})
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, calls)
}
