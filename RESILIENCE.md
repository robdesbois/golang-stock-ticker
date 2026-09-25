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

## Availability

`deploy/k8s/deployment.yaml` currently sets `replicas: 1` — a single pod. Any restart (crash,
node drain, rolling update) causes a brief total outage while a replacement pod starts and
passes its readiness probe, and there's no redundancy across nodes or availability zones.

Production would add:
- More replicas — but naively, see Scaling below: this multiplies upstream API calls per
  replica without a shared cache in place first.
- A `PodDisruptionBudget`, so voluntary disruptions (node drains, cluster upgrades) can't take
  down all replicas at once.
- Pod anti-affinity or topology spread constraints, so replicas land on different nodes/zones
  and a single node/zone failure doesn't take the whole service down.

None of this addresses availability of the Alpha Vantage dependency itself — if it's down or
rate-limiting, this service can't serve fresh data regardless of its own replica count. Caching
(below) reduces how often that matters (most requests served from cache rather than upstream),
but can't help if the upstream is down with an empty/expired cache.

## Caching (deferred decorator)

A TTL-memoising decorator around `alphavantage.Client` (1 hour, fixed window) was designed but
deferred. It would directly address the 25 requests/day quota: without it, restarts, redeploys,
or multiple manual/automated calls exhaust the daily allowance quickly.

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

## Scaling

Horizontal scaling (more replicas) is the natural Kubernetes-native answer to load, but is
actively counter-productive here until the shared-caching gap above is closed: each replica
independently calls the rate-limited upstream API, so replica count directly multiplies quota
consumption — 3 replicas triples the realistic risk of exhausting a 25/day quota. A
`HorizontalPodAutoscaler` driven by CPU/memory would also be the wrong signal: this service's
real bottleneck is an external, I/O-bound quota, not compute — per-request latency is dominated
by waiting on the remote API response, so CPU/memory usage stays flat regardless of concurrency.

Vertical scaling (larger requests/limits) doesn't help either — this is a lightweight,
mostly-idle-between-requests workload with no CPU/memory pressure to relieve.

The realistic production path: add the shared cache first (see Caching above), which both
reduces upstream calls per request *and* makes scaling out safe/meaningful (most requests served
from cache) — directly enabling the multi-replica availability improvements described above
without multiplying quota consumption.

## Image tag mutability

Current choice: images are published under immutable, versioned tags (`v0.1.0`, not a floating
tag like `:latest`), and the Deployment uses `imagePullPolicy: IfNotPresent`. This is intentional
and self-consistent: since a given tag's content is guaranteed to never change, letting nodes
reuse an already-pulled image for that tag is safe — there's nothing new to fetch.

The trade-off: if a security issue (e.g. a base-image CVE) is found in an already-deployed image,
the fix is a **new** tag (`v0.1.1`), not an update to the existing one — every manifest/environment
referencing the old tag must be updated to point at the new one. Many organisations deliberately
accept that coordination cost in exchange for auditability (you can always tell exactly what's
running from its tag) and safe rollback (the old tag still means what it always meant).

The alternative — a floating/mutable tag plus `imagePullPolicy: Always` — avoids the
update-every-manifest step, but loses that auditability/rollback guarantee, and *still* doesn't
automatically fix already-running pods: Kubernetes doesn't restart a pod just because the
registry content behind its image tag changed elsewhere. A rollout restart is required either way.

Production mitigation for the "propagate the version bump everywhere" cost, while keeping
immutable tags: automate it via GitOps tooling (a Kustomize image transformer, a Helm value, or
an image-updater such as Argo CD Image Updater/Flux), so a new release is a one-line change that
propagates through normal deployment pipelines rather than manual editing per environment.

## Namespace isolation

None of the manifests in `deploy/k8s/` specify a `metadata.namespace`, nor is there a `Namespace`
resource — so `kubectl apply -f deploy/k8s/` deploys into whatever namespace is already active in
the caller's context (`default`, unless overridden). That's fine for a single local minikube
cluster, but risky on any shared/real cluster: no isolation from other teams' workloads, no
namespace-scoped RBAC/ResourceQuotas/NetworkPolicies, and a real chance of naming collisions or
applying into the wrong namespace by mistake.

Production would add a dedicated `Namespace` manifest (e.g. `stockticker`), set that namespace on
every other resource, and scope RBAC/quotas/NetworkPolicies to it.

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
