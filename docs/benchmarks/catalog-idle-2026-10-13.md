# Idle catalog traffic and compression passthrough

Measured on 2026-10-13, Apple M4 Pro, macOS arm64, Go 1.27.2, GOMAXPROCS 14.
All connections are isolated loopback WS connections. No running user service,
configuration, or relay was modified. Separate-host/WAN benchmarks were explicitly
excluded from this task. The relay runs in a separate subprocess; its counters
exclude client work. Catalog clients use in-memory registries, not real module
inventories or durable configuration/watermark persistence.

## Method

Each of nine cases uses 2, 5 or 20 clients, with 1, 25 or 100 synthetic application
services per client (no additional administration service). Clients first converge,
then remain idle for 22 seconds with the actual production 20-second publication
and fetch timers. No application streams are opened during this window. Every
client remains connected without catalog warnings; each publishes exactly once
inside the observation window. Startup and initial convergence traffic are excluded.

Figures below are observed deltas divided by 22 seconds and extrapolated to a
minute. Because refreshes are periodic bursts rather than uniform traffic, they
understate a nominal three-cycle minute by roughly 9%; they are not a literal
one-minute capture. One window per case is a baseline, not a statistical performance
claim. Timer alignment and notification coalescing affect results. CPU numbers
for small cases are near the noise floor; lower CPU is not claimed for every case.
TCP payload bytes include admission and WebSocket framing, not TCP/IP headers.

## Implemented optimization

Each client includes at most 32 opaque route-ID / SHA-256 ciphertext-digest pairs
in catalog requests. Only envelopes that passed E2E authentication, identity/replay
checks and the configured acceptance/persistence callback are eligible. Expired
records are excluded. Relay compares these hints with its stored encrypted
records and omits exact matches; it cannot inspect or create semantic catalog
changes. Full snapshot byte limits still include omitted records.

Publication still sends a fresh encrypted record every 20 seconds, with the same
60-second expiry and durable sequence semantics. A missing/unchanged response is
never a freshness assertion; it does not update expiry, pins or watermarks. Changed
ciphertext is returned and authenticated normally. No new heartbeat or liveness
protocol, batching delay, compression, or cryptographic change was introduced.

This targets repeated full snapshots caused by the broadcast/fetch cycle. It does
not eliminate periodic provider publication or the remaining group-wide delivery
cost. Small authenticated freshness records remain a separate protocol design,
not an implemented feature.

The optional `known` request field extends the strict control-message schema.
Update relay and clients together: older relays reject the new field. A request
without hints still obtains the full snapshot. This is not a downgrade fallback.
The E2E record format and handshake protocol remain unchanged.

## Network results

Decimal kB/min, before → after. Savings compare ingress plus egress. Hints increase
request traffic, so reporting egress alone would overstate total savings.

| Clients | Services/client | Ingress kB/min | Egress kB/min | Total-byte reduction |
|---|---|---:|---:|---:|
| 2 | 1 | 5.1 → 8.2 | 17.9 → 6.6 | 35.9% |
| 2 | 25 | 18.7 → 21.7 | 72.1 → 20.1 | 53.9% |
| 2 | 100 | 61.2 → 64.3 | 242.3 → 62.7 | 58.2% |
| 5 | 1 | 12.8 → 31.4 | 104.0 → 47.4 | 32.5% |
| 5 | 25 | 46.6 → 65.2 | 442.7 → 182.9 | 49.3% |
| 5 | 100 | 157.8 → 171.6 | 3755.5 → 608.4 | 80.1% |
| 20 | 1 | 101.6 → 1226.1 | 7564.6 → 843.3 | 73.0% |
| 20 | 25 | 218.1 → 986.0 | 24020.3 → 3402.9 | 81.9% |
| 20 | 100 | 639.7 → 1618.4 | 75543.0 → 11494.2 | 82.8% |

## Relay CPU and allocations

CPU s/min means CPU seconds per minute of elapsed time, not percentage of the
whole machine. MB/min is allocated volume, not retained heap or peak RSS.

