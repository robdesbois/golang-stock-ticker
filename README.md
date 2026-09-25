# golang-stock-ticker

Web service that fetches the last `NDAYS` daily closing prices for `SYMBOL` from
[Alpha Vantage](https://www.alphavantage.co/documentation/#daily) and returns them
along with the average close.

See [RESILIENCE.md](RESILIENCE.md) for what's deliberately deferred/out of scope and what
production readiness would require.

## Configuration

Set via environment variables (see `.env.example`):

| Var      | Description                                  |
|----------|-----------------------------------------------|
| `SYMBOL` | Equity ticker symbol, e.g. `MSFT`             |
| `NDAYS`  | Number of most recent trading days (1-100)    |
| `APIKEY` | Alpha Vantage API key                         |

The server listens on port `8080`.

Never commit a real API key. `.env` is gitignored; copy `.env.example` to `.env`
and fill in your own key for local runs. `.env` is the single local source of truth reused
by every run method below (standalone binary, Docker, Kubernetes).

## Build & run

```sh
go build -o stockticker ./cmd/stockticker
go test ./...
```

Run it (loads `SYMBOL`, `NDAYS`, `APIKEY` from `.env` — see Configuration above):

```sh
set -a; source .env; set +a
./stockticker
```

Then, in another terminal:

```sh
curl http://localhost:8080/
```

## Docker

Pull the published image from [Docker Hub](https://hub.docker.com/r/robdesbois/stock-ticker):

```sh
docker pull robdesbois/stock-ticker:v0.1.1
```

Or build it yourself:

```sh
docker build -t robdesbois/stock-ticker:v0.1.1 .
```

Run it (env vars are supplied at run time, never baked into the image):

```sh
docker run --env-file .env -p 8080:8080 robdesbois/stock-ticker:v0.1.1
```

Then:

```sh
curl http://localhost:8080/
```

### Publishing a release

Build and push both the version tag and `latest` to Docker Hub (requires `docker login`
with push access to `robdesbois/stock-ticker`):

```sh
docker build -t robdesbois/stock-ticker:vX.Y.Z -t robdesbois/stock-ticker:latest .
docker push robdesbois/stock-ticker:vX.Y.Z
docker push robdesbois/stock-ticker:latest
```

Tag the matching commit in git so the image version and source stay traceable:

```sh
git tag -a vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

If a tag is later moved to a different commit, rebuild and re-push the same image tag
so Docker Hub matches; this overwrites the existing public tag, so only do it deliberately.

## Kubernetes (Part 2)

Manifests live in `deploy/k8s/`. Tested against minikube with the `ingress` addon.

```sh
minikube start
minikube addons enable ingress
```

Build the image directly into minikube's Docker daemon (no registry push needed for local testing;
this local build is separate from the tag published to Docker Hub):

```sh
eval $(minikube docker-env)
docker build -t robdesbois/stock-ticker:v0.1.1 .
```

Apply the manifests, then populate the Secret with your real API key from `.env` (never commit
a real key into `secret.yaml`):

```sh
kubectl apply -f deploy/k8s/
kubectl create secret generic stockticker-secret \
  --from-literal=APIKEY="$(grep '^APIKEY=' .env | cut -d= -f2-)" \
  --dry-run=client -o yaml | kubectl apply -f -
```

Point a hostname at the ingress and curl it. The ingress-nginx addon redirects HTTP to
HTTPS by default, so use `https://` with `-k` (self-signed cert) or follow the redirect:

```sh
echo "$(minikube ip) stockticker.local" | sudo tee -a /etc/hosts
curl -k https://stockticker.local/
```
