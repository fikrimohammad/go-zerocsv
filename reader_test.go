package zerocsv

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func readAllZerocsv(t *testing.T, input string, opts ...Option) ([][]string, error) {
	t.Helper()
	r := NewReader(strings.NewReader(input), opts...)
	var rows [][]string
	for {
		rec, err := r.Next()
		if err == io.EOF {
			return rows, nil
		}
		if err != nil {
			return rows, err
		}
		rows = append(rows, rec.Copy()) // Copy: owned strings
	}
}

func readAllStdlib(input string) ([][]string, error) {
	r := csv.NewReader(strings.NewReader(input))
	r.FieldsPerRecord = -1 // don't enforce record-length consistency
	var rows [][]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return rows, nil
		}
		if err != nil {
			return rows, err
		}
		rows = append(rows, rec)
	}
}

func TestReadBasic(t *testing.T) {
	rows, err := readAllZerocsv(t, "a,b,c\n1,2,3\n")
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	want := [][]string{{"a", "b", "c"}, {"1", "2", "3"}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("got %q, want %q", rows, want)
	}
}

func TestReadQuoting(t *testing.T) {
	input := "" +
		`"comma,inside",plain` + "\n" +
		`"quote""inside",x` + "\n" +
		"\"newline\ninside\",y\n" +
		",empty\n"
	rows, err := readAllZerocsv(t, input)
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	want := [][]string{
		{"comma,inside", "plain"},
		{`quote"inside`, "x"},
		{"newline\ninside", "y"},
		{"", "empty"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("got %q, want %q", rows, want)
	}
}

func TestReadEmptyFields(t *testing.T) {
	rows, err := readAllZerocsv(t, "a,,c\n,,\n\"\",x\n", WithFieldsPerRecord(-1))
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	want := [][]string{{"a", "", "c"}, {"", "", ""}, {"", "x"}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("got %q, want %q", rows, want)
	}
}

func TestReadNoTrailingNewline(t *testing.T) {
	rows, err := readAllZerocsv(t, "a,b\nc,d")
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	want := [][]string{{"a", "b"}, {"c", "d"}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("got %q, want %q", rows, want)
	}
}

func TestReadCRLF(t *testing.T) {
	rows, err := readAllZerocsv(t, "a,b\r\nc,d\r\n")
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	want := [][]string{{"a", "b"}, {"c", "d"}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("got %q, want %q", rows, want)
	}
}

func TestReadSkipsBlankLines(t *testing.T) {
	rows, err := readAllZerocsv(t, "a\n\nb\n\n\nc\n")
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	want := [][]string{{"a"}, {"b"}, {"c"}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("got %q, want %q", rows, want)
	}
}

func TestReadCRLFNormalization(t *testing.T) {
	cases := []struct {
		input string
		want  [][]string
	}{
		{"\"a\r\nb\"\n", [][]string{{"a\nb"}}}, // \r\n inside quotes -> \n
		{"a\r\nb\n", [][]string{{"a"}, {"b"}}},
		{"a\rb\n", [][]string{{"a\rb"}}},   // bare \r kept
		{"a\r\n", [][]string{{"a"}}},       // trailing CRLF
		{"a\r", [][]string{{"a"}}},         // trailing \r at EOF dropped
		{"a\rb\r", [][]string{{"a\rb"}}},   // only trailing \r dropped
		{"\"a\r\"\n", [][]string{{"a\r"}}}, // \r not before \n kept
		{"a,\r\nb\n", [][]string{{"a", ""}, {"b"}}},
	}
	for _, c := range cases {
		rows, err := readAllZerocsv(t, c.input, WithFieldsPerRecord(-1))
		if err != nil {
			t.Fatalf("readAll(%q): %v", c.input, err)
		}
		if !reflect.DeepEqual(rows, c.want) {
			t.Fatalf("readAll(%q) = %q, want %q", c.input, rows, c.want)
		}
	}
}

func TestReadEmptyInput(t *testing.T) {
	rows, err := readAllZerocsv(t, "")
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("got %q, want no rows", rows)
	}
}

func TestReadCustomDelimiter(t *testing.T) {
	rows, err := readAllZerocsv(t, "a\t1\tb,c\n", WithDelimiter('\t'))
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	want := [][]string{{"a", "1", "b,c"}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("got %q, want %q", rows, want)
	}
}

