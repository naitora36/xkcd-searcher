package core_test

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"yadro.com/course/update/core"
	mock_core "yadro.com/course/update/core/mocks"
)

const (
	maxLenDescription = 3000
	eventUpdate       = "XKCD DB has been updated"
	eventDrop         = "XKCD DB has been dropped"
)

type mockFields struct {
	db     *mock_core.MockDB
	xkcd   *mock_core.MockXKCD
	words  *mock_core.MockWords
	broker *mock_core.MockBroker
}

func TestLimitWords(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		max    int
		expect string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello world", 11, "hello world"},
		{"cut at space", "hello world again", 10, "hello"},
		{"no space to cut", "helloworld", 5, "hello"},
		{"empty", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expect, core.LimitWords(tt.input, tt.max))
		})
	}
}

func TestService_Update_ConcurrencyLock(t *testing.T) {
	ctrl := gomock.NewController(t)

	f := mockFields{
		db:     mock_core.NewMockDB(ctrl),
		words:  mock_core.NewMockWords(ctrl),
		xkcd:   mock_core.NewMockXKCD(ctrl),
		broker: mock_core.NewMockBroker(ctrl),
	}

	logger := slog.New(slog.DiscardHandler)
	s, err := core.NewService(logger, f.db, f.xkcd, f.words, f.broker, 1)

	require.NoError(t, err, "Expected no error when build update service")

	block := make(chan struct{})
	var wg sync.WaitGroup

	f.db.EXPECT().IDs(gomock.Any()).DoAndReturn(func(ctx context.Context) ([]int, error) {
		<-block
		return nil, fmt.Errorf("stop first goroutine")
	})

	wg.Go(func() {
		err := s.Update(context.Background())
		logger.Error(err.Error())
	})

	time.Sleep(10 * time.Millisecond)

	err = s.Update(context.Background())
	require.ErrorIs(t, err, core.ErrAlreadyRunning)

	close(block)
	wg.Wait()
}

func TestService_FetchWithRetry_MaxAttempts(t *testing.T) {
	ctrl := gomock.NewController(t)
	f := mockFields{
		db:     mock_core.NewMockDB(ctrl),
		words:  mock_core.NewMockWords(ctrl),
		xkcd:   mock_core.NewMockXKCD(ctrl),
		broker: mock_core.NewMockBroker(ctrl),
	}

	logger := slog.New(slog.DiscardHandler)
	s, err := core.NewService(logger, f.db, f.xkcd, f.words, f.broker, 1)

	require.NoError(t, err, "Expected no error when build update service")

	f.xkcd.EXPECT().Get(gomock.Any(), 1).Return(core.XKCDInfo{}, fmt.Errorf("network error")).Times(3)

	_, err = s.FetchWithRetry(context.Background(), 1)

	require.Error(t, err)
	require.Contains(t, err.Error(), "all attempts failed")
}

func TestService_Update_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mf := mockFields{
		db:     mock_core.NewMockDB(ctrl),
		xkcd:   mock_core.NewMockXKCD(ctrl),
		words:  mock_core.NewMockWords(ctrl),
		broker: mock_core.NewMockBroker(ctrl),
	}

	logger := slog.New(slog.DiscardHandler)
	s, err := core.NewService(logger, mf.db, mf.xkcd, mf.words, mf.broker, 2)

	require.NoError(t, err, "Expected no error when build update service")

	mf.db.EXPECT().IDs(gomock.Any()).Return([]int{1}, nil)
	mf.xkcd.EXPECT().LastID(gomock.Any()).Return(2, nil)

	mf.xkcd.EXPECT().Get(gomock.Any(), 2).Return(core.XKCDInfo{ID: 2, Description: "test"}, nil)
	mf.words.EXPECT().Norm(gomock.Any(), "test").Return([]string{"test"}, nil)
	mf.db.EXPECT().Add(gomock.Any(), gomock.Any()).Return(nil)

	mf.broker.EXPECT().SendEvent(eventUpdate).Times(1)

	err = s.Update(context.Background())

	require.NoError(t, err, "Expected no error when update")
}

