FROM golang:1.27-alpine AS builder
WORKDIR /src

COPY . .
RUN CGO_ENABLED=0 go build -o /out/stockticker ./cmd/stockticker

FROM gcr.io/distroless/static-debian13:nonroot
COPY --chown=nonroot:nonroot --from=builder /out/stockticker /stockticker

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/stockticker"]