func TestReadErrors(t *testing.T) {
	cases := []struct {
		name  string
		input string
		err   error
	}{
		{"bare quote", `a"b,c` + "\n", ErrBareQuote},
		{"bad quote", `"a"b,c` + "\n", ErrQuote},
		{"unterminated", `"a,b` + "\n", ErrQuote},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := readAllZerocsv(t, c.input)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), c.err.Error()) {
				t.Fatalf("got error %q, want one containing %q", err, c.err)
			}
		})
	}
}

func TestReadLazyQuotes(t *testing.T) {
	rows, err := readAllZerocsv(t, `a"b,c`+"\n", WithLazyQuotes())
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	want := [][]string{{`a"b`, "c"}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("got %q, want %q", rows, want)
	}
}

// oneByteReader forces the Reader to refill its buffer on every byte,
// exercising the record-spanning-chunk paths.
type oneByteReader struct {
	data []byte
	i    int
}

func (r *oneByteReader) Read(p []byte) (int, error) {
	if r.i >= len(r.data) {
		return 0, io.EOF
	}
	p[0] = r.data[r.i]
	r.i++
	return 1, nil
}

func TestReadChunked(t *testing.T) {
	input := "\"multi,line\nrecord\",x,y\nplain,2,3\n\"a\"\"b\",,last\n"
	r := NewReader(&oneByteReader{data: []byte(input)})

	var got [][]string
	for {
		rec, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		row := make([]string, rec.Len())
		for i := 0; i < rec.Len(); i++ {
			row[i] = string(rec.ValueAt(i))
		}
		got = append(got, row)
	}

	want := [][]string{
		{"multi,line\nrecord", "x", "y"},
		{"plain", "2", "3"},
		{`a"b`, "", "last"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRecord(t *testing.T) {
	r := NewReader(strings.NewReader("a,b,c\n1,2\n"))
	rec, err := r.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if rec.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", rec.Len())
	}
	if got := string(rec.ValueAt(0)); got != "a" {
		t.Fatalf("ValueAt(0) = %q, want %q", got, "a")
	}
	if got := string(rec.ValueAt(2)); got != "c" {
		t.Fatalf("ValueAt(2) = %q, want %q", got, "c")
	}
}

func TestRecordCopyOwnsData(t *testing.T) {
	r := NewReader(strings.NewReader("a,b,c\n1,2,3\n"))
	rec, err := r.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	got := rec.Copy()
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Copy() = %q, want %q", got, want)
	}
	// Advancing the reader must not disturb the copied strings.
	if _, err := r.Next(); err != nil {
		t.Fatalf("Next: %v", err)
	}
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Copy() after advancing reader = %q, want %q", got, want)
	}
}

