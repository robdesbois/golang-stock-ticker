// Package ticker computes closing-price reports for a stock symbol.
package ticker

import (
	"context"
	"errors"
	"fmt"

	"github.com/robdesbois/golang-stock-ticker/alphavantage"
)

// ErrInvalidNDays is returned when ndays is outside the supported 1..100 range.
var ErrInvalidNDays = errors.New("ticker: ndays must be >= 1 and <= 100")

// ErrNoData is returned when the fetcher has no closing prices for the symbol.
var ErrNoData = errors.New("ticker: no closing prices available")

// ClosingPricesFetcher fetches daily closing prices for a stock symbol.
type ClosingPricesFetcher interface {
	FetchClosingPrices(ctx context.Context, symbol string) ([]alphavantage.ClosingPrice, error)
}

// Report is a set of closing prices for a symbol and their mean.
type Report struct {
	Symbol    string
	Prices    []alphavantage.ClosingPrice
	MeanClose float64
}

// Service computes Reports from closing prices supplied by a ClosingPricesFetcher.
type Service struct {
	fetcher ClosingPricesFetcher
}

// New constructs a Service backed by fetcher.
func New(fetcher ClosingPricesFetcher) *Service {
	return &Service{fetcher: fetcher}
}

// GenerateReport returns the last ndays closing prices (fewer if unavailable) for symbol,
// along with their mean closing price.
func (s *Service) GenerateReport(ctx context.Context, symbol string, ndays int) (Report, error) {
	if ndays < 1 || ndays > alphavantage.MaxDays {
		return Report{}, fmt.Errorf("%w: got %d", ErrInvalidNDays, ndays)
	}

	// prices returned in reverse chronological order
	prices, err := s.fetcher.FetchClosingPrices(ctx, symbol)
	if err != nil {
		return Report{}, fmt.Errorf("ticker: fetching closing prices: %w", err)
	}
	if len(prices) == 0 {
		return Report{}, fmt.Errorf("%w: %s", ErrNoData, symbol)
	}

	if len(prices) > ndays {
		prices = prices[:ndays]
	}

	var total float64
	for _, p := range prices {
		total += p.Price
	}

	return Report{
		Symbol:    symbol,
		Prices:    prices,
		MeanClose: total / float64(len(prices)),
	}, nil
}
