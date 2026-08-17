package benchmark_test

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"testing"
	"time"

	zerocsv "github.com/fikrimohammad/go-zerocsv"
)

// BenchmarkScale compares zerocsv against encoding/csv at fixed row counts,
// one write of n rows per iteration. Run with:
//
//	go test -bench BenchmarkScale -benchmem ./benchmark
//
// ns/op gives per-write cost (n rows), allocs/op and B/op give the allocation
// profile for writing n rows. Throughput is n / (ns/op).
func BenchmarkScale(b *testing.B) {
	sizes := []int{100_000, 500_000, 1_000_000, 5_000_000}
	for _, n := range sizes {
		b.Run(fmt.Sprintf("zerocsv/%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				writeZerocsv(io.Discard, n)
			}
		})
		b.Run(fmt.Sprintf("stdlib/%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				writeStdlib(io.Discard, n)
			}
		})
	}
}

func writeZerocsv(w io.Writer, n int) {
	zw := zerocsv.NewWriter(w)
	cols := make([]zerocsv.Column, 6)
	ts := time.Date(2026, 8, 17, 12, 34, 56, 0, time.UTC)
	for i := 0; i < n; i++ {
		cols[0] = zerocsv.ColumnInt(i)
		cols[1] = zerocsv.ColumnString(benchNames[i%len(benchNames)])
		cols[2] = zerocsv.ColumnInt64(int64(i) * 1000)
		cols[3] = zerocsv.ColumnFloat64(float64(i) * 0.5)
		cols[4] = zerocsv.ColumnBool(i%2 == 0)
		cols[5] = zerocsv.ColumnTime(ts, time.RFC3339)
		_ = zw.Write(cols...)
	}
	_ = zw.Flush()
}

func writeStdlib(w io.Writer, n int) {
	sw := csv.NewWriter(w)
	record := make([]string, 0, 6)
	ts := time.Date(2026, 8, 17, 12, 34, 56, 0, time.UTC)
	for i := 0; i < n; i++ {
		record = record[:0]
		record = append(record,
			strconv.Itoa(i),
			benchNames[i%len(benchNames)],
			strconv.FormatInt(int64(i)*1000, 10),
			strconv.FormatFloat(float64(i)*0.5, 'f', -1, 64),
			strconv.FormatBool(i%2 == 0),
			ts.Format(time.RFC3339),
		)
		_ = sw.Write(record)
	}
	sw.Flush()
}

// --- reader scale: full pass over n rows per iteration --------------------
//
// go test -bench BenchmarkReadScale -benchmem ./benchmark
//
// Each iteration reads an entire n-row CSV from a fresh reader. ns/op is the
// cost of one full pass, allocs/op the allocation profile for that pass, and
// MB/s (from SetBytes) the throughput.

// buildCSV returns n rows of CSV, a quarter of them quoted with a comma so the
// scan/decode paths are exercised.
func buildCSV(n int) []byte {
	buf := make([]byte, 0, n*48)
	for i := 0; i < n; i++ {
		if i%4 == 0 {
			buf = append(buf, "\"name,quoted\",2,3,4,5\n"...)
		} else {
			buf = append(buf, "alpha,beta,gamma,delta,epsilon\n"...)
		}
	}
	return buf
}

func BenchmarkReadScale(b *testing.B) {
	sizes := []int{100_000, 500_000, 1_000_000, 5_000_000}
	for _, n := range sizes {
		b.Run(fmt.Sprintf("zerocsv/%d", n), func(b *testing.B) {
			data := buildCSV(n)
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				readZerocsv(data)
			}
		})
		b.Run(fmt.Sprintf("stdlib/%d", n), func(b *testing.B) {
			data := buildCSV(n)
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				readStdlib(data)
			}
		})
	}
}

func readZerocsv(data []byte) {
	r := zerocsv.NewReader(bytes.NewReader(data))
	fields := 0
	for {
		rec, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		for i := 0; i < rec.Len(); i++ {
			fields += len(rec.ValueAt(i))
		}
	}
	if fields < 0 {
		panic(fields)
	}
}

func readStdlib(data []byte) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	fields := 0
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		for i := range rec {
			fields += len(rec[i])
		}
	}
	if fields < 0 {
		panic(fields)
	}
}
