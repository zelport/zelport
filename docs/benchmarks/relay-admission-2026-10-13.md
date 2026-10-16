# Admission write and buffer optimization

Measured on 2026-10-13, Apple M4 Pro, macOS arm64, Go 1.27.2, GOMAXPROCS 14.
The working tree has no committed revision. This extends the
[original baseline](relay-v3-2026-10-13.md) using the same harness and workload.
The relay runs separately from the consumer, provider and HTTP backend; all
processes still share one machine and loopback network. No user configuration or
running service was changed. These are transport measurements, not full Snitzel
gateway/module routing or WAN capacity measurements.

## Changes and staged measurement

1. CPU/allocation profiling of the unmodified relay.
2. One admission transport write for length prefix plus ciphertext, retrying
   short writes. No delayed batching and no wire-format changes.
3. Lazily grown, bounded buffers owned by each admission connection. Incoming
   ciphertext, decrypted plaintext and outgoing frames use separate storage;
   directional locks protect ownership and unread plaintext drains before reuse.
4. Repeat the preserved original test executable and repeat optimized large-body
   cases to assess machine-load variability; profile the final relay separately.

Normal benchmarks use three approximately three-second samples per case.
Profiles use a separate five-second WS/1 MiB/concurrency-eight run and are excluded
from performance tables. The original executable was saved before production
edits. The stage order was original profile, writes-only, buffers, original
repeat, large-body buffers repeat, final profile. Runs were sequential, not
randomized; percentages should not be interpreted as universal speedups.

## Throughput and latency

Each cell is a median across three samples. MB/s is useful response body bytes,
expressed in decimal megabytes per second. p95 columns are medians of per-run
percentiles in the order original / writes-only / buffers, not pooled percentiles.
Original below is the freshly repeated preserved binary, not the earlier report.

| Transport | Body | Concurrency | Original MB/s | Writes-only MB/s | Buffers MB/s | p95 ms: original / writes / buffers |
|---|---|---:|---:|---:|---:|---|
| WS | 1 KiB | 1 | 1.17 | 1.80 | 1.81 | 1.35 / 0.79 / 0.79 |
| WS | 1 KiB | 8 | 1.93 | 2.69 | 2.73 | 5.28 / 3.78 / 3.71 |
| WS | 1 MiB | 1 | 135.89 | 168.90 | 166.43 | 11.80 / 6.73 / 8.81 |
| WS | 1 MiB | 8 | 166.40 | 208.68 | 202.56 | 57.20 / 42.91 / 59.63 |
| WSS | 1 KiB | 1 | 1.21 | 1.69 | 1.55 | 1.06 / 0.85 / 0.95 |
| WSS | 1 KiB | 8 | 1.92 | 3.10 | 3.08 | 5.19 / 3.27 / 3.25 |
| WSS | 1 MiB | 1 | 141.59 | 164.05 | 160.95 | 7.96 / 6.87 / 9.73 |
| WSS | 1 MiB | 8 | 163.61 | 199.25 | 202.01 | 55.27 / 44.45 / 48.86 |

Tail latency varied appreciably. The first buffer run has higher median p95 in
some large-body cases, despite lower median request times and higher throughput.
The dedicated repeat below did not reproduce that increase. This does not prove
latency equivalence under all workloads; WAN and controlled offered-load tests
remain necessary. Both raw series are retained rather than selecting the fastest.

| Transport | Concurrency | Repeated buffers, 1 MiB: MB/s | Repeated p95 ms | Relay allocation KiB/request |
|---|---:|---:|---:|---:|
| WS | 1 | 181.64 | 6.23 | 258.1 |
| WS | 8 | 220.73 | 39.09 | 246.7 |
| WSS | 1 | 166.19 | 6.76 | 266.3 |
| WSS | 8 | 208.86 | 42.25 | 254.9 |

## Relay resource cost

These are concurrency-eight comparisons from the complete matrices, original →
final buffers. CPU s/GiB is calculated from measured relay CPU cores divided by
body throughput, normalized to one GiB. It is more useful than CPU utilization
alone when the optimized workload completes more requests. Allocation volume is
not resident memory. Writes are counted net.Conn calls, not packets. Egress/body
includes protocol framing, requests and controls but excludes TCP/IP overhead.

| Transport | Body | Allocated KiB/request | CPU s/GiB | Writes/request | Egress/body |
|---|---|---:|---:|---:|---:|
| WS | 1 KiB | 57.2 → 31.7 | 798.91 → 571.88 | 104.5 → 50.6 | 3.324 → 3.167 |
| WS | 1 MiB | 3981.6 → 242.7 | 8.87 → 5.70 | 848.5 → 420.8 | 1.014 → 1.013 |
| WSS | 1 KiB | 60.0 → 32.8 | 740.43 → 464.36 | 109.9 → 52.3 | 5.781 → 4.345 |
| WSS | 1 MiB | 4042.6 → 253.6 | 10.26 → 6.04 | 892.7 → 461.6 | 1.033 → 1.023 |

