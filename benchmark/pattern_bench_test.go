package benchmark_test

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"testing"

	zerocsv "github.com/fikrimohammad/go-zerocsv"
)

// ============================================================================
// Reader Pattern Generators & Benchmarks (512KB - 640KB Payloads)
// ============================================================================

// growingRowReader generates CSV data where row sizes grow incrementally.
// Example: Row 1: 512KB, Row 2: 513KB, Row 3: 514KB, up to maxKB (repeating).
type growingRowReader struct {
	totalRows  int
	currentRow int
	templates  [][]byte
	rowOffset  int
}

func newGrowingRowReader(totalRows, startKB, stepKB, maxKB int) *growingRowReader {
	numTemplates := (maxKB-startKB)/stepKB + 1
	templates := make([][]byte, numTemplates)
	for i := 0; i < numTemplates; i++ {
		kb := startKB + (i * stepKB)
		targetBytes := kb * 1024
		buf := make([]byte, 0, targetBytes)
		prefix := fmt.Sprintf("field1_%d,field2_%d,", i, kb)
		buf = append(buf, prefix...)
		for len(buf) < targetBytes-1 {
			buf = append(buf, 'x')
		}
		buf = append(buf, '\n')
		templates[i] = buf
	}
	return &growingRowReader{
		totalRows: totalRows,
		templates: templates,
	}
}

func (g *growingRowReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	written := 0
	for written < len(p) {
		if g.currentRow >= g.totalRows {
			if written > 0 {
				return written, nil
			}
			return 0, io.EOF
		}
		idx := g.currentRow % len(g.templates)
		rowBytes := g.templates[idx]

		if g.rowOffset >= len(rowBytes) {
			g.currentRow++
			g.rowOffset = 0
			continue
		}

		n := copy(p[written:], rowBytes[g.rowOffset:])
		g.rowOffset += n
		written += n
	}
	return written, nil
}

// constantLargeRowReader generates CSV data where every row is a fixed size > 512KB.
type constantLargeRowReader struct {
	totalRows  int
	currentRow int
	rowBytes   []byte
	rowOffset  int
}

func newConstantLargeRowReader(totalRows, sizeKB int) *constantLargeRowReader {
	targetBytes := sizeKB * 1024
	row := make([]byte, 0, targetBytes)
	row = append(row, "col1_val,col2_val,"...)
	for len(row) < targetBytes-1 {
		row = append(row, 'x')
	}
	row = append(row, '\n')
	return &constantLargeRowReader{
		totalRows: totalRows,
		rowBytes:  row,
	}
}

func (c *constantLargeRowReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	written := 0
	for written < len(p) {
		if c.currentRow >= c.totalRows {
			if written > 0 {
				return written, nil
			}
			return 0, io.EOF
		}
		if c.rowOffset >= len(c.rowBytes) {
			c.currentRow++
			c.rowOffset = 0
			continue
		}
		n := copy(p[written:], c.rowBytes[c.rowOffset:])
		c.rowOffset += n
		written += n
	}
	return written, nil
}

// spikeRowReader generates CSV data where every period rows has a large spike > 512KB.
// Example: period=3: Row 1: 3KB, Row 2: 3KB, Row 3: 640KB...
type spikeRowReader struct {
	totalRows   int
	currentRow  int
	normalBytes []byte
	spikeBytes  []byte
	period      int
	rowOffset   int
}

func newSpikeRowReader(totalRows, normalKB, spikeKB, period int) *spikeRowReader {
	norm := make([]byte, 0, normalKB*1024)
	norm = append(norm, "norm1,norm2,"...)
	for len(norm) < normalKB*1024-1 {
		norm = append(norm, 'n')
	}
	norm = append(norm, '\n')

	spk := make([]byte, 0, spikeKB*1024)
	spk = append(spk, "spk1,spk2,"...)
	for len(spk) < spikeKB*1024-1 {
		spk = append(spk, 's')
	}
	spk = append(spk, '\n')

	return &spikeRowReader{
		totalRows:   totalRows,
		normalBytes: norm,
		spikeBytes:  spk,
		period:      period,
	}
}

func (s *spikeRowReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	written := 0
	for written < len(p) {
		if s.currentRow >= s.totalRows {
			if written > 0 {
				return written, nil
			}
			return 0, io.EOF
		}
		active := s.normalBytes
		if (s.currentRow+1)%s.period == 0 {
			active = s.spikeBytes
		}

		if s.rowOffset >= len(active) {
			s.currentRow++
			s.rowOffset = 0
			continue
		}

		n := copy(p[written:], active[s.rowOffset:])
		s.rowOffset += n
		written += n
	}
	return written, nil
}

