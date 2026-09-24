package ticker

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/robdesbois/golang-stock-ticker/alphavantage"
)

func floatsAlmostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

type fakeFetcher struct {
	prices []alphavantage.ClosingPrice
	err    error
}

func (f fakeFetcher) FetchClosingPrices(ctx context.Context, symbol string) ([]alphavantage.ClosingPrice, error) {
	return f.prices, f.err
}

func closingPrices(prices ...float64) []alphavantage.ClosingPrice {
	base := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	out := make([]alphavantage.ClosingPrice, len(prices))
	for i, p := range prices {
		out[i] = alphavantage.ClosingPrice{Date: base.AddDate(0, 0, -i), Price: p}
	}
	return out
}

func TestService_GenerateReport(t *testing.T) {
	errFetch := errors.New("boom")

	tests := map[string]struct {
		fetcher fakeFetcher
		ndays   int
		wantAvg float64
		wantLen int
		wantErr error
	}{
		"valid": {
			fetcher: fakeFetcher{prices: closingPrices(10, 20, 30)},
			ndays:   3,
			wantAvg: 20,
			wantLen: 3,
		},
		"ndays one": {
			fetcher: fakeFetcher{prices: closingPrices(10, 20, 30)},
			ndays:   1,
			wantAvg: 10,
			wantLen: 1,
		},
		"ndays max": {
			fetcher: fakeFetcher{prices: closingPrices(10, 20)},
			ndays:   100,
			wantAvg: 15,
			wantLen: 2,
		},
		"ndays zero": {
			fetcher: fakeFetcher{prices: closingPrices(10)},
			ndays:   0,
			wantErr: ErrInvalidNDays,
		},
		"ndays negative": {
			fetcher: fakeFetcher{prices: closingPrices(10)},
			ndays:   -1,
			wantErr: ErrInvalidNDays,
		},
		"ndays too large": {
			fetcher: fakeFetcher{prices: closingPrices(10)},
			ndays:   101,
			wantErr: ErrInvalidNDays,
		},
		"fetcher error": {
			fetcher: fakeFetcher{err: errFetch},
			ndays:   5,
			wantErr: errFetch,
		},
		"zero prices": {
			fetcher: fakeFetcher{prices: nil},
			ndays:   5,
			wantErr: ErrNoData,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			svc := New(tc.fetcher)
			report, err := svc.GenerateReport(context.Background(), "MSFT", tc.ndays)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("GenerateReport() error = %v, want errors.Is(%v)", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("GenerateReport() unexpected error: %v", err)
			}

			if report.Symbol != "MSFT" {
				t.Errorf("Symbol = %q, want MSFT", report.Symbol)
			}
			if len(report.Prices) != tc.wantLen {
				t.Errorf("len(Prices) = %d, want %d", len(report.Prices), tc.wantLen)
			}
			if !floatsAlmostEqual(report.MeanClose, tc.wantAvg) {
				t.Errorf("MeanClose = %v, want %v", report.MeanClose, tc.wantAvg)
			}
		})
	}
}
