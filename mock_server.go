package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("[Mock Server :8081] Request Received: %s %s | Headers: Via=%s\n",
			r.Method, r.URL.Path, r.Header.Get("Via"))

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf("Hello from User Service! Path: %s\n", r.URL.Path)))
	})

	fmt.Println("🟢 Mock User Service running on http://localhost:8081")
	_ = http.ListenAndServe(":8081", nil)
}