func TestReaderInvalidDelimiter(t *testing.T) {
	for _, d := range []byte{0, '"', '\r', '\n'} {
		r := NewReader(strings.NewReader("a,b\n"), WithDelimiter(d))
		if err := r.Error(); err == nil {
			t.Fatalf("Error() = nil for delimiter %d, want ErrInvalidDelim", d)
		}
		if _, err := r.Next(); err == nil {
			t.Fatalf("Next() = nil error for delimiter %d, want ErrInvalidDelim", d)
		}
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	rows := [][]Column{
		{ColumnString("a,b"), ColumnString(`quote"inside`), ColumnString("line\nbreak")},
		{ColumnString(""), ColumnInt(42), ColumnBool(true)},
		{ColumnFloat64(1.5), ColumnUint(7), ColumnString("semi;colon")},
	}
	if err := w.WriteAll(rows); err != nil {
		t.Fatalf("WriteAll: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	r := NewReader(&buf)
	var got [][]string
	for {
		rec, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		got = append(got, rec.Copy())
	}
	want := [][]string{
		{"a,b", `quote"inside`, "line\nbreak"},
		{"", "42", "true"},
		{"1.5", "7", "semi;colon"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %q, want %q", got, want)
	}
}

func TestReaderZeroAllocs(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		sb.WriteString("alpha,beta,42,3.14,true\n")
	}
	r := NewReader(strings.NewReader(sb.String()))

	// Warm up (fills the buffer).
	if _, err := r.Next(); err != nil {
		t.Fatalf("warmup Next: %v", err)
	}
	if got := testing.AllocsPerRun(50, func() {
		rec, err := r.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		_ = rec.Len()
	}); got != 0 {
		t.Fatalf("Next allocated %v times per run, want 0", got)
	}
}

// noProgressReader always reports (0, nil), exercising the no-progress guard
// in fill.
type noProgressReader struct{}

func (noProgressReader) Read(p []byte) (int, error) {
	return 0, nil
}

func TestReadNoProgress(t *testing.T) {
	r := NewReader(noProgressReader{})
	if _, err := r.Next(); err != io.ErrNoProgress {
		t.Fatalf("Next() error = %v, want io.ErrNoProgress", err)
	}
}

func TestReaderErrorNilAfterCleanEOF(t *testing.T) {
	r := NewReader(strings.NewReader("a,b\n"))
	for {
		_, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
	}
	if err := r.Error(); err != nil {
		t.Fatalf("Error() after clean EOF = %v, want nil", err)
	}
}

// --- fields per record -----------------------------------------------------

func TestReadFieldsPerRecordExact(t *testing.T) {
	rows, err := readAllZerocsv(t, "a,b,c\n1,2,3\n", WithFieldsPerRecord(3))
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	want := [][]string{{"a", "b", "c"}, {"1", "2", "3"}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("got %q, want %q", rows, want)
	}
}

func TestReadFieldsPerRecordMismatch(t *testing.T) {
	r := NewReader(strings.NewReader("a,b,c\n1,2\nx,y,z\n"), WithFieldsPerRecord(3))

	rec, err := r.Next()
	if err != nil {
		t.Fatalf("Next(0): %v", err)
	}
	if got := rec.Copy(); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("Next(0) = %q, want %q", got, []string{"a", "b", "c"})
	}

	// The mismatched record is returned alongside ErrFieldCount, and reading
	// continues with the next record, like encoding/csv.
	rec, err = r.Next()
	if !errors.Is(err, ErrFieldCount) {
		t.Fatalf("Next(1) error = %v, want ErrFieldCount", err)
	}
	if got := rec.Copy(); !reflect.DeepEqual(got, []string{"1", "2"}) {
		t.Fatalf("Next(1) = %q, want %q", got, []string{"1", "2"})
	}

	rec, err = r.Next()
	if err != nil {
		t.Fatalf("Next(2): %v", err)
	}
	if got := rec.Copy(); !reflect.DeepEqual(got, []string{"x", "y", "z"}) {
		t.Fatalf("Next(2) = %q, want %q", got, []string{"x", "y", "z"})
	}

	if _, err := r.Next(); err != io.EOF {
		t.Fatalf("Next(3) = %v, want io.EOF", err)
	}
}

func TestReadFieldsPerRecordAutoDetect(t *testing.T) {
	// The default (0) and an explicit WithFieldsPerRecord(0) both learn the
	// field count from the first record and enforce it from then on.
	for _, opts := range [][]Option{nil, {WithFieldsPerRecord(0)}} {
		r := NewReader(strings.NewReader("a,b\n1,2,3\n"), opts...)

		rec, err := r.Next()
		if err != nil {
			t.Fatalf("Next(0): %v", err)
		}
		if got := rec.Copy(); !reflect.DeepEqual(got, []string{"a", "b"}) {
			t.Fatalf("Next(0) = %q, want %q", got, []string{"a", "b"})
		}

		rec, err = r.Next()
		if !errors.Is(err, ErrFieldCount) {
			t.Fatalf("Next(1) error = %v, want ErrFieldCount", err)
		}
		if got := rec.Copy(); !reflect.DeepEqual(got, []string{"1", "2", "3"}) {
			t.Fatalf("Next(1) = %q, want %q", got, []string{"1", "2", "3"})
		}
	}
}

func TestReadFieldsPerRecordDisabled(t *testing.T) {
	rows, err := readAllZerocsv(t, "a,b,c\n1,2\nx\n", WithFieldsPerRecord(-1))
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	want := [][]string{{"a", "b", "c"}, {"1", "2"}, {"x"}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("got %q, want %q", rows, want)
	}
}

func TestReadFieldsPerRecordFirstRecordWrong(t *testing.T) {
	r := NewReader(strings.NewReader("a,b,c\n1,2\n"), WithFieldsPerRecord(2))

	rec, err := r.Next()
	if !errors.Is(err, ErrFieldCount) {
		t.Fatalf("Next(0) error = %v, want ErrFieldCount", err)
	}
	if got := rec.Copy(); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("Next(0) = %q, want %q", got, []string{"a", "b", "c"})
	}

	rec, err = r.Next()
	if err != nil {
		t.Fatalf("Next(1): %v", err)
	}
	if got := rec.Copy(); !reflect.DeepEqual(got, []string{"1", "2"}) {
		t.Fatalf("Next(1) = %q, want %q", got, []string{"1", "2"})
	}
}

func TestReadFieldsPerRecordBlankLines(t *testing.T) {
	// Blank lines are skipped and never set or violate the count.
	r := NewReader(strings.NewReader("\n\n\nx,y\n\n1,2\n\n"))

	rec, err := r.Next()
	if err != nil {
		t.Fatalf("Next(0): %v", err)
	}
	if got := rec.Copy(); !reflect.DeepEqual(got, []string{"x", "y"}) {
		t.Fatalf("Next(0) = %q, want %q", got, []string{"x", "y"})
	}

	rec, err = r.Next()
	if err != nil {
		t.Fatalf("Next(1): %v", err)
	}
	if got := rec.Copy(); !reflect.DeepEqual(got, []string{"1", "2"}) {
		t.Fatalf("Next(1) = %q, want %q", got, []string{"1", "2"})
	}

	if _, err := r.Next(); err != io.EOF {
		t.Fatalf("Next(2) = %v, want io.EOF", err)
	}
}

func TestReadFieldsPerRecordBlankLineDoesNotLearnCount(t *testing.T) {
	// A file whose first record is mismatched after leading blank lines still
	// reports the mismatch: the count is learned from the first non-blank row.
	r := NewReader(strings.NewReader("\na,b,c\n1,2\n"), WithFieldsPerRecord(0))

	rec, err := r.Next()
	if err != nil {
		t.Fatalf("Next(0): %v", err)
	}
	if got := rec.Copy(); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("Next(0) = %q, want %q", got, []string{"a", "b", "c"})
	}

	rec, err = r.Next()
	if !errors.Is(err, ErrFieldCount) {
		t.Fatalf("Next(1) error = %v, want ErrFieldCount", err)
	}
	if got := rec.Copy(); !reflect.DeepEqual(got, []string{"1", "2"}) {
		t.Fatalf("Next(1) = %q, want %q", got, []string{"1", "2"})
	}
}

