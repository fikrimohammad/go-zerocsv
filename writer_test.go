package zerocsv

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"math"
	"runtime"
	"strconv"
	"testing"
	"unsafe"
)

// --- constructors ---------------------------------------------------------

func TestColumnConstructors(t *testing.T) {
	if c := ColumnString("hi"); c.kind != columnString || c.Kind() != ColumnKindString || c.s != "hi" {
		t.Fatalf("ColumnString: %+v", c)
	}
	if c := ColumnBytes([]byte("hi")); c.kind != columnBytes || c.Kind() != ColumnKindBytes || c.s != "hi" {
		t.Fatalf("ColumnBytes: %+v", c)
	}
	if c := ColumnInt(-42); c.kind != columnInt || c.Kind() != ColumnKindInt || int64(c.n) != -42 {
		t.Fatalf("ColumnInt: %+v", c)
	}
	if c := ColumnUint(42); c.kind != columnUint || c.Kind() != ColumnKindUint || c.n != 42 {
		t.Fatalf("ColumnUint: %+v", c)
	}
	if c := ColumnFloat64(2.5); c.kind != columnFloat || c.Kind() != ColumnKindFloat || math.Float64frombits(c.n) != 2.5 {
		t.Fatalf("ColumnFloat64: %+v", c)
	}
	if c := ColumnFloat32(1.5); c.kind != columnFloat32 || c.Kind() != ColumnKindFloat32 || math.Float64frombits(c.n) != 1.5 {
		t.Fatalf("ColumnFloat32: %+v", c)
	}
	if c := ColumnBool(true); c.kind != columnBool || c.Kind() != ColumnKindBool || c.n != 1 {
		t.Fatalf("ColumnBool(true): %+v", c)
	}
	if c := ColumnBool(false); c.kind != columnBool || c.Kind() != ColumnKindBool || c.n != 0 {
		t.Fatalf("ColumnBool(false): %+v", c)
	}
	tv := testFieldValuer{"x"}
	if c := ColumnValuer(tv); c.kind != columnValuer || c.Kind() != ColumnKindValuer || c.valuer != tv {
		t.Fatalf("ColumnValuer: %+v", c)
	}
}

func TestColumnStructSize(t *testing.T) {
	if sz := unsafe.Sizeof(Column{}); sz != 48 {
		t.Fatalf("sizeof(Column) = %d bytes, want 48 bytes", sz)
	}
}

func TestColumnIntegerWidths(t *testing.T) {
	cases := []Column{
		ColumnInt8(-8), ColumnInt16(-16), ColumnInt32(-32), ColumnInt64(-64),
		ColumnUint8(8), ColumnUint16(16), ColumnUint32(32), ColumnUint64(64), ColumnUintptr(0xbad),
	}
	want := []string{"-8", "-16", "-32", "-64", "8", "16", "32", "64", "2989"}
	var buf bytes.Buffer
	w := NewWriter(&buf)
	for i, c := range cases {
		if err := w.Write(c); err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	var lines []string
	for _, l := range bytes.Split(bytes.TrimRight(buf.Bytes(), "\n"), []byte("\n")) {
		lines = append(lines, string(l))
	}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(lines), len(want))
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("line %d: got %q, want %q", i, lines[i], want[i])
		}
	}
}

// --- basic writing --------------------------------------------------------

