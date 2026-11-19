# logpulse

[![CI](https://github.com/Oleg4311/logpulse/actions/workflows/ci.yml/badge.svg)](https://github.com/Oleg4311/logpulse/actions/workflows/ci.yml)

Fast access-log analyzer written in Go. Feed it an nginx/Apache log or JSON lines and get request rate, status-code breakdown, latency percentiles, top and slowest endpoints, and detected anomalies (traffic spikes, bursts of 5xx).

**Live demo (runs in your browser via Go → WebAssembly):** https://oleg4311.github.io/logpulse/

```
logpulse report
  requests   1613   (skipped lines: 0)
  rate       avg 5.39 req/s, peak 60.00 req/s (per 1s bucket)
Status codes
  2xx ####################....   85.2%  1374
  5xx ###.....................   11.8%  190
Latency (ms)
  p50 57.0   p90 1520.0   p95 1558.0   p99 1764.0   max 2850.0
Anomalies
  13:03:00  spike  traffic spike: 60 requests (avg 5.4)
  13:03:00  errors 41 of 60 requests are 5xx
```

## Install

Download a binary for Windows, Linux or macOS from [Releases](https://github.com/Oleg4311/logpulse/releases), or build it:

```
go install github.com/Oleg4311/logpulse/cmd/logpulse@latest
```

## Usage

```
logpulse access.log                    # one or more files
cat access.log | logpulse              # stdin
tail -f access.log | logpulse -live    # live-updating dashboard
logpulse -json -top 20 access.log      # machine-readable report
```

| Flag | Meaning |
|---|---|
| `-format auto\|nginx\|json` | line format, `auto` decides per line |
| `-top N` | number of endpoints listed (default 10) |
| `-json` | print the report as JSON |
| `-live` | redraw the report twice a second while reading |
| `-no-color` | disable ANSI colors |

### Supported formats

- **nginx / Apache combined**, optionally followed by `request_time` (`... "ua" 0.123` or `rt=0.123`) for latency.
- **JSON lines** with common field names: `time|ts|timestamp|@timestamp`, `method|verb`, `path|uri|url|request_uri`, `status|code`, `bytes|size|body_bytes_sent`, and latency as `duration_ms|latency_ms` (milliseconds) or `duration|latency|request_time|elapsed` (seconds). Timestamps may be RFC 3339 or unix seconds/milliseconds.

## How it works

- Paths are normalized (`/users/42?x=1` → `/users/:id`) so endpoints are grouped.
- Percentiles use the nearest-rank method.
- Time is bucketed (1 s, 1 min or 1 h depending on the log span). A bucket is a **spike** when it has ≥ 5 requests and exceeds mean + 3σ, and an **error burst** when ≥ 50 % of ≥ 5 requests are 5xx.
- The analysis packages (`internal/parse`, `internal/stats`) are shared by the CLI (`cmd/logpulse`) and the WebAssembly build (`cmd/wasm`).

## Development

```
go test ./...
go run ./cmd/gensample > testdata/sample.log   # regenerate the demo log
GOOS=js GOARCH=wasm go build -o logpulse.wasm ./cmd/wasm
```

GitHub Actions: `ci.yml` (vet + tests on Linux/Windows/macOS), `pages.yml` (builds the WASM demo and deploys to Pages), `release.yml` (tag `v*` → cross-compiled binaries in Releases).

## License

MIT
