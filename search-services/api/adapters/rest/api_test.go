package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	mock_rest "yadro.com/course/api/adapters/rest/mocks"
	"yadro.com/course/api/core"
	mock_core "yadro.com/course/api/core/mocks"
)

func TestPingHandler(t *testing.T) {
	tests := []struct {
		name           string
		pingerResults  map[string]error
		expectedStatus map[string]string
	}{
		{
			name: "All service is alive",
			pingerResults: map[string]error{
				"test_update": nil,
				"test_search": nil,
				"test_words":  nil,
			},
			expectedStatus: map[string]string{
				"test_update": "ok",
				"test_search": "ok",
				"test_words":  "ok",
			},
		},
		{
			name: "One service is fail",
			pingerResults: map[string]error{
				"test_update": nil,
				"test_search": fmt.Errorf("connection is refused"),
				"test_words":  nil,
			},
			expectedStatus: map[string]string{
				"test_update": "ok",
				"test_search": "unavailable",
				"test_words":  "ok",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			pingers := make(map[string]core.Pinger)

			for name, err := range tt.pingerResults {
				mockPinger := mock_core.NewMockPinger(ctrl)

				mockPinger.
					EXPECT().
					Ping(gomock.Any()).
					Return(err).
					Times(1)

				pingers[name] = mockPinger
			}

			logger := slog.New(slog.DiscardHandler)
			handlerToTest := NewPingHandler(logger, pingers)

			req := httptest.NewRequest("GET", "/api/ping", nil)
			rec := httptest.NewRecorder()

			handlerToTest.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)

			var response PingResponse

			err := json.Unmarshal(rec.Body.Bytes(), &response)

			require.NoError(t, err)
			require.Equal(t, tt.expectedStatus, response.Replies)
		})
	}
}

func TestLoginHandler(t *testing.T) {
	tests := []struct {
		name          string
		user          string
		password      string
		rawBody       []byte
		statusCode    int
		expectedToken string
		mockErr       error
		shouldCall    bool
	}{
		{
			name:          "correct login",
			user:          "admin",
			password:      "password",
			statusCode:    http.StatusOK,
			expectedToken: "asdasdasd",
			mockErr:       nil,
			shouldCall:    true,
		},
		{
			name:          "wrong login",
			user:          "vasya",
			password:      "incorrect_password",
			statusCode:    http.StatusUnauthorized,
			expectedToken: "",
			mockErr:       fmt.Errorf("db error"),
			shouldCall:    true,
		},
		{
			name:          "incorrect raw data (nil body)",
			rawBody:       nil,
			statusCode:    http.StatusInternalServerError,
			expectedToken: "",
			shouldCall:    false,
		},
		{
			name:          "incorrect raw data (invalid json)",
			rawBody:       []byte(`{"name": "admin", "password":`),
			statusCode:    http.StatusInternalServerError,
			expectedToken: "",
			shouldCall:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockAuth := mock_rest.NewMockAuthenticator(ctrl)

			if tt.shouldCall {
				mockAuth.EXPECT().
					Login(gomock.Eq(tt.user), gomock.Eq(tt.password)).
					Return(tt.expectedToken, tt.mockErr).
					Times(1)
			}

			var body io.Reader

			if tt.rawBody != nil {
				body = bytes.NewBuffer(tt.rawBody)
			} else if tt.user != "" || tt.password != "" {
				data, _ := json.Marshal(LoginRequest{Name: tt.user, Password: tt.password})
				body = bytes.NewBuffer(data)
			}

			req := httptest.NewRequest("POST", "/api/login", body)
			rec := httptest.NewRecorder()
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))

			handlerToTest := NewLoginHandler(logger, mockAuth)

			handlerToTest.ServeHTTP(rec, req)

			require.Equal(t, tt.statusCode, rec.Code)

			if tt.statusCode == http.StatusOK {
				require.Equal(t, tt.expectedToken, rec.Body.String())
			}
		})
	}
}

