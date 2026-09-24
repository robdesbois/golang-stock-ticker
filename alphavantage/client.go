// Package alphavantage provides a client for the Alpha Vantage stock market API.
package alphavantage

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"
)

// ErrUpstream is returned when Alpha Vantage's response doesn't have the shape a successful call requires.
var ErrUpstream = errors.New("alphavantage: unexpected response")

const defaultBaseURL = "https://www.alphavantage.co/query"

// MaxDays is the maximum number of daily closes returned by FetchClosingPrices.
const MaxDays = 100

// ClosingPrice is a single day's closing price.
type ClosingPrice struct {
	Date  time.Time
	Price float64
}

// Client fetches daily closing prices for a stock symbol.
type Client interface {
	FetchClosingPrices(ctx context.Context, symbol string) ([]ClosingPrice, error)
}

// HTTPClient is a Client backed by the real Alpha Vantage HTTP API.
type HTTPClient struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

var _ Client = (*HTTPClient)(nil)

// New constructs an HTTPClient using httpClient to perform requests.
func New(apiKey string, httpClient *http.Client) *HTTPClient {
	return &HTTPClient{
		apiKey:     apiKey,
		httpClient: httpClient,
		baseURL:    defaultBaseURL,
	}
}

type dailySeriesResponse struct {
	TimeSeriesDaily map[string]dailySeriesEntry `json:"Time Series (Daily)"`
}

type dailySeriesEntry struct {
	Price string `json:"4. close"`
}

// FetchClosingPrices fetches and returns daily closing prices in reverse chronological order.
// Returns up to MaxDays closing prices.
func (c *HTTPClient) FetchClosingPrices(ctx context.Context, symbol string) ([]ClosingPrice, error) {
	req, err := buildDailySeriesRequest(ctx, c.baseURL, c.apiKey, symbol)
	if err != nil {
		return nil, fmt.Errorf("alphavantage: building request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("alphavantage: performing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alphavantage: unexpected status %v", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("alphavantage: reading response: %w", err)
	}

	dailySeries, err := decodeDailySeries(body)
	if err != nil {
		return nil, err
	}

	return toClosingPrices(dailySeries)
}

// buildDailySeriesRequest constructs the TIME_SERIES_DAILY request for symbol.
// apiKey can be a free or premium API key: the result data is unaffected, but a rate limit applies to the free key.
func buildDailySeriesRequest(ctx context.Context, baseURL, apiKey, symbol string) (*http.Request, error) {
	requestURL := baseURL + "?" + url.Values{
		"function":   {"TIME_SERIES_DAILY"},
		"symbol":     {symbol},
		"apikey":     {apiKey},
		"outputsize": {"compact"},
	}.Encode()

	return http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
}

// decodeDailySeries decodes body into a dailySeriesResponse, returning ErrUpstream (with the raw
// body attached for diagnosis) if it doesn't have the expected shape.
//
// Alpha Vantage docs don't specify error responses, and unofficial evidence shows inconsistent field
// names; rather than trying to handle specific errors, just treat any response missing the expected data
// as an upstream error.
func decodeDailySeries(body []byte) (dailySeriesResponse, error) {
	var dailySeries dailySeriesResponse
	if err := json.Unmarshal(body, &dailySeries); err != nil {
		return dailySeriesResponse{}, fmt.Errorf("alphavantage: decoding response: %w", err)
	}

	if dailySeries.TimeSeriesDaily == nil {
		return dailySeriesResponse{}, fmt.Errorf("%w: response missing Time Series (Daily): %s", ErrUpstream, body)
	}

	return dailySeries, nil
}

// toClosingPrices converts the raw series entries into ClosingPrices sorted by date descending.
func toClosingPrices(dailySeries dailySeriesResponse) ([]ClosingPrice, error) {
	prices := make([]ClosingPrice, 0, len(dailySeries.TimeSeriesDaily))
	for dateStr, entry := range dailySeries.TimeSeriesDaily {
		date, err := time.Parse(time.DateOnly, dateStr)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid date %q: %v", ErrUpstream, dateStr, err)
		}
		closeVal, err := strconv.ParseFloat(entry.Price, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid close %q: %v", ErrUpstream, entry.Price, err)
		}
		prices = append(prices, ClosingPrice{Date: date, Price: closeVal})
	}

	slices.SortFunc(prices, func(a, b ClosingPrice) int {
		return b.Date.Compare(a.Date)
	})

	return prices, nil
}
