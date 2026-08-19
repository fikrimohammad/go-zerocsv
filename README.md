# go-zerocsv

[![Go Reference](https://pkg.go.dev/badge/github.com/fikrimohammad/go-zerocsv.svg)](https://pkg.go.dev/github.com/fikrimohammad/go-zerocsv)
[![CI](https://github.com/fikrimohammad/go-zerocsv/actions/workflows/ci.yml/badge.svg)](https://github.com/fikrimohammad/go-zerocsv/actions/workflows/ci.yml)
[![License](https://img.shields.io/github/license/fikrimohammad/go-zerocsv.svg)](LICENSE)

**Zero-allocation, flat-memory CSV I/O for Go.** go-zerocsv reads CSV faster
than `encoding/csv` and writes with zero heap allocations per record. It reuses
a single compacting buffer, so a 5M-row file maintains a constant ~5 KB memory
footprint—compared to ~540 MB and 10 million heap allocations with standard
`encoding/csv` (and 140 MB with `ReuseRecord = true`). Same parsing semantics,
conformance-fuzzed against `encoding/csv`.

## Features

- **Zero-allocation hot paths**: `Write()` and lazy `Read()` parse without heap allocations via buffer reuse.
- **Flat-memory streaming**: Reusable compacting buffer maintains a ~5 KB footprint regardless of input size.
- **Typed scanning**: `Record.Scan(&id, &name, &score)` parses fields directly from byte slices without string allocations.
- **Custom types (`Scanner` & `Valuer`)**: Implement `FieldScanner` and `FieldValuer` for zero-allocation domain parsing/formatting.
- **Value-typed columns**: Compact 48-byte `Column` struct passes fields by value without interface boxing.
- **Fuzz-tested parity**: Conformance-fuzzed against `encoding/csv` (strict, lazy-quotes, and field counts). Zero dependencies.

## Installation

Requires Go 1.18 or higher (for `any` and native fuzzing).

```bash
go get github.com/fikrimohammad/go-zerocsv
```

## Quick Start

### Writing

```go
package main

import (
	"bytes"
	"fmt"

	zerocsv "github.com/fikrimohammad/go-zerocsv"
)

func main() {
	var buf bytes.Buffer
	w := zerocsv.NewWriter(&buf)

	w.Write(
		zerocsv.ColumnString("name"),
		zerocsv.ColumnInt(30),
		zerocsv.ColumnBool(true),
	)
	w.Flush()

	fmt.Print(buf.String())
	// name,30,true
}
```

### Reading (Typed & Zero-Allocation)

```go
package main

import (
	"fmt"
	"io"
	"log"
	"strings"

	zerocsv "github.com/fikrimohammad/go-zerocsv"
)

func main() {
	input := "id,name,score,active\n1,Alice,3.14,true\n2,Bob,2.50,false\n"
	r := zerocsv.NewReader(strings.NewReader(input))

	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		if rec.IsFirst() {
			continue // skip header
		}

		var (
			id     int
			name   string
			score  float64
			active bool
		)
		if err := rec.Scan(&id, &name, &score, &active); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("id=%d name=%s score=%.2f active=%t\n", id, name, score, active)
	}
}
```

## Advanced Usage

### Eager Reading with `ReadAll()`

To eagerly load all remaining CSV records into safe, owned `Record`s:

```go
records, err := r.ReadAll()
if err != nil {
	log.Fatal(err)
}
for _, rec := range records {
	fmt.Println(rec.Strings())
}
```

### Custom options

Options are shared by the writer and reader; each applies only to the side it
concerns.

```go
// Tab-separated values.
w := zerocsv.NewWriter(&buf, zerocsv.WithDelimiter('\t'))

// CRLF line endings instead of LF.
w := zerocsv.NewWriter(&buf, zerocsv.WithCRLF())

// Tolerate malformed quoting instead of returning a parse error.
r := zerocsv.NewReader(f, zerocsv.WithLazyQuotes())

// Enforce 3 fields per record, or disable the auto-detected check:
r := zerocsv.NewReader(f, zerocsv.WithFieldsPerRecord(3))
r := zerocsv.NewReader(f, zerocsv.WithFieldsPerRecord(-1)) // variable widths

// Cap the reader's internal buffer so a single oversized record fails with
// ErrRecordTooLarge instead of growing without bound.
r := zerocsv.NewReader(f, zerocsv.WithMaxBuffer(1 << 20))

// The auto-detected count is observable on both reader and writer:
w := zerocsv.NewWriter(&buf)
fmt.Println(w.FieldsPerRecord()) // 0 until the first record is written
```

### Typed columns (Writer)

```go
w.Write(
	zerocsv.ColumnInt64(42),
	zerocsv.ColumnFloat64(3.14),
	zerocsv.ColumnBool(true),
	zerocsv.ColumnString(time.Now().Format(time.RFC3339)),
	zerocsv.ColumnBytes([]byte("raw-bytes")),
)
```

### Zero-allocation writing loop

Reuse one `[]Column` slice across writes so nothing is allocated per record.

```go
row := make([]zerocsv.Column, 3)
for _, v := range values {
	row[0] = zerocsv.ColumnString(v.Name)
	row[1] = zerocsv.ColumnInt(v.Age)
	row[2] = zerocsv.ColumnBool(v.Active)
	_ = w.Write(row...)
}
```

### Custom Types (Scanner & Valuer)

Custom domain types can control their CSV parsing and formatting with **zero allocations** by implementing `FieldScanner` and `FieldValuer`:

```go
type Date time.Time

// Reader: zero-allocation parsing from raw field bytes
func (d *Date) ScanCSV(field []byte) error {
	t, err := time.Parse("2006-01-02", string(field))
	if err != nil {
		return err
	}
	*d = Date(t)
	return nil
}

// Writer: zero-allocation formatting into writer scratch buffer
func (d Date) AppendCSV(dst []byte) ([]byte, error) {
	return time.Time(d).AppendFormat(dst, "2006-01-02"), nil
}
```

Usage:
```go
// Scanning into custom type
var d Date
err := rec.Scan(&id, &name, &d)

// Writing custom type
err := w.Write(zerocsv.ColumnString("alice"), zerocsv.ColumnValuer(d))
```

## Benchmarks

Measured on an AMD Ryzen 5 8400F (12 threads), linux/amd64, Go 1.26.5.
Run them yourself with:

```bash
go test -bench=. -benchmem ./benchmark
```

`B/op` is the total bytes allocated per operation — the real memory cost.
Allocation counts alone are misleading: zerocsv's few allocations are one
reused buffer, while `encoding/csv`'s many allocations are small objects that
accumulate into hundreds of megabytes.

### Reading — full pass over a whole file

Each iteration parses all `n` rows with a fresh reader; `MB/s` is throughput
and `B/op` is the cumulative allocation for the whole pass.

| Rows | zerocsv ns/op | zerocsv MB/s | zerocsv B/op | zerocsv allocs | stdlib (default) ns/op | stdlib B/op | stdlib allocs | stdlib (ReuseRecord) ns/op | stdlib (ReuseRecord) B/op | stdlib (ReuseRecord) allocs |
| ---- | ------------: | -----------: | -----------: | --------------: | ---------------------: | ----------: | -------------: | --------------------------: | -------------------------: | ----------------------------: |
| 100K | 6.45ms | 445.5 | 5.0 KB | 12 | 11.33ms | 10.8 MB | 200,013 | 8.75ms | 2.8 MB | 100,014 |
| 500K | 32.6ms | 440.4 | 5.0 KB | 12 | 54.54ms | 54.0 MB | 1,000,013 | 43.29ms | 14.0 MB | 500,014 |
| 1M | 66.6ms | 431.5 | 5.0 KB | 12 | 109.2ms | 108 MB | 2,000,013 | 82.20ms | 28.0 MB | 1,000,014 |
| 5M | 334.7ms | 429.5 | 5.0 KB | 12 | 530.4ms | 540 MB | 10,000,013 | 415.9ms | 140 MB | 5,000,014 |

zerocsv allocates a constant ~5.0 KB and 12 objects no matter how many rows
are read: it reuses one small buffer that is compacted between records, so its
`B/op` stays flat while `encoding/csv`'s grows linearly to 540 MB at 5M rows
(even with `ReuseRecord = true`, which reuses the outer slice header but still
allocates 140 MB across 5 million field strings).
A record larger than the buffer grows it on demand to fit that single record,
and the buffer is trimmed back to ~4 KB once the record has been consumed, so
memory never stays pinned at the peak record size. Buffers up to 256 KB are
kept as-is to avoid grow/trim churn for records in that size band. For hostile
or untrusted input, `WithMaxBuffer` caps the buffer so a single oversized
record fails with `ErrRecordTooLarge` instead of growing without bound.

### Writing — full pass over a whole file

Each iteration writes all `n` rows (6 fields each, including `int`, `float64`
and RFC3339 time) to `io.Discard` with a fresh writer. `B/op` is the
cumulative allocation for the whole pass.

| Rows | zerocsv ns/op | zerocsv B/op | zerocsv allocs | stdlib ns/op | stdlib B/op | stdlib allocs |
| ---- | ------------: | -----------: | --------------: | -----------: | ----------: | -------------: |
| 100K | 13.6ms | 4.3 KB | 6 | 20.1ms | 4.7 MB | 399,890 |
| 500K | 67.3ms | 4.3 KB | 6 | 101.0ms | 24.0 MB | 1,999,892 |
| 1M | 137.8ms | 4.3 KB | 6 | 204.4ms | 47.9 MB | 3,999,895 |
| 5M | 708.3ms | 4.3 KB | 6 | 1,119ms | 272 MB | 19,999,917 |

zerocsv writes ~1.5x faster than `encoding/csv` and allocates a constant
4.3 KB (one 4 KB buffer plus a small numeric scratch) regardless of row count,
whereas `encoding/csv` allocates ~4 objects per record for its `strconv`
conversions — 272 MB of cumulative allocation for 5M rows.

### Reading — per record

| Benchmark | ns/op | B/op | allocs/op |
| --------- | ----: | ---: | --------: |
| zerocsv `Read` | 50.9 | 19 | 0 |
| `encoding/csv` `Read` | 115.2 | 114 | 2 |
| `encoding/csv` `Read` (`ReuseRecord = true`) | 84.6 | 35 | 1 |

### Writing — per record

| Benchmark | ns/op | B/op | allocs/op |
| --------- | ----: | ---: | --------: |
| zerocsv `Write` (strings) | 63.8 | 0 | 0 |
| `encoding/csv` `Write` (strings) | 58.2 | 0 | 0 |
| zerocsv `Write` (mixed types) | 95.7 | 0 | 0 |
| `encoding/csv` `Write` (mixed types) | 142.5 | 31 | 2 |
| zerocsv `Write` (with time) | 142.4 | 0 | 0 |
| `encoding/csv` `Write` (with time) | 200.5 | 54 | 3 |

For pre-formatted strings both writers are allocation-free and comparable.
When values need formatting, zerocsv formats into its own scratch buffer
without allocating, so the mixed and time cases stay at 0 B/op and 0 allocs/op
while `encoding/csv` allocates for every `strconv`/`Format` call.

### Real-World Impact: GC Pressure & Memory Limits (`GOMEMLIMIT`)

In containerized environments (Kubernetes, AWS ECS, Lambda), memory allocations trigger garbage collection (GC) and CPU throttling. When streaming a 3M-row dataset under a **150 MiB memory ceiling (`GOMEMLIMIT=150MiB`)**:

| Reader | Ingestion Time | Heap Allocations | GC Cycles Triggered | Behavior |
| :--- | ---: | ---: | ---: | :--- |
| **`go-zerocsv`** | **267ms** | **~5 KB** | **0** | Stable, flat memory |
| `encoding/csv (ReuseRecord=true)` | 2,392ms | 138 MB | 14,594 | **GC Thrashing (9x slowdown)** |
| `encoding/csv` (default) | 1,941ms | 366 MB | 10,517 | **GC Thrashing (7x slowdown)** |

Because standard `encoding/csv` allocates millions of transient heap strings, the Go runtime repeatedly halts execution to collect dead strings to stay under the memory ceiling. `go-zerocsv` remains allocation-free and runs at full speed regardless of memory limits.

## Documentation

Full API documentation and runnable examples are available on
[pkg.go.dev](https://pkg.go.dev/github.com/fikrimohammad/go-zerocsv).

## Concurrency

`Writer` and `Reader` are stateful and not safe for concurrent use. Use a
separate instance per goroutine.

## Limitations & Memory Model

go-zerocsv intentionally makes trade-offs to achieve zero allocations:

- **Buffer aliasing on lazy `Read()`**: Field byte slices in a lazy `Record` point
  directly into the reader's internal buffer. Data must be consumed (via `Scan()`,
  `Bytes()`, or `String()`) before the next call to `Read()`. Use `ReadAll()` if you
  need records that safely own their storage.
- **Single-byte delimiters**: The delimiter is a single byte (e.g. `,`, `\t`, `|`);
  multi-rune delimiters are not supported.
- **No comments or whitespace trimming**: Comment lines (`#`) are not recognized, and
  leading whitespace is not automatically stripped.
- **Non-resumable parse errors**: Once a fatal parse error occurs, `Read` and `ReadAll`
  continue returning it. (Field-count mismatches are non-fatal, matching `encoding/csv`).

## Contributing

1. Fork the repository.
2. Create your feature branch (`git checkout -b feature/amazing-feature`).
3. Commit your changes (`git commit -m 'Add some amazing feature'`).
4. Push to the branch (`git push origin feature/amazing-feature`).
5. Open a Pull Request.

### Local Quality Checks

Make sure to run the linters and tests locally before submitting your code:

```bash
# Run golangci-lint locally
golangci-lint run ./...

# Run tests with the race detector enabled
go test -race -count=1 ./...

# Smoke-test the fuzzers (anchor the pattern; run in the package root)
go test -run='^$' -fuzz='^FuzzReaderConformance$' -fuzztime=30s .
```

## License

Distributed under the MIT License. See [LICENSE](LICENSE) for more information.