// BenchmarkReaderPattern1000 benchmarks 1,000 rows for each of the 3 cases (starting from 512KB).
func BenchmarkReaderPattern1000(b *testing.B) {
	const totalRows = 1000

	// Case 1: Row sizes growing (512KB -> 513KB -> ... -> 524KB -> repeat)
	b.Run("Case1_Growing_512KB_to_524KB/zerocsv_1000", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			gen := newGrowingRowReader(totalRows, 512, 1, 524)
			r := zerocsv.NewReader(gen)
			fields := 0
			for {
				rec, err := r.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					b.Fatalf("err: %v", err)
				}
				fields += rec.Len()
			}
			if fields == 0 {
				b.Fatal("read 0 fields")
			}
		}
	})

	b.Run("Case1_Growing_512KB_to_524KB/stdlib_1000", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			gen := newGrowingRowReader(totalRows, 512, 1, 524)
			r := csv.NewReader(gen)
			r.FieldsPerRecord = -1
			fields := 0
			for {
				rec, err := r.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					b.Fatalf("err: %v", err)
				}
				for j := range rec {
					fields += len(rec[j])
				}
			}
		}
	})

	// Case 2: Constant row size > 512KB (640KB)
	b.Run("Case2_Constant_640KB/zerocsv_1000", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			gen := newConstantLargeRowReader(totalRows, 640)
			r := zerocsv.NewReader(gen)
			fields := 0
			for {
				rec, err := r.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					b.Fatalf("err: %v", err)
				}
				fields += rec.Len()
			}
			if fields == 0 {
				b.Fatal("read 0 fields")
			}
		}
	})

	b.Run("Case2_Constant_640KB/stdlib_1000", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			gen := newConstantLargeRowReader(totalRows, 640)
			r := csv.NewReader(gen)
			r.FieldsPerRecord = -1
			fields := 0
			for {
				rec, err := r.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					b.Fatalf("err: %v", err)
				}
				for j := range rec {
					fields += len(rec[j])
				}
			}
		}
	})

	// Case 3: Spike every 3rd row (Row 1: 3KB, Row 2: 3KB, Row 3: 640KB)
	b.Run("Case3_Spike_3KB_3KB_640KB/zerocsv_1000", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			gen := newSpikeRowReader(totalRows, 3, 640, 3)
			r := zerocsv.NewReader(gen)
			fields := 0
			for {
				rec, err := r.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					b.Fatalf("err: %v", err)
				}
				fields += rec.Len()
			}
			if fields == 0 {
				b.Fatal("read 0 fields")
			}
		}
	})

	b.Run("Case3_Spike_3KB_3KB_640KB/stdlib_1000", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			gen := newSpikeRowReader(totalRows, 3, 640, 3)
			r := csv.NewReader(gen)
			r.FieldsPerRecord = -1
			fields := 0
			for {
				rec, err := r.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					b.Fatalf("err: %v", err)
				}
				for j := range rec {
					fields += len(rec[j])
				}
			}
		}
	})
}

// TestZeroAllocsPerRecordDuringStreaming verifies that after warmup, Read() performs
// exactly 0 heap allocations per record across records > 512KB for all 3 cases.
func TestZeroAllocsPerRecordDuringStreaming(t *testing.T) {
	const numRecords = 10_000

	testCases := []struct {
		name string
		gen  io.Reader
	}{
		{
			name: "Case 1: Growing 512KB to 524KB",
			gen:  newGrowingRowReader(numRecords, 512, 1, 524),
		},
		{
			name: "Case 2: Constant 640KB",
			gen:  newConstantLargeRowReader(numRecords, 640),
		},
		{
			name: "Case 3: Spike 3KB, 3KB, 640KB",
			gen:  newSpikeRowReader(numRecords, 3, 640, 3),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := zerocsv.NewReader(tc.gen)

			// Warm up (initial buffer growth to >512KB high-watermark)
			for i := 0; i < 20; i++ {
				_, err := r.Read()
				if err != nil {
					t.Fatalf("warmup error: %v", err)
				}
			}

			// Measure allocations over subsequent reads
			allocs := testing.AllocsPerRun(100, func() {
				rec, err := r.Read()
				if err != nil {
					t.Fatalf("read error: %v", err)
				}
				_ = rec.Len()
			})

			if allocs != 0 {
				t.Fatalf("expected 0 allocs per record, got %f", allocs)
			}
			t.Logf("PASS: %s -> %.1f allocs/record (0 heap allocations on hot path)", tc.name, allocs)
		})
	}
}

