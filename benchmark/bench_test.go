package benchmark_test

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	zerocsv "github.com/fikrimohammad/go-zerocsv"
)

var benchNames = []string{"alpha", "beta", "gamma", "delta", "epsilon"}

// --- apples-to-apples: pure strings --------------------------------------

var benchStrings = []string{"alpha", "beta", "gamma", "delta", "epsilon"}

func BenchmarkWriteStrings(b *testing.B) {
	w := zerocsv.NewWriter(io.Discard)
	cols := []zerocsv.Column{
		zerocsv.ColumnString(benchStrings[0]),
		zerocsv.ColumnString(benchStrings[1]),
		zerocsv.ColumnString(benchStrings[2]),
		zerocsv.ColumnString(benchStrings[3]),
		zerocsv.ColumnString(benchStrings[4]),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := w.Write(cols...); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWriteStringsStdlib(b *testing.B) {
	w := csv.NewWriter(io.Discard)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := w.Write(benchStrings); err != nil {
			b.Fatal(err)
		}
	}
}

// --- realistic mixed types: ours converts zero-alloc, stdlib must strconv --

func BenchmarkWriteMixed(b *testing.B) {
	w := zerocsv.NewWriter(io.Discard)
	cols := make([]zerocsv.Column, 5)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cols[0] = zerocsv.ColumnString(benchNames[i%len(benchNames)])
		cols[1] = zerocsv.ColumnInt(i)
		cols[2] = zerocsv.ColumnInt64(int64(i) * 1000)
		cols[3] = zerocsv.ColumnFloat64(float64(i) * 0.5)
		cols[4] = zerocsv.ColumnBool(i%2 == 0)
		if err := w.Write(cols...); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWriteMixedStdlib(b *testing.B) {
	w := csv.NewWriter(io.Discard)
	record := make([]string, 0, 5)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		record = record[:0]
		record = append(record,
			benchNames[i%len(benchNames)],
			strconv.Itoa(i),
			strconv.FormatInt(int64(i)*1000, 10),
			strconv.FormatFloat(float64(i)*0.5, 'f', -1, 64),
			strconv.FormatBool(i%2 == 0),
		)
		if err := w.Write(record); err != nil {
			b.Fatal(err)
		}
	}
}

// --- mixed including a time column ---------------------------------------

func BenchmarkWriteWithTime(b *testing.B) {
	w := zerocsv.NewWriter(io.Discard)
	ts := time.Date(2026, 8, 17, 12, 34, 56, 0, time.UTC)
	cols := make([]zerocsv.Column, 6)
	var timeScratch [32]byte
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cols[0] = zerocsv.ColumnInt(i)
		cols[1] = zerocsv.ColumnString(benchNames[i%len(benchNames)])
		cols[2] = zerocsv.ColumnInt64(int64(i) * 1000)
		cols[3] = zerocsv.ColumnFloat64(float64(i) * 0.5)
		cols[4] = zerocsv.ColumnBool(i%2 == 0)
		cols[5] = zerocsv.ColumnBytes(ts.AppendFormat(timeScratch[:0], time.RFC3339))
		if err := w.Write(cols...); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWriteWithTimeStdlib(b *testing.B) {
	w := csv.NewWriter(io.Discard)
	ts := time.Date(2026, 8, 17, 12, 34, 56, 0, time.UTC)
	record := make([]string, 0, 6)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		record = record[:0]
		record = append(record,
			strconv.Itoa(i),
			benchNames[i%len(benchNames)],
			strconv.FormatInt(int64(i)*1000, 10),
			strconv.FormatFloat(float64(i)*0.5, 'f', -1, 64),
			strconv.FormatBool(i%2 == 0),
			ts.Format(time.RFC3339),
		)
		if err := w.Write(record); err != nil {
			b.Fatal(err)
		}
	}
}

// --- reader ---------------------------------------------------------------

var benchCSV = strings.Repeat("alpha,beta,gamma,delta,epsilon\n"+
	"1,2,3,4,5\n"+
	"\"a,b\",\"c\"\"d\",e,f,g\n"+
	"10,20,30,40,50\n", 64)

func BenchmarkRead(b *testing.B) {
	r := zerocsv.NewReader(strings.NewReader(benchCSV))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec, err := r.Read()
		if err == io.EOF {
			r = zerocsv.NewReader(strings.NewReader(benchCSV))
			rec, err = r.Read()
		}
		if err != nil {
			b.Fatal(err)
		}
		_ = rec.Len()
	}
}

func BenchmarkReadStdlib(b *testing.B) {
	r := csv.NewReader(strings.NewReader(benchCSV))
	r.FieldsPerRecord = -1
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec, err := r.Read()
		if err == io.EOF {
			r = csv.NewReader(strings.NewReader(benchCSV))
			r.FieldsPerRecord = -1
			rec, err = r.Read()
		}
		if err != nil {
			b.Fatal(err)
		}
		for j := range rec {
			_ = rec[j]
		}
	}
}

func BenchmarkReadStdlibReuseRecord(b *testing.B) {
	r := csv.NewReader(strings.NewReader(benchCSV))
	r.FieldsPerRecord = -1
	r.ReuseRecord = true
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec, err := r.Read()
		if err == io.EOF {
			r = csv.NewReader(strings.NewReader(benchCSV))
			r.FieldsPerRecord = -1
			r.ReuseRecord = true
			rec, err = r.Read()
		}
		if err != nil {
			b.Fatal(err)
		}
		for j := range rec {
			_ = rec[j]
		}
	}
}

var benchCSVMultiByte = strings.ReplaceAll(benchCSV, ",", "||")

func BenchmarkReadMultiByteString(b *testing.B) {
	r := zerocsv.NewReader(strings.NewReader(benchCSVMultiByte), zerocsv.WithDelimiterString("||"))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec, err := r.Read()
		if err == io.EOF {
			r = zerocsv.NewReader(strings.NewReader(benchCSVMultiByte), zerocsv.WithDelimiterString("||"))
			rec, err = r.Read()
		}
		if err != nil {
			b.Fatal(err)
		}
		_ = rec.Len()
	}
}

func BenchmarkWriteMultiByteString(b *testing.B) {
	w := zerocsv.NewWriter(io.Discard, zerocsv.WithDelimiterString("||"))
	cols := make([]zerocsv.Column, 5)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cols[0] = zerocsv.ColumnString(benchNames[i%len(benchNames)])
		cols[1] = zerocsv.ColumnInt(i)
		cols[2] = zerocsv.ColumnInt64(int64(i) * 1000)
		cols[3] = zerocsv.ColumnFloat64(float64(i) * 0.5)
		cols[4] = zerocsv.ColumnBool(i%2 == 0)
		if err := w.Write(cols...); err != nil {
			b.Fatal(err)
		}
	}
}
