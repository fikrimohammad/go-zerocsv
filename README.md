# go-zerocsv

[![Go Reference](https://pkg.go.dev/badge/github.com/fikrimohammad/go-zerocsv.svg)](https://pkg.go.dev/github.com/fikrimohammad/go-zerocsv)
[![CI](https://github.com/fikrimohammad/go-zerocsv/actions/workflows/ci.yml/badge.svg)](https://github.com/fikrimohammad/go-zerocsv/actions/workflows/ci.yml)
[![License](https://img.shields.io/github/license/fikrimohammad/go-zerocsv.svg)](LICENSE)

go-zerocsv is a zero-allocation CSV reader and writer for Go. It reads and
writes CSV records without heap-allocating per record, so throughput stays
high and memory use stays flat no matter how large the file is.

## Features

- **Zero allocations on the hot path**: each `Write` and each `Next` parses
  without heap allocation; the buffer and field slices are allocated once and
  reused.
- **Zero dependencies**: relies exclusively on the Go standard library.
- **Fuzz-tested parity**: reader output is conformance-fuzzed against
  `encoding/csv` in both strict and lazy-quote modes.
- **Flat memory**: the reader holds one reusable buffer regardless of input
  size, so a 5M-row file costs roughly as much memory as a 100-row one.

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
	w := zerocsv.New(&buf)

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

### Reading

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
	input := "name,age\nAlice,30\nBob,40\n"
	r := zerocsv.NewReader(strings.NewReader(input))

	for {
		rec, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		for i := 0; i < rec.Len(); i++ {
			fmt.Printf("%d:%s ", i, rec.ValueAt(i))
		}
		fmt.Println()
	}
}
```

## Advanced Usage

### Custom options

Options are shared by the writer and reader; each applies only to the side it
concerns.

```go
// Tab-separated values.
w := zerocsv.New(&buf, zerocsv.WithDelimiter('\t'))

// CRLF line endings instead of LF.
w := zerocsv.New(&buf, zerocsv.WithCRLF())

// Tolerate malformed quoting instead of returning a parse error.
r := zerocsv.NewReader(f, zerocsv.WithLazyQuotes())
```

### Typed columns

```go
w.Write(
	zerocsv.ColumnInt64(42),
	zerocsv.ColumnFloat64(3.14),
	zerocsv.ColumnTime(time.Now(), time.RFC3339),
	zerocsv.ColumnBytes([]byte("raw")),
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

### Zero-copy reading

`ValueAt` returns a zero-copy view into the reader's reused buffer. The value
is valid only until the next `Next` call. Convert it to a string to own the
data, since `string([]byte)` copies:

```go
for {
	rec, err := r.Next()
	if err == io.EOF {
		break
	}
	if err != nil {
		log.Fatal(err)
	}
	name := string(rec.ValueAt(0)) // owned copy, safe to retain
	_ = name
}
```

## Benchmarks

Measured on an AMD Ryzen 5 8400F (12 threads), linux/amd64, Go 1.26.5.
Run them yourself with:

```bash
go test -bench=. -benchmem ./benchmark
```

### Reading — full pass over a whole file

Each iteration parses all `n` rows with a fresh reader; `MB/s` is throughput.

| Rows | zerocsv ns/op | stdlib ns/op | zerocsv MB/s | stdlib MB/s | zerocsv allocs | stdlib allocs |
| ---- | ------------: | -----------: | -----------: | ----------: | --------------: | -------------: |
| 100K | 9.00ms | 10.90ms | 319.6 | 263.8 | 8 | 200,013 |
| 500K | 45.4ms | 51.9ms | 316.7 | 277.2 | 8 | 1,000,013 |
| 1M | 91.6ms | 102.9ms | 313.8 | 279.5 | 8 | 2,000,013 |
| 5M | 446.6ms | 516.9ms | 321.9 | 278.1 | 8 | 10,000,013 |

The reader allocates a constant 8 objects (internal buffer growth) regardless
of how many rows are read, while `encoding/csv` allocates 2 objects per record
(10M allocations for 5M rows, ~540 MB of garbage). Memory use stays flat
because the reader keeps one reusable buffer; stdlib's footprint grows with
file size.

### Writing — full pass over a whole file

Each iteration writes all `n` rows (6 fields each, including `int`, `float64`
and RFC3339 time) to `io.Discard` with a fresh writer.

| Rows | zerocsv ns/op | stdlib ns/op | zerocsv allocs | stdlib allocs |
| ---- | ------------: | -----------: | --------------: | -------------: |
| 100K | 14.1ms | 19.9ms | 5 | 399,890 |
| 500K | 71.5ms | 100.9ms | 5 | 1,999,890 |
| 1M | 143.2ms | 205.3ms | 5 | 3,999,890 |
| 5M | 735.0ms | 1,062ms | 5 | 19,999,891 |

zerocsv writes ~1.4x faster than `encoding/csv` and allocates a constant
5 objects (buffer and scratch growth) no matter how many rows are written,
whereas `encoding/csv` allocates ~4 objects per record for its `strconv`
conversions.

### Reading — per record

| Benchmark | ns/op | B/op | allocs/op |
| --------- | ----: | ---: | --------: |
| zerocsv `Next` | 64.0 | 18 | 0 |
| `encoding/csv` `Read` | 115.1 | 114 | 2 |

### Writing — per record

| Benchmark | ns/op | B/op | allocs/op |
| --------- | ----: | ---: | --------: |
| zerocsv `Write` (strings) | 67.2 | 0 | 0 |
| `encoding/csv` `Write` (strings) | 57.1 | 0 | 0 |
| zerocsv `Write` (mixed types) | 99.3 | 0 | 0 |
| `encoding/csv` `Write` (mixed types) | 141.7 | 31 | 2 |
| zerocsv `Write` (with time) | 148.0 | 0 | 0 |
| `encoding/csv` `Write` (with time) | 202.5 | 54 | 3 |

For pre-formatted strings both writers are allocation-free and comparable.
When values need formatting, zerocsv formats into its own scratch buffer
without allocating, so the mixed and time cases stay at 0 allocs/op while
`encoding/csv` allocates for every `strconv`/`Format` call.

## Documentation

Full API documentation and runnable examples are available on
[pkg.go.dev](https://pkg.go.dev/github.com/fikrimohammad/go-zerocsv).

## Concurrency

`Writer` and `Reader` are stateful and not safe for concurrent use. Use a
separate instance per goroutine.

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

# Smoke-test the fuzzers (anchor the pattern so it matches one target)
go test -run='^$' -fuzz='^FuzzReaderConformance$' -fuzztime=30s ./...
```

## License

Distributed under the MIT License. See [LICENSE](LICENSE) for more information.
