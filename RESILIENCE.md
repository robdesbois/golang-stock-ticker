# Resilience considerations (Part 3)

This service is intentionally minimal, scoped to a single GET endpoint, Dockerfile, and k8s
manifests. This document answers "what would it take to run this in production?" for everything
deliberately deferred or left out of scope. None of it is implemented — this is a design
discussion only.

## Health probes

Today, `deploy/k8s/deployment.yaml` uses `tcpSocket` liveness/readiness probes rather than HTTP
probes. That's a direct consequence of the service having no dedicated health endpoint: its only
route calls the real Alpha Vantage API (a 25 requests/day free-tier quota) on every request, so an
HTTP probe hitting that route would burn quota on every probe tick — likely exhausting it within
minutes at default probe intervals.

A production version would add:
- **`GET /healthz`** (liveness) — process-only check (always 200 if the process can respond),
  no upstream calls. Cheap to poll frequently.
- **`GET /readyz`** (readiness) — could optionally reflect current upstream reachability, but
  should reuse cached state (see Caching below) rather than issuing a fresh upstream call per
  probe.
- A Dockerfile `HEALTHCHECK` instruction hitting `/healthz`, for plain `docker run` usage
  outside Kubernetes.

## Graceful shutdown

`main.go` currently calls `server.ListenAndServe()` directly with no signal handling: `SIGTERM`
(sent by Kubernetes before a pod is killed) terminates the process immediately, dropping any
in-flight request.

Production would:
- Listen for `SIGTERM`/`SIGINT` (`signal.NotifyContext`), then call `server.Shutdown(ctx)` to stop
  accepting new connections and let in-flight requests finish within a bounded timeout.
- Set `terminationGracePeriodSeconds` on the Deployment (and ideally a `preStop` hook sleeping
  briefly) so Kubernetes' endpoint-removal and the app's own shutdown are coordinated — otherwise
  a request can still arrive at a pod that's mid-shutdown.

## Caching (deferred decorator)

A TTL-memoising decorator around `alphavantage.Client` (1 hour, fixed window) was designed but
deferred — see PLAN.md's "Deferred follow-ups". It directly addresses the 25 requests/day quota:
without it, restarts, redeploys, or multiple manual/automated calls exhaust the daily allowance
quickly.

Two further production-grade refinements, both explicitly out of scope for this exercise:
- **Timezone-aware cache expiry**: a fixed 1-hour TTL is simple but arbitrary. Alpha Vantage
  publishes one new daily close per trading day (US/Eastern). A production cache would instead
  invalidate at the next expected data refresh (e.g. shortly after US market close), maximizing
  cache-hit rate through the day while guaranteeing freshness once new data is published — rather
  than serving stale data for up to an hour after a refresh, or re-fetching identical data within
  the hour.
- **Shared/cross-replica caching**: an in-memory decorator is per-replica — each pod in a
  multi-replica Deployment has its own independent cache and hits the upstream API independently,
  multiplying quota consumption by replica count. A shared cache (Redis/Memcached, or similar)
  would give one cache shared across all replicas, which is what actually protects the quota at
  any scale beyond a single replica.

## Retries / backoff

`alphavantage.Client` currently treats any non-200 response or unexpected shape as a single
terminal error — no retries. Production would distinguish:
- **Retryable**: transient network errors, timeouts, `5xx` responses — worth a short bounded
  retry with exponential backoff and jitter.
- **Non-retryable**: shape mismatches indicating a bad API key or invalid request — retrying
  wastes quota on a call that will never succeed.

Given the free-tier quota, retries need to be very conservative here specifically (a naive retry
policy could burn through a day's quota in a handful of failed requests); caching (above) reduces
the need for retries to matter as much in practice.

## Structured logging

`main.go` uses plain `log.Printf`/`log.Fatalf` text logs. Production would use `log/slog` with a
JSON handler, including fields like request ID, status code, latency, and upstream call outcome
(success/rate-limited/shape-error), so a log aggregator (e.g. Loki, CloudWatch Logs, ELK) can
query and alert on them. Note this is a *logs* concern, not metrics — see below.

## Metrics

No metrics are currently exposed. Production would instrument: request count/latency (by status
code), upstream call count/latency/outcome, and (if the cache decorator above existed) cache
hit/miss ratio — exposed on a `/metrics` endpoint for Prometheus to scrape. Doing this well
typically means a metrics client library (e.g. `client_golang`), which would be the first
justified exception to this project's stdlib-only decision.
