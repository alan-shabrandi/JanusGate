package integration

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type MockUpstream struct {
	Server       *httptest.Server
	URL          string
	RequestCount int64
	statusCode   int
	delay        time.Duration
	headers      map[string]string
}

func NewMockUpstream(t *testing.T) *MockUpstream {
	t.Helper()

	mock := &MockUpstream{
		statusCode: http.StatusOK,
		headers:    make(map[string]string),
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&mock.RequestCount, 1)

		if mock.delay > 0 {
			time.Sleep(mock.delay)
		}

		for k, v := range mock.headers {
			w.Header().Set(k, v)
		}

		w.WriteHeader(mock.statusCode)

		_, _ = w.Write([]byte(r.URL.Path)) //nolint:gosec
	})

	server := httptest.NewServer(handler)
	mock.Server = server
	mock.URL = server.URL

	t.Cleanup(func() {
		server.Close()
	})

	return mock
}

func (m *MockUpstream) SetStatusCode(code int) {
	m.statusCode = code
}

func (m *MockUpstream) SetDelay(d time.Duration) {
	m.delay = d
}

func (m *MockUpstream) SetHeader(key, value string) {
	m.headers[key] = value
}

func (m *MockUpstream) GetRequestCount() int64 {
	return atomic.LoadInt64(&m.RequestCount)
}

func (m *MockUpstream) ResetRequestCount() {
	atomic.StoreInt64(&m.RequestCount, 0)
}
