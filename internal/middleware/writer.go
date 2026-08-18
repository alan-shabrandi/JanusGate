package middleware

import "net/http"

type responseRecorder struct {
	http.ResponseWriter
	StatusCode   int
	BytesWritten int64
}

func getRecorder(w http.ResponseWriter) *responseRecorder {
	if rec, ok := w.(*responseRecorder); ok {
		return rec
	}
	return &responseRecorder{
		ResponseWriter: w,
	}
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	if r.StatusCode == 0 {
		r.StatusCode = statusCode
		r.ResponseWriter.WriteHeader(statusCode)
	}
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.StatusCode == 0 {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.BytesWritten += int64(n)
	return n, err
}

func (r *responseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
