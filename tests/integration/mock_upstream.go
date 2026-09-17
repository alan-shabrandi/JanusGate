package integration

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type MockUpstream struct {
	mu           sync.RWMutex
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

		mock.mu.RLock()
		delay := mock.delay
		statusCode := mock.statusCode
		localHeaders := make(map[string]string, len(mock.headers))
		for k, v := range mock.headers {
			localHeaders[k] = v
		}
		mock.mu.RUnlock()

		if delay > 0 {
			time.Sleep(delay)
		}

		for k, v := range localHeaders {
			w.Header().Set(k, v)
		}

		w.WriteHeader(statusCode)

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
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusCode = code
}

func (m *MockUpstream) SetDelay(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delay = d
}

func (m *MockUpstream) SetHeader(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.headers[key] = value
}

func (m *MockUpstream) GetRequestCount() int64 {
	return atomic.LoadInt64(&m.RequestCount)
}

func (m *MockUpstream) ResetRequestCount() {
	atomic.StoreInt64(&m.RequestCount, 0)
}