func TestWriteBasic(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.Write(ColumnString("a"), ColumnString("b"), ColumnString("c")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got, want := buf.String(), "a,b,c\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteEmptyRecord(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.Write(); err != ErrEmptyRecord {
		t.Fatalf("Write() error = %v, want %v", err, ErrEmptyRecord)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got, want := buf.String(), ""; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteNumbers(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	cols := []Column{
		ColumnInt(-42),
		ColumnInt64(9223372036854775807),
		ColumnUint(42),
		ColumnUint64(18446744073709551615),
		ColumnFloat32(2.5),
		ColumnFloat64(3.14),
		ColumnFloat64(1e10),
		ColumnBool(true),
		ColumnBool(false),
	}
	if err := w.Write(cols...); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	want := "-42,9223372036854775807,42,18446744073709551615,2.5,3.14,10000000000,true,false\n"
	if got := buf.String(); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteBytes(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.Write(ColumnBytes([]byte("a,b")), ColumnBytes([]byte("plain"))); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got, want := buf.String(), "\"a,b\",plain\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteFloat32(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.Write(ColumnFloat32(0.1), ColumnFloat32(2.5), ColumnFloat32(1.0/3.0)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got, want := buf.String(), "0.1,2.5,0.33333334\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWithDelimiterInvalid(t *testing.T) {
	for _, c := range []byte{0, '"', '\r', '\n', 0x80, 0xff, 'é'} {
		w := NewWriter(io.Discard, WithDelimiter(c))
		if err := w.Error(); err != ErrInvalidDelim {
			t.Errorf("WithDelimiter(%q): got err %v, want %v", c, err, ErrInvalidDelim)
		}
	}
	for _, c := range []byte{',', '\t', ';', '|', ' '} {
		w := NewWriter(io.Discard, WithDelimiter(c))
		if err := w.Error(); err != nil {
			t.Errorf("WithDelimiter(%q): unexpected error %v", c, err)
		}
	}
}

func TestWriteAll(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	rows := [][]Column{
		{ColumnString("a"), ColumnInt(1)},
		{ColumnString("c"), ColumnFloat64(2.5)},
	}
	// WriteAll flushes automatically like encoding/csv
	if err := w.WriteAll(rows); err != nil {
		t.Fatalf("WriteAll: %v", err)
	}
	if got, want := buf.String(), "a,1\nc,2.5\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// --- quoting --------------------------------------------------------------

func TestWriteQuoting(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	rows := [][]Column{
		{ColumnString("simple"), ColumnString("field")},
		{ColumnString("comma,inside"), ColumnString("plain")},
		{ColumnString(`quote"inside`), ColumnString("plain")},
		{ColumnString("newline\ninside"), ColumnString("plain")},
		{ColumnString(`"both,edge"`), ColumnString("plain")},
		{ColumnString(""), ColumnString("empty")},
	}
	if err := w.WriteAll(rows); err != nil {
		t.Fatalf("WriteAll: %v", err)
	}
	want := "" +
		"simple,field\n" +
		`"comma,inside",plain` + "\n" +
		`"quote""inside",plain` + "\n" +
		"\"newline\ninside\",plain\n" +
		`"""both,edge""",plain` + "\n" +
		",empty\n"
	if got := buf.String(); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteValuerQuotes(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.Write(ColumnValuer(testFieldValuer{"a,b"}), ColumnValuer(testFieldValuer{"plain"}), ColumnValuer(nil)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got, want := buf.String(), "\"a,b\",plain,\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFieldNeedsQuotes(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"plain", false},
		{"with,comma", true},
		{`with"quote`, true},
		{"with\nnewline", true},
		{"with\rreturn", true},
		{"", false},
		{" leading", true},
		{"trailing ", false},
		{"\tleading", true},
	}
	for _, c := range cases {
		if got := fieldNeedsQuotes(c.in, ','); got != c.want {
			t.Errorf("fieldNeedsQuotes(%q): got %v, want %v", c.in, got, c.want)
		}
	}
}

// --- delimiter / line ending ---------------------------------------------

func TestWriteCustomComma(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, WithDelimiter('\t'))
	if err := w.Write(ColumnString("a"), ColumnInt(1), ColumnString("b,c")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	// tab is the delimiter, so a comma inside a field no longer forces quotes
	if got, want := buf.String(), "a\t1\tb,c\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteCRLF(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, WithCRLF())
	if err := w.Write(ColumnString("a"), ColumnString("b")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got, want := buf.String(), "a,b\r\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteCombinedOptions(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, WithDelimiter(';'), WithCRLF())
	if err := w.Write(ColumnString("a"), ColumnString("b;c")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got, want := buf.String(), "a;\"b;c\"\r\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// --- custom valuer --------------------------------------------------------

type testFieldValuer struct {
	text string
}

func (v testFieldValuer) AppendCSV(dst []byte) ([]byte, error) {
	return append(dst, v.text...), nil
}

func TestWriteFieldValuer(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.Write(ColumnValuer(testFieldValuer{"custom,val"}), ColumnValuer(nil)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got, want := buf.String(), "\"custom,val\",\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// --- error handling -------------------------------------------------------

type failWriter struct{ n int }

func (f *failWriter) Write(p []byte) (int, error) {
	if f.n == 0 {
		return 0, errors.New("boom")
	}
	f.n--
	return len(p), nil
}

func TestErrorSticky(t *testing.T) {
	w := NewWriter(&failWriter{})
	// small writes are buffered, so the underlying error surfaces on Flush
	if err := w.Write(ColumnString("a"), ColumnString("b")); err != nil {
		t.Fatalf("unexpected error before flush: %v", err)
	}
	if err := w.Flush(); err == nil {
		t.Fatal("expected error on flush")
	}
	if err := w.Write(ColumnString("c"), ColumnString("d")); err == nil {
		t.Fatal("expected sticky error")
	}
	if err := w.Flush(); err == nil {
		t.Fatal("expected sticky error on flush")
	}
	if err := w.Error(); err == nil {
		t.Fatal("expected Error() to return sticky error")
	}
}

func TestErrorDuringWrite(t *testing.T) {
	// write enough to overflow the bufio buffer so the error surfaces on Write
	big := make([]byte, 8192)
	for i := range big {
		big[i] = 'x'
	}
	w := NewWriter(&failWriter{})
	if err := w.Write(ColumnBytes(big)); err == nil {
		t.Fatal("expected error from oversized write")
	}
	if err := w.Error(); err == nil {
		t.Fatal("expected Error() to be set")
	}
}

// --- conformance ----------------------------------------------------------

func TestRoundTripAgainstEncodingCSV(t *testing.T) {
	records := [][]string{
		{"a", "b", "c"},
		{"x,y", "z"},
		{`q"r`, "s"},
		{"multi\nline", "t"},
		{"", ""},
		{" tab", "trailing "},
	}
	var ours, std bytes.Buffer
	wo := NewWriter(&ours, WithFieldsPerRecord(-1))
	ws := csv.NewWriter(&std)
	for _, r := range records {
		cols := make([]Column, len(r))
		for i, f := range r {
			cols[i] = ColumnString(f)
		}
		if err := wo.Write(cols...); err != nil {
			t.Fatalf("ours: %v", err)
		}
		if err := ws.Write(r); err != nil {
			t.Fatalf("std: %v", err)
		}
	}
	if err := wo.Flush(); err != nil {
		t.Fatalf("ours flush: %v", err)
	}
	ws.Flush()
	if ours.String() != std.String() {
		t.Fatalf("mismatch:\nours=%q\nstd =%q", ours.String(), std.String())
	}
}

func TestWriteWithBufioUnderlying(t *testing.T) {
	var buf bytes.Buffer
	outer := bufio.NewWriter(&buf)
	w := NewWriter(outer)
	if err := w.WriteAll([][]Column{
		{ColumnString("a"), ColumnString("b,c")},
		{ColumnInt(1), ColumnBool(true)},
	}); err != nil {
		t.Fatalf("WriteAll: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got, want := buf.String(), "a,\"b,c\"\n1,true\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// --- zero allocation ------------------------------------------------------

func TestWriteZeroAllocs(t *testing.T) {
	stringCols := []Column{ColumnString("alpha"), ColumnString("beta"), ColumnString("gamma")}
	mixedCols := []Column{ColumnString("alpha"), ColumnInt(42), ColumnFloat64(3.14), ColumnBool(true)}
	bytesCols := []Column{ColumnString("ts"), ColumnBytes([]byte("2026-08-17T12:34:56Z"))}
	valuerCols := []Column{ColumnString("val"), ColumnValuer(testFieldValuer{"custom"})}

	tests := []struct {
		name string
		cols []Column
	}{
		{"strings", stringCols},
		{"mixed", mixedCols},
		{"bytes", bytesCols},
		{"valuer", valuerCols},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := NewWriter(io.Discard)
			if got := testing.AllocsPerRun(1000, func() {
				if err := w.Write(tc.cols...); err != nil {
					t.Fatalf("Write: %v", err)
				}
			}); got != 0 {
				t.Fatalf("Write allocated %v times per run, want 0", got)
			}
		})
	}
}

func TestWriteZeroAllocsBufioUnderlying(t *testing.T) {
	cols := []Column{ColumnString("alpha"), ColumnInt(42), ColumnFloat64(3.14), ColumnBool(true)}
	outer := bufio.NewWriter(io.Discard)
	w := NewWriter(outer)
	if got := testing.AllocsPerRun(1000, func() {
		if err := w.Write(cols...); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}); got != 0 {
		t.Fatalf("Write allocated %v times per run, want 0", got)
	}
}

func TestFirstWriteZeroAllocs(t *testing.T) {
	cols := []Column{ColumnInt(42), ColumnFloat64(3.14), ColumnBytes([]byte("raw-bytes")), ColumnBool(true)}
	w := NewWriter(io.Discard)

	runtime.GC()
	var m0, m1 runtime.MemStats
	runtime.ReadMemStats(&m0)
	if err := w.Write(cols...); err != nil {
		t.Fatalf("Write: %v", err)
	}
	runtime.ReadMemStats(&m1)
	if got := m1.Mallocs - m0.Mallocs; got != 0 {
		t.Fatalf("first Write allocated %d times, want 0", got)
	}
}

// --- fuzzing --------------------------------------------------------------

// FuzzWriterConformance verifies that writing arbitrary string fields produces
// byte-identical output to encoding/csv.
func FuzzWriterConformance(f *testing.F) {
	f.Add("", "", "")
	f.Add("a,b", `"q"`, "multi\nline")
	f.Add(" leading", "trailing ", `\.`)
	f.Add("tab\tvalue", "üñïçø∂é", "日本語")

	f.Fuzz(func(t *testing.T, a, b, c string) {
		rec := []string{a, b, c}
		cols := []Column{ColumnString(a), ColumnString(b), ColumnString(c)}

		var ours, std bytes.Buffer
		wo := NewWriter(&ours)
		ws := csv.NewWriter(&std)

		if err := wo.Write(cols...); err != nil {
			t.Fatalf("zerocsv Write: %v", err)
		}
		if err := ws.Write(rec); err != nil {
			t.Fatalf("stdlib Write: %v", err)
		}
		if err := wo.Flush(); err != nil {
			t.Fatalf("zerocsv Flush: %v", err)
		}
		ws.Flush()

		if ours.String() != std.String() {
			t.Fatalf("record %q:\n  zerocsv=%q\n  stdlib =%q", rec, ours.String(), std.String())
		}
	})
}

// --- fields per record -----------------------------------------------------

func TestWriteFieldsPerRecordExact(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, WithFieldsPerRecord(2))
	if err := w.Write(ColumnString("a"), ColumnString("b")); err != nil {
		t.Fatalf("Write(0): %v", err)
	}
	if err := w.Write(ColumnString("c"), ColumnString("d")); err != nil {
		t.Fatalf("Write(1): %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got, want := buf.String(), "a,b\nc,d\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteFieldsPerRecordMismatch(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, WithFieldsPerRecord(3))

	// The mismatched record is still written, and writing continues.
	if err := w.Write(ColumnString("a"), ColumnString("b")); !errors.Is(err, ErrFieldCount) {
		t.Fatalf("Write(0) error = %v, want ErrFieldCount", err)
	}
	if err := w.Write(ColumnString("x"), ColumnString("y"), ColumnString("z")); err != nil {
		t.Fatalf("Write(1): %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got, want := buf.String(), "a,b\nx,y,z\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	// ErrFieldCount is non-fatal: Error() is not poisoned.
	if err := w.Error(); err != nil {
		t.Fatalf("Error() = %v after ErrFieldCount, want nil", err)
	}
}

func TestWriteFieldsPerRecordAutoDetect(t *testing.T) {
	// The default (0) and an explicit WithFieldsPerRecord(0) both learn the
	// field count from the first written record and enforce it from then on.
	for _, opts := range [][]Option{nil, {WithFieldsPerRecord(0)}} {
		var buf bytes.Buffer
		w := NewWriter(&buf, opts...)
		if err := w.Write(ColumnString("a"), ColumnString("b"), ColumnString("c")); err != nil {
			t.Fatalf("Write(0): %v", err)
		}
		if err := w.Write(ColumnString("1"), ColumnString("2")); !errors.Is(err, ErrFieldCount) {
			t.Fatalf("Write(1) error = %v, want ErrFieldCount", err)
		}
		if err := w.Flush(); err != nil {
			t.Fatalf("Flush: %v", err)
		}
		if got, want := buf.String(), "a,b,c\n1,2\n"; got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}

func TestWriteFieldsPerRecordDisabled(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, WithFieldsPerRecord(-1))
	if err := w.Write(ColumnString("a"), ColumnString("b"), ColumnString("c")); err != nil {
		t.Fatalf("Write(0): %v", err)
	}
	if err := w.Write(ColumnString("1")); err != nil {
		t.Fatalf("Write(1): %v", err)
	}
	if err := w.Write(ColumnString("x"), ColumnString("y")); err != nil {
		t.Fatalf("Write(2): %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got, want := buf.String(), "a,b,c\n1\nx,y\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteFieldsPerRecordEmptyRecord(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, WithFieldsPerRecord(2))

	// An empty record is still ErrEmptyRecord, and does not set the count.
	if err := w.Write(); err != ErrEmptyRecord {
		t.Fatalf("Write() error = %v, want ErrEmptyRecord", err)
	}
	if err := w.Write(ColumnString("a"), ColumnString("b")); err != nil {
		t.Fatalf("Write(1): %v", err)
	}
	if err := w.Write(ColumnString("x")); !errors.Is(err, ErrFieldCount) {
		t.Fatalf("Write(2) error = %v, want ErrFieldCount", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got, want := buf.String(), "a,b\nx\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteAllFieldsPerRecord(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, WithFieldsPerRecord(2))
	rows := [][]Column{
		{ColumnString("a"), ColumnString("b")},
		{ColumnString("c")},
		{ColumnString("d"), ColumnString("e")},
	}
	// WriteAll stops at the first error: the third row is never written.
	if err := w.WriteAll(rows); !errors.Is(err, ErrFieldCount) {
		t.Fatalf("WriteAll error = %v, want ErrFieldCount", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got, want := buf.String(), "a,b\nc\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteFieldsPerRecordZeroAllocs(t *testing.T) {
	cols := []Column{ColumnString("a"), ColumnString("b"), ColumnString("c")}
	w := NewWriter(io.Discard, WithFieldsPerRecord(3))
	if got := testing.AllocsPerRun(1000, func() {
		if err := w.Write(cols...); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}); got != 0 {
		t.Fatalf("Write allocated %v times per run, want 0", got)
	}
}

func TestWriterFieldsPerRecordAccessor(t *testing.T) {
	// Auto-detect: 0 before any record is written, then the learned count.
	w := NewWriter(io.Discard)
	if got := w.FieldsPerRecord(); got != 0 {
		t.Fatalf("FieldsPerRecord() before write = %d, want 0", got)
	}
	if err := w.Write(ColumnString("a"), ColumnString("b")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := w.FieldsPerRecord(); got != 2 {
		t.Fatalf("FieldsPerRecord() after write = %d, want 2", got)
	}

	// Explicit positive count is reported as configured.
	w = NewWriter(io.Discard, WithFieldsPerRecord(5))
	if got := w.FieldsPerRecord(); got != 5 {
		t.Fatalf("FieldsPerRecord() = %d, want 5", got)
	}

	// Disabled mode stays negative.
	w = NewWriter(io.Discard, WithFieldsPerRecord(-1))
	if got := w.FieldsPerRecord(); got != -1 {
		t.Fatalf("FieldsPerRecord() = %d, want -1", got)
	}
}

func TestWriteFieldsPerRecordCombinedOptions(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, WithFieldsPerRecord(2), WithCRLF(), WithDelimiter(';'))

	if err := w.Write(ColumnString("a"), ColumnString("b")); err != nil {
		t.Fatalf("Write(0): %v", err)
	}
	if err := w.Write(ColumnString("c"), ColumnString("d;e"), ColumnString("f")); !errors.Is(err, ErrFieldCount) {
		t.Fatalf("Write(1) error = %v, want ErrFieldCount", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	// CRLF endings, ';' delimiter, and the mismatched record still written.
	if got, want := buf.String(), "a;b\r\nc;\"d;e\";f\r\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// FuzzWriterFieldCount verifies that with the default auto-detect field
// count, every record is fully written (the output is the exact concatenation
// of all records) and ErrFieldCount is returned precisely for records whose
// width differs from the first record's.
func FuzzWriterFieldCount(f *testing.F) {
	f.Add(2, 2, 2, 2)
	f.Add(1, 2, 3, 4)
	f.Add(5, 1, 5, 5)
	f.Add(3, 3, 2, 1)
	f.Fuzz(func(t *testing.T, w0, w1, w2, w3 int) {
		widths := []int{w0, w1, w2, w3}
		for i := range widths {
			// uint cast keeps the modulo non-negative for negative inputs.
			widths[i] = 1 + int(uint(widths[i])%5) // width in [1,5]
		}

		var buf bytes.Buffer
		w := NewWriter(&buf)
		var want string
		for i, n := range widths {
			cols := make([]Column, n)
			for j := range cols {
				cols[j] = ColumnString(strconv.Itoa(j))
			}
			err := w.Write(cols...)
			var wantErr error
			if i > 0 && n != widths[0] {
				wantErr = ErrFieldCount
			}
			if !errors.Is(err, wantErr) {
				t.Fatalf("record %d: error = %v, want %v (widths=%v)", i, err, wantErr, widths)
			}
			for j := 0; j < n; j++ {
				if j > 0 {
					want += ","
				}
				want += strconv.Itoa(j)
			}
			want += "\n"
		}
		if err := w.Flush(); err != nil {
			t.Fatalf("Flush: %v", err)
		}
		if buf.String() != want {
			t.Fatalf("output mismatch: got %q, want %q", buf.String(), want)
		}
	})
}
