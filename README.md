# golang-stock-ticker

Web service that fetches the last `NDAYS` daily closing prices for `SYMBOL` from
[Alpha Vantage](https://www.alphavantage.co/documentation/#daily) and returns them
along with the average close.

## Configuration

Set via environment variables (see `.env.example`):

| Var      | Description                                  |
|----------|-----------------------------------------------|
| `SYMBOL` | Equity ticker symbol, e.g. `MSFT`             |
| `NDAYS`  | Number of most recent trading days (1-100)    |
| `APIKEY` | Alpha Vantage API key                         |

The server listens on port `8080`.

Never commit a real API key. `.env` is gitignored; copy `.env.example` to `.env`
and fill in your own key for local runs.

## Build & run

```sh
go build ./...
go test ./...
```
