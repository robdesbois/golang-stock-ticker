package alphavantage

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *HTTPClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := New("test-key", server.Client())
	client.baseURL = server.URL
	return client
}

func TestClosingPrices_ValidResponse(t *testing.T) {
	body := `{
		"Time Series (Daily)": {
			"2024-01-10": {"1. open": "1", "4. close": "110.5000"},
			"2024-01-12": {"1. open": "1", "4. close": "112.7500"},
			"2024-01-11": {"1. open": "1", "4. close": "111.2500"}
		}
	}`
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})

	got, err := client.FetchClosingPrices(context.Background(), "IBM")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []ClosingPrice{
		{Date: mustDate(t, "2024-01-12"), Price: 112.75},
		{Date: mustDate(t, "2024-01-11"), Price: 111.25},
		{Date: mustDate(t, "2024-01-10"), Price: 110.5},
	}

	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if !got[i].Date.Equal(want[i].Date) || got[i].Price != want[i].Price {
			t.Errorf("entry %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestClosingPrices_NonOKStatus(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := client.FetchClosingPrices(context.Background(), "IBM")
	if err == nil {
		t.Fatal("expected an error for non-200 status, got nil")
	}
}

func TestClosingPrices_InvalidDate(t *testing.T) {
	body := `{"Time Series (Daily)": {"not-a-date": {"4. close": "1.00"}}}`
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	})

	_, err := client.FetchClosingPrices(context.Background(), "IBM")
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("got error %v, want errors.Is(err, ErrUpstream)", err)
	}
}

func TestClosingPrices_InvalidClose(t *testing.T) {
	body := `{"Time Series (Daily)": {"2024-01-10": {"4. close": "not-a-number"}}}`
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	})

	_, err := client.FetchClosingPrices(context.Background(), "IBM")
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("got error %v, want errors.Is(err, ErrUpstream)", err)
	}
}

func TestBuildDailySeriesRequest(t *testing.T) {
	req, err := buildDailySeriesRequest(context.Background(), "https://example.com/query", "test-key", "IBM")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := req.URL.Query()
	want := map[string]string{
		"function":   "TIME_SERIES_DAILY",
		"symbol":     "IBM",
		"apikey":     "test-key",
		"outputsize": "compact",
	}

	for key, wantVal := range want {
		if gotVal := got.Get(key); gotVal != wantVal {
			t.Errorf("query param %q: got %q, want %q", key, gotVal, wantVal)
		}
	}
}

func TestDecodeDailySeries_Valid(t *testing.T) {
	parsed, err := decodeDailySeries([]byte(`{"Time Series (Daily)": {"2024-01-10": {"4. close": "1.00"}}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parsed.TimeSeriesDaily) != 1 {
		t.Fatalf("got %d entries, want 1", len(parsed.TimeSeriesDaily))
	}
}

func TestDecodeDailySeries_MissingShape(t *testing.T) {
	cases := map[string]string{
		"rate limit note":     `{"Note": "Thank you for using Alpha Vantage! Our standard API call frequency is 25 requests per day."}`,
		"information field":   `{"Information": "the parameter apikey is invalid or missing."}`,
		"error message field": `{"Error Message": "Invalid API call. Please retry or visit the documentation."}`,
		"empty object":        `{}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := decodeDailySeries([]byte(body))
			if !errors.Is(err, ErrUpstream) {
				t.Fatalf("got error %v, want errors.Is(err, ErrUpstream)", err)
			}
		})
	}
}

func TestDecodeDailySeries_MalformedJSON(t *testing.T) {
	_, err := decodeDailySeries([]byte("{not valid json"))
	if err == nil {
		t.Fatal("expected an error for malformed JSON, got nil")
	}
}

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		t.Fatalf("invalid test date %q: %v", s, err)
	}
	return d
}