| Clients | Services/client | CPU s/min | Allocated MB/min |
|---|---|---:|---:|
| 2 | 1 | 0.0047 → 0.0089 | 0.247 → 0.250 |
| 2 | 25 | 0.0042 → 0.0066 | 0.762 → 0.681 |
| 2 | 100 | 0.0107 → 0.0091 | 2.189 → 2.055 |
| 5 | 1 | 0.0128 → 0.0104 | 0.717 → 0.767 |
| 5 | 25 | 0.0177 → 0.0189 | 2.041 → 1.844 |
| 5 | 100 | 0.0310 → 0.0301 | 9.686 → 5.843 |
| 20 | 1 | 0.2391 → 0.1700 | 19.338 → 15.677 |
| 20 | 25 | 0.2673 → 0.1631 | 37.448 → 18.705 |
| 20 | 100 | 0.4507 → 0.1998 | 113.325 → 45.631 |

## Lifecycle observations

A separate local test measures service renaming and publication withdrawal until
all clients have accepted the new catalog, then restores publication and measures
an established stream closing when its provider disconnects. It uses one service
per client, three repetitions, and 1 ms polling for catalog propagation. These are
medians, not latency guarantees. Active sharing revocation is additionally covered
by the existing full module HTTP/WebSocket integration tests.

| Clients | Rename visible to all, ms | Withdrawal visible to all, ms | Disconnect closes stream, ms |
|---|---:|---:|---:|
| 2 | 1.103 | 1.099 | 0.239 |
| 5 | 1.108 | 1.100 | 0.132 |
| 20 | 2.572 | 2.291 | 0.127 |

## Compression verification

The integration test `TestCloudPreservesGzipRepresentation` exercises both actual
Snitzel gateway handlers through relay and E2E transport, using a controlled source
module and temporary configuration. An explicit `Accept-Encoding: gzip` request
must return identical compressed bytes, `Content-Encoding`, `Vary`, and `ETag`.
It tests fixed Content-Length and flushed streaming responses; decompression at
the test client must recover the original body. Automatic client decompression is
disabled so it cannot hide proxy transformations. This verifies existing application
compression, not compression of the Noise tunnel, WebSocket compression extensions,
or an arbitrary production upstream's behavior.

## Reproduction and evidence

```sh
go test ./internal/cloud/client -run '^$' -bench '^BenchmarkCatalogIdle$' -benchtime=1x -count=1
go test ./internal/cloud/client -run '^TestCatalogLifecycleLatency$' -count=3 -v
go test -race ./internal/cloud/integration -run '^TestCloudPreservesGzipRepresentation$' -count=1
```

The idle benchmark takes approximately 3.5 minutes per complete matrix; use
`-benchtime=1x` to avoid automatic multi-minute calibration. The CPU and network
counters reuse the [relay worker harness](../../internal/cloud/client/relay_benchmark_test.go).

- [Before raw output](catalog-idle-before-2026-10-13.txt).
- [After raw output](catalog-idle-after-2026-10-13.txt).
- [Lifecycle observations](catalog-lifecycle-2026-10-13.txt).
- [Idle benchmark](../../internal/cloud/client/catalog_benchmark_test.go).
- [Gzip integration test](../../internal/cloud/integration/compression_test.go).

Focused tests cover malformed/oversized hints, changed ciphertext, authenticated
acceptance, persistence failure, forged records, expiry and malicious relay
omissions. Independent static review found no actionable security/correctness
issues. Verification passed:

- `go test -race -count=1 ./internal/cloud/...` (including the new gzip and cache tests).
- `make check` (core/full and experimental race tests plus vet).
- `make check-modules` (rerun with loopback/cache access after sandbox restrictions).
- `make build` and both binary `--version` checks.
- Go formatting and relative documentation links.

Binaries were built into `bin/`; running instances were not restarted.

## Remaining decisions

For a few development machines with small catalogs, absolute idle CPU remains
small. Large catalogs still justify evaluating authenticated compact freshness
records, but that requires a separate specification for body digests, expiry,
late joiners, reconnects, persistent watermarks and synchronous revocation. It
should not be replaced by trusting relay heartbeats. Endpoint tunnel compression
is deferred: preserving existing application compression already avoids sending
its uncompressed body. The explicitly excluded separate-host benchmark remains
pending.