func TestService_ProcessOne(t *testing.T) {
	type mockFields struct {
		db    *mock_core.MockDB
		xkcd  *mock_core.MockXKCD
		words *mock_core.MockWords
	}

	tests := []struct {
		name        string
		id          int
		prepareMock func(f mockFields)
		expectedErr error
	}{
		{
			name: "success processing",
			id:   1,
			prepareMock: func(f mockFields) {
				f.xkcd.EXPECT().Get(gomock.Any(), 1).
					Return(core.XKCDInfo{ID: 1, URL: "url1", Description: "test desc"}, nil)

				f.words.EXPECT().Norm(gomock.Any(), "test desc").
					Return([]string{"test", "desc"}, nil)

				f.db.EXPECT().Add(gomock.Any(), core.Comics{
					ID: 1, URL: "url1", Words: []string{"test", "desc"},
				}).Return(nil)
			},
			expectedErr: nil,
		},
		{
			name: "comic not found add empty",
			id:   2,
			prepareMock: func(f mockFields) {
				f.xkcd.EXPECT().Get(gomock.Any(), 2).
					Return(core.XKCDInfo{}, core.ErrNotFound).
					Times(3) //

				f.db.EXPECT().Add(gomock.Any(), core.Comics{
					ID: 2, URL: "http://empty_comics", Words: []string{},
				}).Return(nil)
			},
			expectedErr: core.ErrSkipped,
		},
		{
			name: "comic not found db error on add empty",
			id:   3,
			prepareMock: func(f mockFields) {
				f.xkcd.EXPECT().Get(gomock.Any(), 3).
					Return(core.XKCDInfo{}, core.ErrNotFound).Times(3)

				f.db.EXPECT().Add(gomock.Any(), gomock.Any()).
					Return(fmt.Errorf("db internal error"))
			},
			expectedErr: fmt.Errorf("db add comics error"),
		},
		{
			name: "fetch generic error",
			id:   4,
			prepareMock: func(f mockFields) {
				f.xkcd.EXPECT().Get(gomock.Any(), 4).
					Return(core.XKCDInfo{}, fmt.Errorf("network error")).Times(3)
			},
			expectedErr: core.ErrSkipped,
		},
		{
			name: "normalize error",
			id:   5,
			prepareMock: func(f mockFields) {
				f.xkcd.EXPECT().Get(gomock.Any(), 5).
					Return(core.XKCDInfo{ID: 5, Description: "bad words"}, nil)

				f.words.EXPECT().Norm(gomock.Any(), "bad words").
					Return(nil, fmt.Errorf("norm error"))
			},
			expectedErr: core.ErrSkipped,
		},
		{
			name: "db error on final add",
			id:   6,
			prepareMock: func(f mockFields) {
				f.xkcd.EXPECT().Get(gomock.Any(), 6).
					Return(core.XKCDInfo{ID: 6, Description: "test"}, nil)
				f.words.EXPECT().Norm(gomock.Any(), "test").
					Return([]string{"test"}, nil)

				f.db.EXPECT().Add(gomock.Any(), gomock.Any()).
					Return(fmt.Errorf("disk full"))
			},
			expectedErr: fmt.Errorf("db add comics error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mf := mockFields{
				db:    mock_core.NewMockDB(ctrl),
				xkcd:  mock_core.NewMockXKCD(ctrl),
				words: mock_core.NewMockWords(ctrl),
			}

			if tt.prepareMock != nil {
				tt.prepareMock(mf)
			}

			logger := slog.New(slog.DiscardHandler)
			s, err := core.NewService(logger, mf.db, mf.xkcd, mf.words, nil, 1)

			require.NoError(t, err, "Expected no error when build update service")

			err = s.ProcessOne(context.Background(), tt.id)

			if tt.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedErr.Error())
			}
		})
	}
}