func TestReadFieldsPerRecordChunked(t *testing.T) {
	// Records split across buffer boundaries count fields, not lines.
	input := "\"multi\nline\",x\ny,z\n"
	r := NewReader(&oneByteReader{data: []byte(input)}, WithFieldsPerRecord(2))

	rec, err := r.Next()
	if err != nil {
		t.Fatalf("Next(0): %v", err)
	}
	want0 := []string{"multi\nline", "x"}
	if got := rec.Copy(); !reflect.DeepEqual(got, want0) {
		t.Fatalf("Next(0) = %q, want %q", got, want0)
	}

	rec, err = r.Next()
	if err != nil {
		t.Fatalf("Next(1): %v", err)
	}
	want1 := []string{"y", "z"}
	if got := rec.Copy(); !reflect.DeepEqual(got, want1) {
		t.Fatalf("Next(1) = %q, want %q", got, want1)
	}
}

func TestReadFieldsPerRecordEmptyInput(t *testing.T) {
	for _, opts := range [][]Option{nil, {WithFieldsPerRecord(1)}, {WithFieldsPerRecord(-1)}} {
		rows, err := readAllZerocsv(t, "", opts...)
		if err != nil {
			t.Fatalf("readAll: %v", err)
		}
		if len(rows) != 0 {
			t.Fatalf("got %q, want no rows", rows)
		}
	}
}