The substantial and repeatable changes are approximately half as many writes,
about 94% fewer allocated relay bytes for 1 MiB responses, and lower CPU per
useful GiB. Small WSS responses also see substantially lower egress per body byte.
Large-body network-byte savings are small; this optimization primarily reduces
transport operations and allocation churn for those workloads.

Buffer reuse trades transient allocations for retained capacity: at most 98,340
bytes of combined slice capacity across the three buffers per admission connection
after maximum-sized records in both directions. Idle control connections grow
only as needed. This bound excludes allocator rounding, WebSocket/TLS buffers,
bridge buffers, goroutines and other state;
it is neither measured RSS nor a relay capacity estimate. Buffers are not pooled
across connections and are released with their owning connection object.

## Profiles

Before optimization, sampled allocated bytes were attributed primarily to AEAD
output allocation (56.6% flat) and protocol.ReadFrame (33.4% flat); samples also
put substantial CPU time on the admission/WebSocket write path. After optimization,
context.AfterFunc accounts for approximately 75% of sampled allocation volume,
with WebSocket timeout setup contributing further allocations. These percentages
have different totals and do not imply that timeout work increased. No timeout,
cryptography, renewal, peer-credit or compression behavior was changed.

CPU profiles cover the measured interval. Allocation profiles are cumulative
worker profiles including startup, warm-up and profiler work; they are sampled
estimates, unlike the runtime allocation-counter deltas in the benchmark tables.
The local Go installation did not include the pprof command. Summaries were
computed with a small diagnostic reader of the official
[pprof profile format](https://github.com/google/pprof/blob/main/proto/profile.proto),
using embedded symbol tables. Raw standard profiles are retained for independent
inspection with pprof. Percentages are descriptive, not a bottleneck proof.

- [Original CPU](relay-admission-profiles/before-cpu.pprof), [original allocations](relay-admission-profiles/before-allocs.pprof), [original summary](relay-admission-profiles/before-summary.txt).
- [Final CPU](relay-admission-profiles/after-cpu.pprof), [final allocations](relay-admission-profiles/after-allocs.pprof), [final summary](relay-admission-profiles/after-summary.txt).

## Reproduction and raw results

From the repository root, without concurrent test/build workloads:

```sh
go test ./internal/cloud/client -run '^$' -bench '^BenchmarkRelayHTTP$' -benchtime=3s -count=3
SNITZEL_BENCH_PROFILE_DIR=/tmp/snitzel-relay-profiles go test ./internal/cloud/client -run '^$' -bench '^BenchmarkRelayHTTP/WS$/1048576B/concurrency8$' -benchtime=5s -count=1
```

Use an absolute profile directory. Each benchmark case writes cpu.pprof and
allocs.pprof in a separate subdirectory; calibration overwrites earlier profiles.
This profiling interface exists only in the isolated benchmark worker, not the
production relay. Performance counters from profile or race runs should not be
compared directly with uninstrumented results.

- [Repeated original](relay-admission-before-2026-10-13.txt).
- [Writes only](relay-admission-writes-2026-10-13.txt).
- [Writes and buffer reuse](relay-admission-buffers-2026-10-13.txt).
- [Repeated large-body cases](relay-admission-buffers-repeat-2026-10-13.txt).

## Verification and remaining work

The single-write test failed before the first change (four writes for two records)
and passes afterward (two writes). The warmed-record allocation test failed before
reuse (eight allocations per exchange, budget four) and passes afterward. Tests
also cover short and zero writes, partial reads with changing payloads, invalid
lengths and unchanged framing interoperability. Existing suites exercise rekey,
tampering, replay, stream backpressure, renewal and lifecycle behavior.

Independent focused static review found no actionable issues. Verification passed:

- `make check` (root core/full race tests, nested experiment race tests, vet).
- `go test -race -count=1 ./internal/cloud/...` (fresh uncached cloud tests).
- `make check-modules` (feature-exclusion and build matrix).
- `make build` (full Snitzel, standalone relay and experimental binaries).
- WS/WSS benchmark race smoke checks with `-benchtime=16x` and concurrency one/eight.
- Go formatting, relative documentation links and binary `--version` checks.

Race smoke results are correctness checks only. New binaries were built in
`bin/`; running user instances were not restarted.

Next measure full-gateway routing, WAN behavior and idle/catalog traffic. The
remaining WebSocket timeout allocations are a profiling lead, not authorization
to weaken cancellation or deadlines. Catalog protocol changes and endpoint
compression remain separate proposals.