func TestWordsHandler(t *testing.T) {
	tests := []struct {
		name           string
		phrase         string
		mockWords      []string
		mockErr        error
		expectedCode   int
		expectedResult any
		isJSON         bool
	}{
		{
			name:         "success mapping",
			phrase:       "east or west home is best",
			mockWords:    []string{"east", "west", "home", "best"},
			mockErr:      nil,
			expectedCode: http.StatusOK,
			expectedResult: NormResponse{
				Words: []string{"east", "west", "home", "best"},
				Total: 4,
			},
			isJSON: true,
		},
		{
			name:           "empty phrase",
			phrase:         "",
			mockWords:      nil,
			mockErr:        core.ErrEmptyPhrase,
			expectedCode:   http.StatusBadRequest,
			expectedResult: core.ErrEmptyPhrase.Error(),
			isJSON:         false,
		},
		{
			name:           "very long phrase",
			phrase:         "long long phrase",
			mockWords:      nil,
			mockErr:        core.ErrResourceExhausted,
			expectedCode:   http.StatusBadRequest,
			expectedResult: core.ErrResourceExhausted.Error(),
			isJSON:         false,
		},
		{
			name:           "internal server error",
			phrase:         "problem phrase",
			mockWords:      nil,
			mockErr:        fmt.Errorf("another error"),
			expectedCode:   http.StatusInternalServerError,
			expectedResult: "normalize phrase error",
			isJSON:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockNormalizer := mock_core.NewMockNormalizer(ctrl)

			mockNormalizer.EXPECT().
				Norm(gomock.Any(), gomock.Eq(tt.phrase)).
				Return(tt.mockWords, tt.mockErr).
				Times(1)

			q := url.Values{}
			q.Add(phraseKey, tt.phrase)
			endpoint := fmt.Sprintf("/api/words?%s", q.Encode())

			req := httptest.NewRequest("GET", endpoint, nil)
			rec := httptest.NewRecorder()

			logger := slog.New(slog.DiscardHandler)
			handlerToTest := NewWordsHandler(logger, mockNormalizer)

			handlerToTest.ServeHTTP(rec, req)

			require.Equal(t, tt.expectedCode, rec.Code)

			if tt.isJSON {
				expectedData, err := json.Marshal(tt.expectedResult)
				require.NoError(t, err)
				require.JSONEq(t, string(expectedData), rec.Body.String())
			} else {
				require.Contains(t, rec.Body.String(), tt.expectedResult.(string))
			}
		})
	}
}

func TestUpdateHandler(t *testing.T) {
	tests := []struct {
		name         string
		mockErr      error
		expectedCode int
	}{
		{"All is right", nil, http.StatusOK},
		{"Update already is running", core.ErrAlreadyRunning, http.StatusAccepted},
		{"Internal error", fmt.Errorf("internal update error"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockUpdater := mock_core.NewMockUpdater(ctrl)

			mockUpdater.
				EXPECT().
				Update(gomock.Any()).
				Return(tt.mockErr)

			req := httptest.NewRequest("POST", "/api/db/update", nil)
			rec := httptest.NewRecorder()

			logger := slog.New(slog.DiscardHandler)
			handlerToTest := NewUpdateHandler(logger, mockUpdater)

			handlerToTest.ServeHTTP(rec, req)

			require.Equal(t, tt.expectedCode, rec.Code)
		})
	}
}

func TestUpdateStatsHandler(t *testing.T) {
	tests := []struct {
		name           string
		mockStats      core.UpdateStats
		mockErr        error
		expectedCode   int
		expectedObject UpdateStatsDto
	}{
		{
			name: "success case",
			mockStats: core.UpdateStats{
				WordsTotal:    10,
				WordsUnique:   5,
				ComicsTotal:   100,
				ComicsFetched: 50,
			},
			mockErr:      nil,
			expectedCode: http.StatusOK,
			expectedObject: UpdateStatsDto{
				WordsTotal:    10,
				WordsUnique:   5,
				ComicsTotal:   100,
				ComicsFetched: 50,
			},
		},
		{
			name:         "error case",
			mockStats:    core.UpdateStats{},
			mockErr:      fmt.Errorf("db internal error"),
			expectedCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockUpdater := mock_core.NewMockUpdater(ctrl)

			mockUpdater.EXPECT().
				Stats(gomock.Any()).
				Return(tt.mockStats, tt.mockErr).
				Times(1)

			req := httptest.NewRequest("GET", "/api/db/stats", nil)
			rec := httptest.NewRecorder()

			logger := slog.New(slog.DiscardHandler)
			handler := NewUpdateStatsHandler(logger, mockUpdater)

			handler.ServeHTTP(rec, req)

			require.Equal(t, tt.expectedCode, rec.Code)

			if tt.expectedCode == http.StatusOK {
				var res UpdateStatsDto
				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)
				require.Equal(t, tt.expectedObject, res)
			}
		})
	}
}