func TestReadFieldsPerRecordNonFatal(t *testing.T) {
	// ErrFieldCount does not stick: Error() stays nil and reading continues.
	r := NewReader(strings.NewReader("a,b\nc\n"))

	if _, err := r.Next(); err != nil {
		t.Fatalf("Next(0): %v", err)
	}
	if _, err := r.Next(); !errors.Is(err, ErrFieldCount) {
		t.Fatalf("Next(1) error = %v, want ErrFieldCount", err)
	}
	if err := r.Error(); err != nil {
		t.Fatalf("Error() = %v after ErrFieldCount, want nil", err)
	}
	if _, err := r.Next(); err != io.EOF {
		t.Fatalf("Next(2) = %v, want io.EOF", err)
	}
	if err := r.Error(); err != nil {
		t.Fatalf("Error() = %v after EOF, want nil", err)
	}
}

func TestReaderFieldsPerRecordAccessor(t *testing.T) {
	// Auto-detect: 0 before any record is read, then the learned count.
	r := NewReader(strings.NewReader("a,b\n"))
	if got := r.FieldsPerRecord(); got != 0 {
		t.Fatalf("FieldsPerRecord() before read = %d, want 0", got)
	}
	if _, err := r.Next(); err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got := r.FieldsPerRecord(); got != 2 {
		t.Fatalf("FieldsPerRecord() after read = %d, want 2", got)
	}

	// Explicit positive count is reported as configured.
	r = NewReader(strings.NewReader("a,b\n"), WithFieldsPerRecord(5))
	if got := r.FieldsPerRecord(); got != 5 {
		t.Fatalf("FieldsPerRecord() = %d, want 5", got)
	}

	// Disabled mode stays negative.
	r = NewReader(strings.NewReader("a,b\n"), WithFieldsPerRecord(-1))
	if got := r.FieldsPerRecord(); got != -1 {
		t.Fatalf("FieldsPerRecord() = %d, want -1", got)
	}
}

func TestReadFieldsPerRecordLazyQuotes(t *testing.T) {
	r := NewReader(strings.NewReader("a\"b,c\n1,2,3\n"), WithLazyQuotes(), WithFieldsPerRecord(2))

	rec, err := r.Next()
	if err != nil {
		t.Fatalf("Next(0): %v", err)
	}
	if got := rec.Copy(); !reflect.DeepEqual(got, []string{`a"b`, "c"}) {
		t.Fatalf("Next(0) = %q, want %q", got, []string{`a"b`, "c"})
	}

	rec, err = r.Next()
	if !errors.Is(err, ErrFieldCount) {
		t.Fatalf("Next(1) error = %v, want ErrFieldCount", err)
	}
	if got := rec.Copy(); !reflect.DeepEqual(got, []string{"1", "2", "3"}) {
		t.Fatalf("Next(1) = %q, want %q", got, []string{"1", "2", "3"})
	}

	if _, err := r.Next(); err != io.EOF {
		t.Fatalf("Next(2) = %v, want io.EOF", err)
	}
}

// chunkedReader returns data in fixed-size chunks, forcing the Reader to
// refill and compact its buffer between records.
type chunkedReader struct {
	data []byte
	off  int
	step int
}

func (r *chunkedReader) Read(p []byte) (int, error) {
	if r.off >= len(r.data) {
		return 0, io.EOF
	}
	n := len(p)
	if n > r.step {
		n = r.step
	}
	if n > len(r.data)-r.off {
		n = len(r.data) - r.off
	}
	copy(p, r.data[r.off:r.off+n])
	r.off += n
	return n, nil
}

