package middleware

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	mock_middleware "yadro.com/course/api/adapters/rest/middleware/mocks"
)

func TestAuth(t *testing.T) {
	test := []struct {
		name           string
		token          string
		mockErr        error
		expectedStatus int
	}{
		{"Valid token test", "Token", nil, http.StatusOK},
		{"Not valid token test", "WrongToken", fmt.Errorf("verify error"), http.StatusUnauthorized},
	}
	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set(AuthorizationKey, tt.token)
			w := httptest.NewRecorder()

			ctrl := gomock.NewController(t)
			m := mock_middleware.NewMockTokenVerifier(ctrl)

			m.
				EXPECT().
				Verify(tt.token).
				Return(tt.mockErr).
				Times(1)

			handlerToTest := Auth(nextHandler, m)
			handlerToTest.ServeHTTP(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestConcurrency(t *testing.T) {
	test := []struct {
		name            string
		limit           int
		expectedSuccess int
		expectedFailed  int
	}{
		{"Not taken limit", 2, 2, 0},
		{"Taken limit", 1, 1, 1},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			hold := make(chan struct{})

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				<-hold
				w.WriteHeader(http.StatusOK)
			})

			handlerToTest := Concurrency(nextHandler, tt.limit)

			req := httptest.NewRequest("GET", "/", nil)
			rr1 := httptest.NewRecorder()
			rr2 := httptest.NewRecorder()

			var wg sync.WaitGroup

			wg.Go(func() {
				handlerToTest.ServeHTTP(rr1, req)
			})

			wg.Go(func() {
				handlerToTest.ServeHTTP(rr2, req)
			})

			time.Sleep(100 * time.Millisecond)

			close(hold)

			wg.Wait()

			var successCount, failCount int

			for _, code := range []int{rr1.Code, rr2.Code} {
				switch code {
				case http.StatusOK:
					successCount++
				case http.StatusServiceUnavailable:
					failCount++
				}
			}

			require.Equal(t, tt.expectedSuccess, successCount)
			require.Equal(t, tt.expectedFailed, failCount)
		})
	}
}

func TestRate_Throttling(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handlerToTest := Rate(nextHandler, 1, 30*time.Second)

	req1 := httptest.NewRequest("GET", "/", nil)
	req2 := httptest.NewRequest("GET", "/", nil)

	rec1 := httptest.NewRecorder()
	handlerToTest.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusOK, rec1.Code)

	now := time.Now()

	rec2 := httptest.NewRecorder()
	handlerToTest.ServeHTTP(rec2, req2)

	duration := time.Since(now)

	require.Equal(t, http.StatusOK, rec2.Code)
	require.GreaterOrEqual(t, duration, time.Second)
}

func TestRate_QueueOverflow(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handlerToTest := Rate(nextHandler, 1, 10*time.Millisecond)

	requestCount := 5
	codesResult := make(chan int, requestCount)

	var wg sync.WaitGroup

	for range requestCount {
		wg.Go(func() {
			req := httptest.NewRequest("GET", "/", nil)
			rec := httptest.NewRecorder()

			handlerToTest.ServeHTTP(rec, req)
			codesResult <- rec.Code
		})
	}

	wg.Wait()

	close(codesResult)

	var failCount int
	for code := range codesResult {
		if code == http.StatusTooManyRequests {
			failCount++
		}
	}

	require.Greater(t, failCount, 0)
}

func TestWithMetrics(t *testing.T) {
	expectedStatus := http.StatusCreated

	expectedURL := "/test/metrics"

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(expectedStatus)
		_, err := w.Write([]byte("created"))
		require.NoError(t, err)
	})

	handlerToTest := WithMetrics(nextHandler)

	req := httptest.NewRequest("GET", expectedURL, nil)
	rec := httptest.NewRecorder()

	handlerToTest.ServeHTTP(rec, req)

	require.Equal(t, expectedStatus, rec.Code)

	var buf bytes.Buffer

	metrics.WritePrometheus(&buf, true)

	output := buf.String()

	expectedMetricCount := `http_request_duration_seconds_count{url="/test/metrics", status="201"}`

	require.True(t, strings.Contains(output, expectedMetricCount))
}
