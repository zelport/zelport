# Catalog compression — 2026-10-13

Implemented endpoint-only catalog compression using Go `compress/flate` at
BestSpeed. Every record gets an independent context. A byte inside the AEAD
plaintext identifies raw or DEFLATE data; compression is used only if smaller.
The relay still sees opaque encrypted envelopes. Application payloads are unchanged.

All Snitzel peers need the new payload format. No legacy decoder is provided;
this change alone does not require a relay upgrade. The existing `known` catalog
hint feature still requires a relay supporting that field.

## Size measurements

Isolated fixtures use random service UUIDs, `project-N.localhost` domains, a random
public-key fixture, and no administration service. Each size includes the full
JSON/base64 encrypted catalog envelope, but excludes admission, WebSocket, TLS
and network headers. The baseline is an envelope encrypting the previous plain
canonical record, built by a test helper. These are not full relay traffic rates.

| Services | Previous envelope bytes | Compressed envelope bytes | Reduction |
| --- | ---: | ---: | ---: |
| 1 | 558 | 481 | 13.8% |
| 25 | 3,042 | 1,278 | 58.0% |
| 100 | 10,842 | 3,662 | 66.2% |
| 256 | 27,274 | 8,689 | 68.1% |

Values vary slightly with random keys and UUIDs; real domain lengths and contents
also affect the ratio. [Raw size/test output](catalog-compression-size-2026-10-13.txt).

## Endpoint costs

One local microbenchmark sample on Apple M4 Pro, darwin/arm64, Go 1.27.2,
200 ms per case. Seal/open include serialization, cryptography and record
validation, not just compression. They exclude network I/O and persistence.
There is no uncompressed CPU baseline in this measurement.

| Services | Seal µs/op | Open µs/op | Seal allocated bytes/op | Open allocated bytes/op |
| --- | ---: | ---: | ---: | ---: |
| 1 | 61 | 27 | 834,720 | 80,810 |
| 25 | 84 | 94 | 845,266 | 150,126 |
| 100 | 161 | 280 | 894,029 | 330,099 |
| 256 | 315 | 701 | 998,610 | 835,045 |

The fresh flate writer allocates workspace for each publication. These are
cumulative allocation bytes per operation, not permanently retained memory.
Publication remains once every 20 seconds per provider. No global compressor pool
or shared dictionary was added. [Raw benchmark output](catalog-compression-cpu-2026-10-13.txt).

## Validation and limits

Tests cover round trips and unchanged canonical bytes, incompressible raw
fallback, exact 128 KiB decoded limit, over-limit expansion, malformed/truncated
DEFLATE, trailing data, unknown encoding and rejection of legacy payloads.
Existing catalog tests cover authentication, identity binding and expiry.
Decompression happens only after AEAD authentication. Compression does not hide
lengths or constitute a security audit. No application tunnel compression was added.

Reproduce from the repository root:

```sh
go test ./internal/cloud/secure -run 'TestCatalog(CompressionSavings|PayloadValidation|PayloadBoundsAndFallback)' -v -count=1
go test ./internal/cloud/secure -run '^$' -bench BenchmarkCatalogCompression -benchtime=200ms -count=1
```
