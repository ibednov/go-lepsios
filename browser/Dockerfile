# Shared Chromium sidecar. Products call POST /internal/v1/fetch-in-page.
# Do not bake this image into product API containers.
FROM golang:1.25-bookworm AS builder

ENV GOTOOLCHAIN=go1.25.0
WORKDIR /src

RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential git ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
ENV GOWORK=off
RUN go mod download

COPY . .
ENV CGO_ENABLED=0
RUN GOWORK=off GOOS=linux go build -o /out/lepsios-browser ./cmd/browser

ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright
RUN go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 install --with-deps chromium

FROM mcr.microsoft.com/playwright:v1.62.1-noble

COPY --from=builder /out/lepsios-browser /usr/local/bin/lepsios-browser
COPY --from=builder /root/.cache/ms-playwright-go /root/.cache/ms-playwright-go
COPY --from=builder /ms-playwright /ms-playwright

ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright
ENV BROWSER_LISTEN=:8090
ENV BROWSER_HEADLESS=true
ENV BROWSER_TIMEOUT=75s
ENV BROWSER_WARMUP=20s

WORKDIR /root
EXPOSE 8090

HEALTHCHECK --interval=30s --timeout=5s --start-period=45s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:8090/health || exit 1

CMD ["lepsios-browser"]
