package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	ports := []string{"8081", "8082", "8083", "8084"}

	for _, port := range ports {
		go func(p string) {
			mux := http.NewServeMux()

			mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
			})

			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"message": "Response from upstream", "port": "%s", "path": "%s"}`+"\n", p, r.URL.Path)
			})

			log.Printf("Starting mock upstream server on :%s\n", p)
			if err := http.ListenAndServe(":"+p, mux); err != nil {
				log.Fatal(err)
			}
		}(port)
	}

	select {}
}
