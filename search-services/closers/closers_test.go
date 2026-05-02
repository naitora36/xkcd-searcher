package closers

import (
	"bytes"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockCloser struct {
	flag bool
	err  error
}

func (mc *mockCloser) Close() error {
	mc.flag = true
	return mc.err
}

func TestCloseOrLog(t *testing.T) {
	t.Run("should not log when close is successful", func(t *testing.T) {
		mcWithoutErr := &mockCloser{err: nil}

		var buf bytes.Buffer
		th := slog.NewTextHandler(&buf, nil)
		l := slog.New(th)

		CloseOrLog(mcWithoutErr, l)

		require.Equal(t, mcWithoutErr.flag, true)
		require.Equal(t, buf.Len(), 0)
	})
	t.Run("should log when close is fails", func(t *testing.T) {
		mcWithErr := &mockCloser{err: fmt.Errorf("my-error")}

		var buf bytes.Buffer
		th := slog.NewTextHandler(&buf, nil)
		l := slog.New(th)

		CloseOrLog(mcWithErr, l)

		require.Equal(t, mcWithErr.flag, true)
		require.Contains(t, buf.String(), "close failed")
		require.Contains(t, buf.String(), "my-error")
	})
}

func TestCloseOrPanic(t *testing.T) {
	t.Run("should not panic when no error", func(t *testing.T) {
		mcWithoutErr := &mockCloser{err: nil}

		defer func() {
			rec := recover()
			require.Equal(t, mcWithoutErr.flag, true)
			require.Equal(t, rec, nil)
		}()

		CloseOrPanic(mcWithoutErr)
	})
	t.Run("should panic when fails", func(t *testing.T) {
		mcWithErr := &mockCloser{err: fmt.Errorf("my-error")}

		defer func() {
			rec := recover()
			require.Equal(t, mcWithErr.flag, true)
			require.NotEqual(t, rec, nil)
			require.Contains(t, rec, "close failed")
			require.Contains(t, rec, "my-error")
		}()

		CloseOrPanic(mcWithErr)
	})
}