// ============================================================================
// Writer Pattern Benchmarks (512KB - 640KB Payloads)
// ============================================================================

func BenchmarkWriterPattern1000(b *testing.B) {
	const totalRows = 1000

	// Case 1: Growing row sizes (512KB -> 513KB -> 514KB -> ... -> 524KB -> repeat)
	b.Run("Case1_Growing_512KB_to_524KB/zerocsv_1000", func(b *testing.B) {
		const startKB = 512
		const stepKB = 1
		const maxKB = 524
		numTemplates := (maxKB-startKB)/stepKB + 1
		templates := make([]string, numTemplates)
		for i := 0; i < numTemplates; i++ {
			kb := startKB + i*stepKB
			buf := make([]byte, kb*1024)
			for j := range buf {
				buf[j] = 'a'
			}
			templates[i] = string(buf)
		}

		cols := make([]zerocsv.Column, 2)
		b.ReportAllocs()
		b.ResetTimer()

		for n := 0; n < b.N; n++ {
			w := zerocsv.NewWriter(io.Discard)
			for row := 0; row < totalRows; row++ {
				idx := row % len(templates)
				cols[0] = zerocsv.ColumnInt64(int64(row))
				cols[1] = zerocsv.ColumnString(templates[idx])
				if err := w.Write(cols...); err != nil {
					b.Fatalf("Write: %v", err)
				}
			}
			if err := w.Flush(); err != nil {
				b.Fatalf("Flush: %v", err)
			}
		}
	})

	b.Run("Case1_Growing_512KB_to_524KB/stdlib_1000", func(b *testing.B) {
		const startKB = 512
		const stepKB = 1
		const maxKB = 524
		numTemplates := (maxKB-startKB)/stepKB + 1
		templates := make([]string, numTemplates)
		for i := 0; i < numTemplates; i++ {
			kb := startKB + i*stepKB
			buf := make([]byte, kb*1024)
			for j := range buf {
				buf[j] = 'a'
			}
			templates[i] = string(buf)
		}

		record := make([]string, 2)
		b.ReportAllocs()
		b.ResetTimer()

		for n := 0; n < b.N; n++ {
			w := csv.NewWriter(io.Discard)
			for row := 0; row < totalRows; row++ {
				idx := row % len(templates)
				record[0] = strconv.Itoa(row)
				record[1] = templates[idx]
				if err := w.Write(record); err != nil {
					b.Fatalf("Write: %v", err)
				}
			}
			w.Flush()
			if err := w.Error(); err != nil {
				b.Fatalf("Flush: %v", err)
			}
		}
	})

	// Case 2: Constant row size > 512KB (640KB per row)
	b.Run("Case2_Constant_640KB/zerocsv_1000", func(b *testing.B) {
		const sizeKB = 640
		buf := make([]byte, sizeKB*1024)
		for i := range buf {
			buf[i] = 'c'
		}
		fieldData := string(buf)

		cols := make([]zerocsv.Column, 2)
		b.ReportAllocs()
		b.ResetTimer()

		for n := 0; n < b.N; n++ {
			w := zerocsv.NewWriter(io.Discard)
			for row := 0; row < totalRows; row++ {
				cols[0] = zerocsv.ColumnInt64(int64(row))
				cols[1] = zerocsv.ColumnString(fieldData)
				if err := w.Write(cols...); err != nil {
					b.Fatalf("Write: %v", err)
				}
			}
			if err := w.Flush(); err != nil {
				b.Fatalf("Flush: %v", err)
			}
		}
	})

	b.Run("Case2_Constant_640KB/stdlib_1000", func(b *testing.B) {
		const sizeKB = 640
		buf := make([]byte, sizeKB*1024)
		for i := range buf {
			buf[i] = 'c'
		}
		fieldData := string(buf)

		record := make([]string, 2)
		b.ReportAllocs()
		b.ResetTimer()

		for n := 0; n < b.N; n++ {
			w := csv.NewWriter(io.Discard)
			for row := 0; row < totalRows; row++ {
				record[0] = strconv.Itoa(row)
				record[1] = fieldData
				if err := w.Write(record); err != nil {
					b.Fatalf("Write: %v", err)
				}
			}
			w.Flush()
			if err := w.Error(); err != nil {
				b.Fatalf("Flush: %v", err)
			}
		}
	})

	// Case 3: Spike pattern: Row 1: 3KB, Row 2: 3KB, Row 3: 640KB
	b.Run("Case3_Spike_3KB_3KB_640KB/zerocsv_1000", func(b *testing.B) {
		const normKB = 3
		const spkKB = 640
		bufNorm := make([]byte, normKB*1024)
		for i := range bufNorm {
			bufNorm[i] = 'n'
		}
		normStr := string(bufNorm)

		bufSpk := make([]byte, spkKB*1024)
		for i := range bufSpk {
			bufSpk[i] = 's'
		}
		spkStr := string(bufSpk)

		cols := make([]zerocsv.Column, 2)
		b.ReportAllocs()
		b.ResetTimer()

		for n := 0; n < b.N; n++ {
			w := zerocsv.NewWriter(io.Discard)
			for row := 0; row < totalRows; row++ {
				cols[0] = zerocsv.ColumnInt64(int64(row))
				if (row+1)%3 == 0 {
					cols[1] = zerocsv.ColumnString(spkStr)
				} else {
					cols[1] = zerocsv.ColumnString(normStr)
				}
				if err := w.Write(cols...); err != nil {
					b.Fatalf("Write: %v", err)
				}
			}
			if err := w.Flush(); err != nil {
				b.Fatalf("Flush: %v", err)
			}
		}
	})

	b.Run("Case3_Spike_3KB_3KB_640KB/stdlib_1000", func(b *testing.B) {
		const normKB = 3
		const spkKB = 640
		bufNorm := make([]byte, normKB*1024)
		for i := range bufNorm {
			bufNorm[i] = 'n'
		}
		normStr := string(bufNorm)

		bufSpk := make([]byte, spkKB*1024)
		for i := range bufSpk {
			bufSpk[i] = 's'
		}
		spkStr := string(bufSpk)

		record := make([]string, 2)
		b.ReportAllocs()
		b.ResetTimer()

		for n := 0; n < b.N; n++ {
			w := csv.NewWriter(io.Discard)
			for row := 0; row < totalRows; row++ {
				record[0] = strconv.Itoa(row)
				if (row+1)%3 == 0 {
					record[1] = spkStr
				} else {
					record[1] = normStr
				}
				if err := w.Write(record); err != nil {
					b.Fatalf("Write: %v", err)
				}
			}
			w.Flush()
			if err := w.Error(); err != nil {
				b.Fatalf("Flush: %v", err)
			}
		}
	})
}

