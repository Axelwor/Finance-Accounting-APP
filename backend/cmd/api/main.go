package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func healthHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(map[string]string{"status": "ok"})
}

func main() {
	http.HandleFunc("/healthz", healthHandler)

	log.Println("api listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