func TestReaderTrimsOversizedBuffer(t *testing.T) {
	// A single record larger than the buffer forces it to grow; once that
	// record is consumed the buffer must be trimmed back to its default size
	// so memory does not stay pinned at the peak record size.
	huge := strings.Repeat("x", 256<<10)
	var input bytes.Buffer
	input.WriteString(`"`)
	input.WriteString(huge)
	input.WriteString(`"` + "\n")
	for i := 0; i < 100_000; i++ {
		input.WriteString("a,b\n")
	}
	r := NewReader(&chunkedReader{data: input.Bytes(), step: 4096}, WithFieldsPerRecord(-1))

	rec, err := r.Next()
	if err != nil {
		t.Fatalf("Next(huge): %v", err)
	}
	if rec.Len() != 1 || len(rec.ValueAt(0)) != len(huge) {
		t.Fatalf("huge record = Len %d, field %d bytes; want 1/%d", rec.Len(), len(rec.ValueAt(0)), len(huge))
	}
	if cap(r.buf) < len(huge) {
		t.Fatalf("buffer did not grow to fit record: cap=%d, want >= %d", cap(r.buf), len(huge))
	}

	n := 0
	for {
		rec, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next(small %d): %v", n, err)
		}
		if got := rec.Copy(); !reflect.DeepEqual(got, []string{"a", "b"}) {
			t.Fatalf("row %d = %q, want [a b]", n, got)
		}
		n++
	}
	if n != 100_000 {
		t.Fatalf("read %d small rows, want 100000", n)
	}
	if cap(r.buf) > defaultBufSize {
		t.Fatalf("buffer not trimmed: cap=%d, want <= %d", cap(r.buf), defaultBufSize)
	}
}

func TestReaderMaxBuffer(t *testing.T) {
	// A record that fits within the cap parses normally; a record that would
	// need the buffer to grow past the cap fails with a sticky ErrRecordTooLarge
	// instead of allocating unbounded memory.
	input := strings.Repeat("a", 40) + "\n" + strings.Repeat("b", 70) + "\n" + strings.Repeat("c", 30) + "\n"
	r := NewReader(&chunkedReader{data: []byte(input), step: 8}, WithMaxBuffer(64), WithFieldsPerRecord(-1))

	rec, err := r.Next()
	if err != nil {
		t.Fatalf("Next(0): %v", err)
	}
	if got := rec.Copy(); len(got[0]) != 40 {
		t.Fatalf("Next(0) field length = %d, want 40", len(got[0]))
	}
	if _, err := r.Next(); !errors.Is(err, ErrRecordTooLarge) {
		t.Fatalf("Next(1) error = %v, want ErrRecordTooLarge", err)
	}
	if err := r.Error(); !errors.Is(err, ErrRecordTooLarge) {
		t.Fatalf("Error() = %v, want ErrRecordTooLarge", err)
	}
	if _, err := r.Next(); !errors.Is(err, ErrRecordTooLarge) {
		t.Fatalf("Next(2) error = %v, want sticky ErrRecordTooLarge", err)
	}
}

func TestReaderLiveMemoryBounded(t *testing.T) {
	// After a large record is consumed, live heap must drop back to a small
	// baseline even though the buffer peaked at the record's size.
	huge := strings.Repeat("x", 256<<10)
	var input bytes.Buffer
	input.WriteString(`"`)
	input.WriteString(huge)
	input.WriteString(`"` + "\n")
	for i := 0; i < 100_000; i++ {
		input.WriteString("a,b\n")
	}
	r := NewReader(&chunkedReader{data: input.Bytes(), step: 4096}, WithFieldsPerRecord(-1))

	for {
		if _, err := r.Next(); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("Next: %v", err)
		}
	}

	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	if m.HeapAlloc > 4<<20 {
		t.Fatalf("live heap after stream = %d bytes, want <= 4 MiB (buffer not trimmed)", m.HeapAlloc)
	}
}

func readAllStdlibLazy(input string) ([][]string, error) {
	r := csv.NewReader(strings.NewReader(input))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	var rows [][]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return rows, nil
		}
		if err != nil {
			return rows, err
		}
		rows = append(rows, rec)
	}
}

func FuzzReaderConformanceLazy(f *testing.F) {
	seeds := []string{
		"",
		"a,b,c\n",
		"\"a,b\",c\n",
		"a,\"b\nc\",d\n",
		"a\"b,c\n",
		"\"a\"b,c\n",
		"\"a\"b\"c\"\n",
		"a,\"b\n",
		"\"a\n",
		"\"a\"\r\n",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		wantRows, wantErr := readAllStdlibLazy(string(data))
		gotRows, gotErr := readAllZerocsv(t, string(data), WithLazyQuotes(), WithFieldsPerRecord(-1))

		if wantErr != nil {
			if gotErr == nil {
				t.Fatalf("stdlib errored (%v) but zerocsv succeeded: input=%q got=%q", wantErr, data, gotRows)
			}
			return
		}
		if gotErr != nil {
			t.Fatalf("stdlib succeeded but zerocsv errored (%v): input=%q want=%q", gotErr, data, wantRows)
		}
		if !reflect.DeepEqual(wantRows, gotRows) {
			t.Fatalf("mismatch: input=%q\n std=%q\n got=%q", data, wantRows, gotRows)
		}
	})
}

