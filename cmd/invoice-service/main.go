package main

import (
	"log"
	"net/http"
	"os"

	invoice "github.com/example/field-invoice-service"
)

func main() {
	baseURL := os.Getenv("INFRAI_BASE_URL")
	if baseURL == "" {
		baseURL = invoice.DefaultBaseURL
	}
	client, err := invoice.NewClient(baseURL, os.Getenv("INFRAI_API_KEY"), nil)
	if err != nil {
		log.Fatal(err)
	}
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("invoice service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, invoice.Handler(invoice.NewService(client))))
}
