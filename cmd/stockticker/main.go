// Command stockticker serves the configured symbol's closing-price report over HTTP.
package main

import (
	"log"
	"net/http"
	"os"

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

	client := alphavantage.New(cfg.APIKey, http.DefaultClient)
	service := ticker.New(client)
	handler := httpapi.NewHandler(service, cfg.Symbol, cfg.NDays)

	log.Printf("listening on %s for %s over the last %d days", addr, cfg.Symbol, cfg.NDays)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
