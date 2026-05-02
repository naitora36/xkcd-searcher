package initiator

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	mock_core "yadro.com/course/isearch/core/mocks"
)

func TestInitiator_Tick(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSearcher := mock_core.NewMockSearcher(ctrl)

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	duration := 10 * time.Millisecond
	initiator := NewInitiator(duration, mockSearcher)
	defer initiator.StopTicker()

	mockSearcher.
		EXPECT().
		BuildIndex(gomock.Any()).
		Return(nil).
		MinTimes(2)

	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(context.Background())

	wg.Go(func() {
		initiator.Tick(ctx, logger)
	})

	time.Sleep(time.Millisecond * 50)

	cancel()

	wg.Wait()

	require.Contains(t, buf.String(), "in timer context canceled")
}

func TestInitiator_Tick_ErrorHandling(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSearcher := mock_core.NewMockSearcher(ctrl)

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	mockSearcher.EXPECT().
		BuildIndex(gomock.Any()).
		Return(fmt.Errorf("db connection failed")).
		AnyTimes()

	duration := 10 * time.Millisecond
	initiator := NewInitiator(duration, mockSearcher)
	defer initiator.StopTicker()

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	defer cancel()

	initiator.Tick(ctx, logger)

	require.Contains(t, buf.String(), "build index error")
	require.Contains(t, buf.String(), "db connection failed")
}
