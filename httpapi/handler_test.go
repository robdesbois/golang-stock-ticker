package httpapi

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/robdesbois/golang-stock-ticker/alphavantage"
	"github.com/robdesbois/golang-stock-ticker/ticker"
)

// fakeGenerator is a ReportGenerator returning a fixed report or error.
type fakeGenerator struct {
	report ticker.Report
	err    error
}

func (f *fakeGenerator) GenerateReport(_ context.Context, _ string, _ int) (ticker.Report, error) {
	return f.report, f.err
}

func TestServeHTTP_Success(t *testing.T) {
	report := ticker.Report{
		Symbol: "MSFT",
		Prices: []alphavantage.ClosingPrice{
			{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Price: 124.56},
			{Date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Price: 123.45},
		},
		MeanClose: 497.13714285714286,
	}

	handler := NewHandler(&fakeGenerator{report: report}, "MSFT", 2)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/json")
	}
	if got := rec.Header().Get("Content-Length"); got != strconv.Itoa(rec.Body.Len()) {
		t.Fatalf("Content-Length = %q, want %q", got, strconv.Itoa(rec.Body.Len()))
	}
	if got := rec.Header().Get("Cache-Control"); got != successCacheControl {
		t.Fatalf("Cache-Control = %q, want %q", got, successCacheControl)
	}

	var got reportResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding response body: %v", err)
	}

	want := reportResponse{
		Symbol: "MSFT",
		Days: []dayClose{
			{Date: "2024-01-02", Close: 124.56},
			{Date: "2024-01-01", Close: 123.45},
		},
		AverageClose: jsontext.Value("497.14"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("body = %+v, want %+v", got, want)
	}
}

func TestServeHTTP_Errors(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"alphavantage upstream error", alphavantage.ErrUpstream, http.StatusBadGateway},
		{"ticker no data error", ticker.ErrNoData, http.StatusBadGateway},
		{"generic error", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewHandler(&fakeGenerator{err: tc.err}, "MSFT", 2)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("Content-Type = %q, want %q", got, "application/json")
			}
			var got errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decoding response body: %v", err)
			}
			if got.Error == "" {
				t.Fatalf("body error message is empty")
			}
			if got := rec.Header().Get("Cache-Control"); got != "" {
				t.Fatalf("Cache-Control = %q, want unset for error responses", got)
			}
		})
	}
}

func TestServeHTTP_MethodNotAllowed(t *testing.T) {
	handler := NewHandler(&fakeGenerator{}, "MSFT", 2)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if got := rec.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("Allow = %q, want %q", got, http.MethodGet)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/json")
	}
}