func FuzzReaderConformance(f *testing.F) {
	seeds := []string{
		"",
		"a,b,c\n",
		"a,b,c",
		"\"a,b\",c\n",
		"a,\"b\nc\",d\n",
		"\"a\"\"b\",c\n",
		"a,b\nc,d\n",
		"a,b\r\nc,d\r\n",
		",,\n",
		"\"\"\n",
		"\"a,b\"\n\"c\"\n",
		"x",
		"\n",
		"\r\n",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		wantRows, wantErr := readAllStdlib(string(data))
		gotRows, gotErr := readAllZerocsv(t, string(data), WithFieldsPerRecord(-1))

		if wantErr != nil {
			if gotErr == nil {
				t.Fatalf("stdlib errored (%v) but zerocsv succeeded: input=%q got=%q", wantErr, data, gotRows)
			}
			return
		}
		if gotErr != nil {
			t.Fatalf("stdlib succeeded but zerocsv errored (%v): input=%q want=%q", gotErr, data, wantRows)
		}
		if !reflect.DeepEqual(wantRows, gotRows) {
			t.Fatalf("mismatch: input=%q\n std=%q\n got=%q", data, wantRows, gotRows)
		}
	})
}

// fprResult is the outcome of reading with field-count checking enabled: one
// row (and its nil-or-ErrFieldCount error) per record, plus the first fatal
// error, if any.
type fprResult struct {
	rows [][]string
	errs []error
	err  error
}

func readAllStdlibFPR(input string) fprResult {
	r := csv.NewReader(strings.NewReader(input)) // FieldsPerRecord defaults to 0
	var res fprResult
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return res
		}
		if err != nil {
			if !errors.Is(err, csv.ErrFieldCount) {
				res.err = err
				return res
			}
			res.errs = append(res.errs, ErrFieldCount)
		} else {
			res.errs = append(res.errs, nil)
		}
		res.rows = append(res.rows, rec)
	}
}

func readAllZerocsvFPR(input string) fprResult {
	r := NewReader(strings.NewReader(input)) // WithFieldsPerRecord defaults to 0
	var res fprResult
	for {
		rec, err := r.Next()
		if err == io.EOF {
			return res
		}
		if err != nil {
			if !errors.Is(err, ErrFieldCount) {
				res.err = err
				return res
			}
			res.errs = append(res.errs, ErrFieldCount)
		} else {
			res.errs = append(res.errs, nil)
		}
		res.rows = append(res.rows, rec.Copy())
	}
}

// FuzzReaderConformanceFieldCount verifies that the auto-detect field-count
// behavior (default 0) matches encoding/csv record for record, including
// which records report ErrFieldCount.
func FuzzReaderConformanceFieldCount(f *testing.F) {
	seeds := []string{
		"",
		"a,b,c\n",
		"a,b,c\n1,2\n",
		"a,b,c\n1,2\nx,y,z\n",
		"a,b\nc\n",
		"\n\nx,y\n\n1,2\n\n",
		"a,b,c\nd,e\nf,g,h\n",
		"\"a,b\",c\n1,2\n",
		"a,b,c\r\n1,2\r\n",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		want := readAllStdlibFPR(string(data))
		got := readAllZerocsvFPR(string(data))

		if want.err != nil {
			if got.err == nil {
				t.Fatalf("stdlib errored (%v) but zerocsv succeeded: input=%q rows=%q", want.err, data, got.rows)
			}
			return
		}
		if got.err != nil {
			t.Fatalf("stdlib succeeded but zerocsv errored (%v): input=%q rows=%q", got.err, data, want.rows)
		}
		if !reflect.DeepEqual(want.rows, got.rows) {
			t.Fatalf("row mismatch: input=%q\n std=%q\n got=%q", data, want.rows, got.rows)
		}
		if !reflect.DeepEqual(want.errs, got.errs) {
			t.Fatalf("error mismatch: input=%q\n std=%v\n got=%v", data, want.errs, got.errs)
		}
	})
}
