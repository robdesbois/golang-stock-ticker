// Package httpapi serves ticker reports over HTTP.
package httpapi

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/robdesbois/golang-stock-ticker/alphavantage"
	"github.com/robdesbois/golang-stock-ticker/ticker"
)

// ReportGenerator generates ticker reports; satisfied by *ticker.Service.
type ReportGenerator interface {
	GenerateReport(ctx context.Context, symbol string, ndays int) (ticker.Report, error)
}

// Handler serves the fixed symbol/ndays report as JSON.
type Handler struct {
	generator ReportGenerator
	symbol    string
	ndays     int
}

var _ http.Handler = (*Handler)(nil)

// NewHandler constructs a Handler that reports on symbol over the last ndays days.
func NewHandler(generator ReportGenerator, symbol string, ndays int) *Handler {
	return &Handler{generator: generator, symbol: symbol, ndays: ndays}
}

// reportResponse is the response body format for a ticker.Report.
type reportResponse struct {
	Symbol       string     `json:"symbol"`
	Days         []dayClose `json:"days"`
	AverageClose float64    `json:"averageClose"`
}

// dayClose is a single day's closing price in the response body.
type dayClose struct {
	Date  string  `json:"date"`
	Close float64 `json:"close"`
}

// errorResponse is the response body format for an error.
type errorResponse struct {
	Error string `json:"error"`
}

// successCacheControl lets clients/proxies reuse a response for up to an hour, reducing
// repeat hits against the rate-limited upstream API (see RESILIENCE.md).
const successCacheControl = "public, max-age=3600"

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// NB: no path-based routing: only have 1 capability to expose

	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	report, err := h.generator.GenerateReport(r.Context(), h.symbol, h.ndays)
	if err != nil {
		if errors.Is(err, alphavantage.ErrUpstream) || errors.Is(err, ticker.ErrNoData) {
			writeJSONError(w, http.StatusBadGateway, "upstream data provider error")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if !writeJSON(w, http.StatusOK, toReportResponse(report), successCacheControl) {
		writeJSONError(w, http.StatusInternalServerError, "internal server error")
	}
}

// writeJSON marshals body and writes it as a JSON response with the given status and an
// optional Cache-Control value (empty string omits the header), reporting false (without
// writing anything) if marshaling fails.
func writeJSON(w http.ResponseWriter, status int, body any, cacheControl string) bool {
	var buf bytes.Buffer
	if err := json.MarshalWrite(&buf, body); err != nil {
		return false
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	if cacheControl != "" {
		w.Header().Set("Cache-Control", cacheControl)
	}
	w.WriteHeader(status)
	w.Write(buf.Bytes())
	return true
}

// writeJSONError writes message as a JSON error response, falling back to a plain-text
// body (which cannot fail to marshal) if that somehow fails, so a response is always sent.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	if !writeJSON(w, status, errorResponse{Error: message}, "") {
		http.Error(w, message, status)
	}
}

// toReportResponse converts a domain Report into the response format.
// dates are formatted as time.DateOnly
func toReportResponse(report ticker.Report) reportResponse {
	days := make([]dayClose, len(report.Prices))
	for i, p := range report.Prices {
		days[i] = dayClose{
			Date:  p.Date.Format(time.DateOnly),
			Close: p.Price,
		}
	}

	return reportResponse{
		Symbol:       report.Symbol,
		Days:         days,
		AverageClose: report.MeanClose,
	}
}
