// Command stockticker serves the configured symbol's closing-price report over HTTP.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/robdesbois/golang-stock-ticker/alphavantage"
	"github.com/robdesbois/golang-stock-ticker/config"
	"github.com/robdesbois/golang-stock-ticker/httpapi"
	"github.com/robdesbois/golang-stock-ticker/ticker"
)

const addr = ":8080"

func main() {
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	upstreamClient := &http.Client{Timeout: 10 * time.Second}
	client := alphavantage.New(cfg.APIKey, upstreamClient)
	service := ticker.New(client)
	handler := httpapi.NewHandler(service, cfg.Symbol, cfg.NDays)

	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("listening on %s for %s over the last %d days", addr, cfg.Symbol, cfg.NDays)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