func TestDropHandler(t *testing.T) {
	tests := []struct {
		name         string
		expectedCode int
		mockErr      error
	}{
		{"success case", http.StatusOK, nil},
		{"already running case", http.StatusConflict, core.ErrAlreadyRunning},
		{"internal error case", http.StatusInternalServerError, fmt.Errorf("drop internal error")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockUpdater := mock_core.NewMockUpdater(ctrl)

			mockUpdater.
				EXPECT().
				Drop(gomock.Any()).
				Return(tt.mockErr).
				Times(1)

			req := httptest.NewRequest("GET", "/api/dp/drop", nil)
			rec := httptest.NewRecorder()

			logger := slog.New(slog.DiscardHandler)
			handlerToTest := NewDropHandler(logger, mockUpdater)

			handlerToTest.ServeHTTP(rec, req)

			require.Equal(t, tt.expectedCode, rec.Code)
		})
	}
}

func TestSearchHandler(t *testing.T) {
	tests := []struct {
		name         string
		queryLimit   string
		queryPhrase  string
		expectedBody string
		expectedCode int
		prepareMock  func(m *mock_core.MockSearcher)
		sr           SearchResponse
	}{
		{
			name:         "not integer limit",
			queryLimit:   "bad_limit",
			queryPhrase:  "not_empty",
			expectedBody: "wrong limit value, it must be a integer value bigger than zero",
			expectedCode: http.StatusBadRequest,
			prepareMock:  nil,
		},
		{
			name:         "empty phrase in request",
			queryLimit:   "10",
			queryPhrase:  "",
			expectedBody: "phrase cannot be empty",
			expectedCode: http.StatusBadRequest,
			prepareMock:  nil,
		},
		{
			name:         "successful search",
			queryLimit:   "10",
			queryPhrase:  "linux",
			expectedCode: http.StatusOK,
			prepareMock: func(m *mock_core.MockSearcher) {
				m.
					EXPECT().
					Search(gomock.Any(), "linux", 10).
					Return([]core.Comics{{ID: 1}}, nil).
					Times(1)
			},
			sr: SearchResponse{
				Comics: []core.Comics{{ID: 1}},
				Total:  1,
			},
		},
		{
			name:         "search internal error",
			queryLimit:   "10",
			queryPhrase:  "linux",
			expectedCode: http.StatusInternalServerError,
			prepareMock: func(m *mock_core.MockSearcher) {
				m.
					EXPECT().
					Search(gomock.Any(), "linux", 10).
					Return(nil, fmt.Errorf("search internal error")).
					Times(1)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockSearcher := mock_core.NewMockSearcher(ctrl)
			if tt.prepareMock != nil {
				tt.prepareMock(mockSearcher)
			}

			logger := slog.New(slog.DiscardHandler)
			handlerToTest := NewSearchHandler(logger, mockSearcher)

			q := url.Values{}
			q.Add(limitKey, tt.queryLimit)
			q.Add(phraseKey, tt.queryPhrase)

			endpoint := fmt.Sprintf("/api/search?%s", q.Encode())
			req := httptest.NewRequest("GET", endpoint, nil)

			rec := httptest.NewRecorder()

			handlerToTest.ServeHTTP(rec, req)

			require.Equal(t, tt.expectedCode, rec.Code)
			require.Contains(t, rec.Body.String(), tt.expectedBody)

			if tt.expectedCode == http.StatusOK {
				var res SearchResponse

				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)

				require.Equal(t, tt.sr, res)
			}
		})
	}
}

