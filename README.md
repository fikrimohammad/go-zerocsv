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
