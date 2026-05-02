package core_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"yadro.com/course/isearch/core"
	mock_core "yadro.com/course/isearch/core/mocks"
)

func TestSortByFrequency(t *testing.T) {
	tests := []struct {
		name        string
		idsSet      []int
		limit       int
		expectedRes []int
	}{
		{
			name:        "simple sort",
			idsSet:      []int{1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6},
			limit:       4,
			expectedRes: []int{6, 5, 4, 3},
		},
		{
			name:        "tie breaking",
			idsSet:      []int{1, 2, 1, 1, 2, 2, 3, 4, 5},
			limit:       6,
			expectedRes: []int{2, 1, 5, 4, 3},
		},
		{
			name:        "limit applied",
			idsSet:      []int{1, 2, 1, 1, 2, 2, 3, 4, 5},
			limit:       1,
			expectedRes: []int{2},
		},
		{
			name:        "zero limit",
			idsSet:      []int{1, 2, 3, 4, 5, 6},
			limit:       0,
			expectedRes: []int{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := core.SortByFrequency(tt.idsSet, tt.limit)
			require.Equal(t, tt.expectedRes, res)
		})
	}
}

func TestService_BuildIndex(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mock_core.NewMockDB(ctrl)
	mockWords := mock_core.NewMockWords(ctrl)

	mockDB.
		EXPECT().
		GetAllComics(gomock.Any()).
		Return([]core.DBComic{
			{ID: 1, URL: "url1", Words: []string{"go"}},
			{ID: 2, URL: "url2", Words: []string{"go", "rust"}},
		}, nil).
		Times(1)

	logger := slog.New(slog.DiscardHandler)
	isearchService, err := core.NewService(logger, mockDB, mockWords)

	require.NoError(t, err, "Expect no error when build isearh service")

	err = isearchService.BuildIndex(context.Background())

	require.NoError(t, err, "Expect no error when build index")

	index := isearchService.InternalIndex()
	require.ElementsMatch(t, []int{1, 2}, index["go"])
	require.Equal(t, []int{2}, index["rust"])

	isearchService.DropIndex()
	require.Empty(t, isearchService.InternalIndex())
}

func TestService_BuildIndex_DBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mock_core.NewMockDB(ctrl)
	mockWords := mock_core.NewMockWords(ctrl)

	mockDB.
		EXPECT().
		GetAllComics(gomock.Any()).
		Return([]core.DBComic{}, fmt.Errorf("db internal error")).
		Times(1)

	logger := slog.New(slog.DiscardHandler)
	isearchService, err := core.NewService(logger, mockDB, mockWords)

	require.NoError(t, err, "Expect no error when build isearh service")

	err = isearchService.BuildIndex(context.Background())

	require.Error(t, err, "Expect error when build index")
}

func TestService_Search(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockDB := mock_core.NewMockDB(ctrl)
	mockWords := mock_core.NewMockWords(ctrl)

	logger := slog.New(slog.DiscardHandler)
	isearchService, err := core.NewService(logger, mockDB, mockWords)

	require.NoError(t, err, "Expect no error when build isearh service")

	mockDB.
		EXPECT().
		GetAllComics(gomock.Any()).
		Return([]core.DBComic{
			{ID: 1, URL: "url1", Words: []string{"apple", "maple"}},
			{ID: 2, URL: "url2", Words: []string{"orange", "juice"}},
		}, nil).
		Times(1)

	err = isearchService.BuildIndex(context.Background())

	require.NoError(t, err, "Expect no error when build isearh service")

	mockWords.
		EXPECT().
		Norm(gomock.Any(), gomock.Eq("orange")).
		Return([]string{"orange"}, nil).
		Times(1)

	mockDB.
		EXPECT().
		GetComicsByIDs(gomock.Any(), gomock.Eq([]int{2})).
		Return([]core.DBLightComic{{ID: 2, URL: "url2"}}, nil).
		Times(1)

	res, err := isearchService.SearchIndex(context.Background(), &core.SearchRequest{Phrase: "orange", Limit: 2})

	require.NoError(t, err, "Expect no error when search")

	expectedRes := &core.SearchReply{
		Comics: []core.Comic{{ID: 2, URL: "url2"}},
	}

	require.Equal(t, expectedRes, res)
}

func TestService_SearchIndex_Errors(t *testing.T) {
	type mockFields struct {
		db    *mock_core.MockDB
		words *mock_core.MockWords
	}

	tests := []struct {
		name          string
		phrase        string
		prepareMock   func(mf mockFields)
		expectedError string
	}{
		{
			name:   "normalization service failed",
			phrase: "some phrase",
			prepareMock: func(mf mockFields) {
				mf.words.EXPECT().
					Norm(gomock.Any(), "some phrase").
					Return(nil, fmt.Errorf("words service unavailable"))
			},
			expectedError: "words service unavailable",
		},
		{
			name:   "database failed to get comic details",
			phrase: "apple",
			prepareMock: func(f mockFields) {
				f.words.EXPECT().
					Norm(gomock.Any(), "apple").
					Return([]string{"apple"}, nil)

				f.db.EXPECT().
					GetComicsByIDs(gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("db connection timeout"))
			},
			expectedError: "db connection timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mf := mockFields{
				db:    mock_core.NewMockDB(ctrl),
				words: mock_core.NewMockWords(ctrl),
			}

			logger := slog.New(slog.DiscardHandler)
			svc, _ := core.NewService(logger, mf.db, mf.words)

			mf.db.EXPECT().GetAllComics(gomock.Any()).Return([]core.DBComic{
				{ID: 1, Words: []string{"apple"}},
			}, nil).AnyTimes()

			_ = svc.BuildIndex(context.Background())

			tt.prepareMock(mf)

			res, err := svc.SearchIndex(context.Background(), &core.SearchRequest{Phrase: tt.phrase, Limit: 10})

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.expectedError)
			require.Nil(t, res)
		})
	}
}