func TestSearchIndexHandler(t *testing.T) {
	tests := []struct {
		name         string
		queryLimit   string
		queryPhrase  string
		expectedBody string
		expectedCode int
		prepareMock  func(m *mock_core.MockISearcher)
		sr           SearchResponse
	}{
		{
			name:         "not integer limit",
			queryLimit:   "bad_limit",
			queryPhrase:  "not_empty",
			expectedBody: "wrong limit value, it must be a integer value bigger than zero",
			expectedCode: http.StatusBadRequest,
			prepareMock:  nil,
		},
		{
			name:         "empty phrase in request",
			queryLimit:   "10",
			queryPhrase:  "",
			expectedBody: "phrase cannot be empty",
			expectedCode: http.StatusBadRequest,
			prepareMock:  nil,
		},
		{
			name:         "successful search",
			queryLimit:   "10",
			queryPhrase:  "linux",
			expectedCode: http.StatusOK,
			prepareMock: func(m *mock_core.MockISearcher) {
				m.
					EXPECT().
					SearchIndex(gomock.Any(), "linux", 10).
					Return([]core.Comics{{ID: 1}}, nil).
					Times(1)
			},
			sr: SearchResponse{
				Comics: []core.Comics{{ID: 1}},
				Total:  1,
			},
		},
		{
			name:         "search internal error",
			queryLimit:   "10",
			queryPhrase:  "linux",
			expectedCode: http.StatusInternalServerError,
			prepareMock: func(m *mock_core.MockISearcher) {
				m.
					EXPECT().
					SearchIndex(gomock.Any(), "linux", 10).
					Return(nil, fmt.Errorf("search index internal error")).
					Times(1)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			mockISearcher := mock_core.NewMockISearcher(ctrl)
			if tt.prepareMock != nil {
				tt.prepareMock(mockISearcher)
			}

			logger := slog.New(slog.DiscardHandler)
			handlerToTest := NewSearchIndexHandler(logger, mockISearcher)

			q := url.Values{}
			q.Add(limitKey, tt.queryLimit)
			q.Add(phraseKey, tt.queryPhrase)

			endpoint := fmt.Sprintf("/api/isearch?%s", q.Encode())
			req := httptest.NewRequest("GET", endpoint, nil)

			rec := httptest.NewRecorder()

			handlerToTest.ServeHTTP(rec, req)

			require.Equal(t, tt.expectedCode, rec.Code)
			require.Contains(t, rec.Body.String(), tt.expectedBody)

			if tt.expectedCode == http.StatusOK {
				var res SearchResponse

				err := json.Unmarshal(rec.Body.Bytes(), &res)
				require.NoError(t, err)

				require.Equal(t, tt.sr, res)
			}
		})
	}
}

type errorWriter struct {
	http.ResponseWriter
}

func (e *errorWriter) Header() http.Header {
	return http.Header{}
}

func (e *errorWriter) Write(p []byte) (int, error) {
	return 0, fmt.Errorf("network connection lost")
}

func (e *errorWriter) WriteHeader(statusCode int) {}

func TestSendJSON(t *testing.T) {
	tests := []struct {
		name               string
		data               any
		writer             http.ResponseWriter
		expectedCode       int
		expectedJsonString string
		expectLog          string
	}{
		{
			name:               "success",
			data:               map[string]string{"status": "ok"},
			writer:             httptest.NewRecorder(),
			expectedCode:       http.StatusOK,
			expectedJsonString: `{"status":"ok"}`,
		},
		{
			name:         "marshal error",
			data:         make(chan int),
			writer:       httptest.NewRecorder(),
			expectedCode: http.StatusInternalServerError,
			expectLog:    "failed to encode response",
		},
		{
			name:         "write error",
			data:         map[string]string{"status": "ok"},
			writer:       &errorWriter{},
			expectedCode: 0,
			expectLog:    "write response error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewTextHandler(&buf, nil))

			sendJSON(tt.writer, log, tt.data)

			if rec, ok := tt.writer.(*httptest.ResponseRecorder); ok {
				require.Equal(t, tt.expectedCode, rec.Code)
				if tt.expectedCode == http.StatusOK {
					require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
					require.JSONEq(t, tt.expectedJsonString, rec.Body.String())
				}
			}
			if tt.expectLog != "" {
				require.Contains(t, buf.String(), tt.expectLog)
			}
		})
	}
}