func TestService_AddEmptyComics(t *testing.T) {
	type mockFields struct {
		db *mock_core.MockDB
	}

	tests := []struct {
		name        string
		id          int
		prepareMock func(f mockFields)
		expectedErr string
	}{
		{
			name: "success add empty",
			id:   42,
			prepareMock: func(f mockFields) {
				f.db.EXPECT().Add(gomock.Any(), core.Comics{
					ID:    42,
					URL:   "http://empty_comics",
					Words: []string{},
				}).Return(nil)
			},
		},
		{
			name: "db error",
			id:   42,
			prepareMock: func(f mockFields) {
				f.db.EXPECT().Add(gomock.Any(), gomock.Any()).
					Return(fmt.Errorf("db is down"))
			},
			expectedErr: "db add comics error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mf := mockFields{db: mock_core.NewMockDB(ctrl)}
			if tt.prepareMock != nil {
				tt.prepareMock(mf)
			}

			logger := slog.New(slog.DiscardHandler)
			s, err := core.NewService(logger, mf.db, nil, nil, nil, 1)

			require.NoError(t, err, "Expected no error when build update service")

			err = s.AddEmptyComics(context.Background(), tt.id)

			if tt.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedErr)
			}
		})
	}
}

func TestService_Stats(t *testing.T) {
	tests := []struct {
		name          string
		prepareMock   func(mf mockFields)
		expectedStats core.ServiceStats
		expectedErr   string
	}{
		{
			name: "success all",
			prepareMock: func(mf mockFields) {
				mf.db.EXPECT().Stats(gomock.Any()).Return(core.DBStats{WordsTotal: 100}, nil)
				mf.xkcd.EXPECT().LastID(gomock.Any()).Return(500, nil)
			},
			expectedStats: core.ServiceStats{DBStats: core.DBStats{WordsTotal: 100}, ComicsTotal: 500},
		},
		{
			name: "db error fails everything",
			prepareMock: func(mf mockFields) {
				mf.db.EXPECT().Stats(gomock.Any()).Return(core.DBStats{}, fmt.Errorf("db dead"))
			},
			expectedErr: "failed to get db stats",
		},
		{
			name: "xkcd error uses 0 for lastID",
			prepareMock: func(mf mockFields) {
				mf.db.EXPECT().Stats(gomock.Any()).Return(core.DBStats{WordsTotal: 100}, nil)
				mf.xkcd.EXPECT().LastID(gomock.Any()).Return(0, fmt.Errorf("network error"))
			},
			expectedStats: core.ServiceStats{DBStats: core.DBStats{WordsTotal: 100}, ComicsTotal: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mf := mockFields{db: mock_core.NewMockDB(ctrl), xkcd: mock_core.NewMockXKCD(ctrl)}
			if tt.prepareMock != nil {
				tt.prepareMock(mf)
			}

			logger := slog.New(slog.DiscardHandler)
			s, err := core.NewService(logger, mf.db, mf.xkcd, nil, nil, 1)

			require.NoError(t, err, "Expected no error when build update service")

			res, err := s.Stats(context.Background())

			if tt.expectedErr == "" {
				require.NoError(t, err)
				require.Equal(t, tt.expectedStats, res)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedErr)
			}
		})
	}
}

func TestService_StatusAndDrop(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mock_core.NewMockDB(ctrl)
	mockBroker := mock_core.NewMockBroker(ctrl)

	logger := slog.New(slog.DiscardHandler)
	s, err := core.NewService(logger, mockDB, nil, nil, mockBroker, 1)

	require.NoError(t, err, "Expected no error when build update service")

	t.Run("Status idle by default", func(t *testing.T) {
		require.Equal(t, core.StatusIdle, s.Status(context.Background()))
	})

	t.Run("Status running when updated", func(t *testing.T) {
		s.SetUpdating(true)
		require.Equal(t, core.StatusRunning, s.Status(context.Background()))

		err := s.Drop(context.Background())
		require.ErrorIs(t, err, core.ErrAlreadyRunning)

		s.SetUpdating(false)
	})

	t.Run("Drop success sends event", func(t *testing.T) {
		mockDB.EXPECT().Drop(gomock.Any()).Return(nil)
		mockBroker.EXPECT().SendEvent(eventDrop).Times(1)

		err := s.Drop(context.Background())
		require.NoError(t, err)

		require.Equal(t, core.StatusIdle, s.Status(context.Background()))
	})

	t.Run("Drop db error does not send event", func(t *testing.T) {
		mockDB.EXPECT().Drop(gomock.Any()).Return(fmt.Errorf("db drop failed"))

		err := s.Drop(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "db drop failed")

		require.Equal(t, core.StatusIdle, s.Status(context.Background()))
	})
}
