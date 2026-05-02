package core_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"yadro.com/course/search/core"
	mock_core "yadro.com/course/search/core/mocks"
)

func TestService_Search(t *testing.T) {
	type mockFields struct {
		db    *mock_core.MockDB
		words *mock_core.MockWords
	}

	tests := []struct {
		name           string
		prepareMock    func(f mockFields)
		req            *core.SearchRequest
		expectedResult *core.SearchReply
		wantErr        bool
	}{
		{
			name: "normalize req error",
			prepareMock: func(mf mockFields) {
				mf.words.
					EXPECT().
					Norm(gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("words internal error")).
					Times(1)
			},
			req:     &core.SearchRequest{Phrase: "bad", Limit: 10},
			wantErr: true,
		},
		{
			name: "db get error",
			prepareMock: func(mf mockFields) {
				mf.db.
					EXPECT().
					GetAllComics(gomock.Any()).
					Return([]core.DBComic{}, fmt.Errorf("db internal error")).
					Times(1)
				mf.words.
					EXPECT().
					Norm(gomock.Any(), gomock.Eq("good phrase")).
					Return([]string{"good", "phrase"}, nil).
					Times(1)
			},
			req:     &core.SearchRequest{Phrase: "good phrase", Limit: 10},
			wantErr: true,
		},
		{
			name: "successes search with sort and limit",
			prepareMock: func(mf mockFields) {
				mf.words.
					EXPECT().
					Norm(gomock.Any(), gomock.Eq("linus and arch")).
					Return([]string{"linus", "arch"}, nil).
					Times(1)

				mf.db.
					EXPECT().
					GetAllComics(gomock.Any()).
					Return([]core.DBComic{
						{ID: 1, URL: "url1", Words: []string{"arch", "linus", "red hat"}},
						{ID: 2, URL: "url2", Words: []string{"arch", "red hat"}},
					}, nil).
					Times(1)
			},
			req: &core.SearchRequest{Phrase: "linus and arch", Limit: 2},
			expectedResult: &core.SearchReply{
				Comics: []core.Comic{
					{ID: 1, URL: "url1"},
					{ID: 2, URL: "url2"},
				},
			},
			wantErr: false,
		},
		{
			name: "successes search with sort and limit less than found comics",
			prepareMock: func(mf mockFields) {
				mf.words.
					EXPECT().
					Norm(gomock.Any(), gomock.Eq("linus and arch")).
					Return([]string{"linus", "arch"}, nil).
					Times(1)

				mf.db.
					EXPECT().
					GetAllComics(gomock.Any()).
					Return([]core.DBComic{
						{ID: 1, URL: "url1", Words: []string{"arch", "linus", "red hat"}},
						{ID: 2, URL: "url2", Words: []string{"arch", "red hat"}},
					}, nil).
					Times(1)
			},
			req: &core.SearchRequest{Phrase: "linus and arch", Limit: 1},
			expectedResult: &core.SearchReply{
				Comics: []core.Comic{
					{ID: 1, URL: "url1"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mf := mockFields{
				db:    mock_core.NewMockDB(ctrl),
				words: mock_core.NewMockWords(ctrl),
			}

			if tt.prepareMock != nil {
				tt.prepareMock(mf)
			}

			logger := slog.New(slog.DiscardHandler)
			searchService, err := core.NewService(logger, mf.db, mf.words)

			require.NoError(t, err, "Error when create search service")

			res, err := searchService.Search(context.Background(), tt.req)

			if tt.wantErr {
				require.Error(t, err, "Want error but doesn't get it")
				return
			}

			require.NoError(t, err, "Error when search")
			require.Equal(t, tt.expectedResult, res)
		})
	}
}

func TestGetScore(t *testing.T) {
	tests := []struct {
		name       string
		querySet   map[string]struct{}
		comicWords []string
		expected   float64
	}{
		{
			name:       "exact match",
			querySet:   map[string]struct{}{"cat": {}, "fat": {}},
			comicWords: []string{"cat", "fat"},
			expected:   1.0, // (2 / (2+2-2)) = 1.0
		},
		{
			name:       "partial match",
			querySet:   map[string]struct{}{"cat": {}, "fat": {}},
			comicWords: []string{"cat", "bat"},
			expected:   0.333, // (1 / (2+2-1)) = 0.333...
		},
		{
			name:       "no match",
			querySet:   map[string]struct{}{"cat": {}},
			comicWords: []string{"dog", "bird"},
			expected:   0.0,
		},
		{
			name:       "empty comic words",
			querySet:   map[string]struct{}{"cat": {}},
			comicWords: []string{},
			expected:   0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := core.GetScore(tt.querySet, tt.comicWords)
			require.InDelta(t, tt.expected, result, 0.001)
		})
	}
}
