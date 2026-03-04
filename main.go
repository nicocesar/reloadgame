package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
)

func main() {
	port := os.Getenv("PORT") // empty if not set :contentReference[oaicite:0]{index=0}
	if port == "" {
		port = "8080"
	}
	if _, err := strconv.Atoi(port); err != nil {
		log.Fatalf("invalid PORT %q: %v", port, err)
	}

	addr := ":" + port

	ln, err := net.Listen("tcp", addr) // fails fast if port is taken :contentReference[oaicite:1]{index=1}
	if err != nil {
		log.Fatalf("can't listen on %s: %v", addr, err)
	}
	defer ln.Close()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello world"))
	})

	log.Printf("listening on http://localhost%s", addr)
	log.Fatal(http.Serve(ln, handler)) // serve HTTP on an existing listener :contentReference[oaicite:2]{index=2}
}