func TestZeroAllocsPerRecordDuringWriting(t *testing.T) {
	// Verify hot-path per-row writing allocations are strictly 0.00
	const normKB = 3
	const spkKB = 640

	normBuf := bytes.Repeat([]byte("n"), normKB*1024)
	spkBuf := bytes.Repeat([]byte("s"), spkKB*1024)
	normStr := string(normBuf)
	spkStr := string(spkBuf)

	cols := make([]zerocsv.Column, 2)
	w := zerocsv.NewWriter(io.Discard)

	// Warmup 10 rows
	for i := 0; i < 10; i++ {
		cols[0] = zerocsv.ColumnInt64(int64(i))
		cols[1] = zerocsv.ColumnString(spkStr)
		if err := w.Write(cols...); err != nil {
			t.Fatalf("Write warmup: %v", err)
		}
	}

	// Test 1000 writes of alternating large spikes
	allocs := testing.AllocsPerRun(1000, func() {
		for i := 0; i < 3; i++ {
			cols[0] = zerocsv.ColumnInt64(int64(i))
			if i == 2 {
				cols[1] = zerocsv.ColumnString(spkStr)
			} else {
				cols[1] = zerocsv.ColumnString(normStr)
			}
			if err := w.Write(cols...); err != nil {
				t.Fatalf("Write: %v", err)
			}
		}
	})

	if allocs != 0 {
		t.Fatalf("Writer hot path had %.2f allocs/run, want 0", allocs)
	}
	t.Logf("PASS: Writer hot path has %.2f heap allocations across 3KB/3KB/640KB spike pattern", allocs)
}
