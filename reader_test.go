package zerocsv

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func readAllZerocsv(t *testing.T, input string, opts ...Option) ([][]string, error) {
	t.Helper()
	r := NewReader(strings.NewReader(input), opts...)
	var rows [][]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return rows, nil
		}
		if err != nil && !errors.Is(err, ErrFieldCount) {
			return rows, err
		}
		rows = append(rows, rec.Strings())
		if errors.Is(err, ErrFieldCount) {
			return rows, err
		}
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
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		row := make([]string, rec.Len())
		for i := 0; i < rec.Len(); i++ {
			row[i] = rec.String(i)
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
	rec, err := r.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if rec.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", rec.Len())
	}
	if got := rec.String(0); got != "a" {
		t.Fatalf("String(0) = %q, want %q", got, "a")
	}
	if got := rec.String(2); got != "c" {
		t.Fatalf("String(2) = %q, want %q", got, "c")
	}
}

func TestRecordStringsOwnsData(t *testing.T) {
	r := NewReader(strings.NewReader("a,b,c\n1,2,3\n"))
	rec, err := r.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	got := rec.Strings()
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Strings() = %q, want %q", got, want)
	}
	// Advancing the reader must not disturb the copied strings.
	if _, err := r.Read(); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Strings() after advancing reader = %q, want %q", got, want)
	}
}

func TestReaderInvalidDelimiter(t *testing.T) {
	for _, d := range []byte{0, '"', '\r', '\n', 0x80, 0xff, 'é'} {
		r := NewReader(strings.NewReader("a,b\n"), WithDelimiter(d))
		if err := r.Error(); err == nil {
			t.Fatalf("Error() = nil for delimiter %d, want ErrInvalidDelim", d)
		}
		if _, err := r.Read(); err == nil {
			t.Fatalf("Read() = nil error for delimiter %d, want ErrInvalidDelim", d)
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
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		got = append(got, rec.Strings())
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
	if _, err := r.Read(); err != nil {
		t.Fatalf("warmup Read: %v", err)
	}
	var (
		s1, s2 string
		i      int
		f      float64
		b      bool
	)
	_ = s1
	_ = s2
	_ = i
	_ = f
	_ = b
	if got := testing.AllocsPerRun(50, func() {
		rec, err := r.Read()
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		_ = rec.Len()
	}); got != 0 {
		t.Fatalf("Read allocated %v times per run, want 0", got)
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
	if _, err := r.Read(); err != io.ErrNoProgress {
		t.Fatalf("Read() error = %v, want io.ErrNoProgress", err)
	}
}

func TestReaderErrorNilAfterCleanEOF(t *testing.T) {
	r := NewReader(strings.NewReader("a,b\n"))
	for {
		_, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read: %v", err)
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

	rec, err := r.Read()
	if err != nil {
		t.Fatalf("Read(0): %v", err)
	}
	if got := rec.Strings(); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("Read(0) = %q, want %q", got, []string{"a", "b", "c"})
	}

	// The mismatched record is returned alongside ErrFieldCount, and reading
	// continues with the next record, like encoding/csv.
	rec, err = r.Read()
	if !errors.Is(err, ErrFieldCount) {
		t.Fatalf("Read(1) error = %v, want ErrFieldCount", err)
	}
	if got := rec.Strings(); !reflect.DeepEqual(got, []string{"1", "2"}) {
		t.Fatalf("Read(1) = %q, want %q", got, []string{"1", "2"})
	}

	rec, err = r.Read()
	if err != nil {
		t.Fatalf("Read(2): %v", err)
	}
	if got := rec.Strings(); !reflect.DeepEqual(got, []string{"x", "y", "z"}) {
		t.Fatalf("Read(2) = %q, want %q", got, []string{"x", "y", "z"})
	}

	if _, err := r.Read(); err != io.EOF {
		t.Fatalf("Read(3) = %v, want io.EOF", err)
	}
}

func TestReadFieldsPerRecordAutoDetect(t *testing.T) {
	// The default (0) and an explicit WithFieldsPerRecord(0) both learn the
	// field count from the first record and enforce it from then on.
	for _, opts := range [][]Option{nil, {WithFieldsPerRecord(0)}} {
		r := NewReader(strings.NewReader("a,b\n1,2,3\n"), opts...)

		rec, err := r.Read()
		if err != nil {
			t.Fatalf("Read(0): %v", err)
		}
		if got := rec.Strings(); !reflect.DeepEqual(got, []string{"a", "b"}) {
			t.Fatalf("Read(0) = %q, want %q", got, []string{"a", "b"})
		}

		rec, err = r.Read()
		if !errors.Is(err, ErrFieldCount) {
			t.Fatalf("Read(1) error = %v, want ErrFieldCount", err)
		}
		if got := rec.Strings(); !reflect.DeepEqual(got, []string{"1", "2", "3"}) {
			t.Fatalf("Read(1) = %q, want %q", got, []string{"1", "2", "3"})
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

	rec, err := r.Read()
	if !errors.Is(err, ErrFieldCount) {
		t.Fatalf("Read(0) error = %v, want ErrFieldCount", err)
	}
	if got := rec.Strings(); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("Read(0) = %q, want %q", got, []string{"a", "b", "c"})
	}

	rec, err = r.Read()
	if err != nil {
		t.Fatalf("Read(1): %v", err)
	}
	if got := rec.Strings(); !reflect.DeepEqual(got, []string{"1", "2"}) {
		t.Fatalf("Read(1) = %q, want %q", got, []string{"1", "2"})
	}
}

func TestReadFieldsPerRecordBlankLines(t *testing.T) {
	// Blank lines are skipped and never set or violate the count.
	r := NewReader(strings.NewReader("\n\n\nx,y\n\n1,2\n\n"))

	rec, err := r.Read()
	if err != nil {
		t.Fatalf("Read(0): %v", err)
	}
	if got := rec.Strings(); !reflect.DeepEqual(got, []string{"x", "y"}) {
		t.Fatalf("Read(0) = %q, want %q", got, []string{"x", "y"})
	}

	rec, err = r.Read()
	if err != nil {
		t.Fatalf("Read(1): %v", err)
	}
	if got := rec.Strings(); !reflect.DeepEqual(got, []string{"1", "2"}) {
		t.Fatalf("Read(1) = %q, want %q", got, []string{"1", "2"})
	}

	if _, err := r.Read(); err != io.EOF {
		t.Fatalf("Read(2) = %v, want io.EOF", err)
	}
}

func TestReadFieldsPerRecordBlankLineDoesNotLearnCount(t *testing.T) {
	// A file whose first record is mismatched after leading blank lines still
	// reports the mismatch: the count is learned from the first non-blank row.
	r := NewReader(strings.NewReader("\na,b,c\n1,2\n"), WithFieldsPerRecord(0))

	rec, err := r.Read()
	if err != nil {
		t.Fatalf("Read(0): %v", err)
	}
	if got := rec.Strings(); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("Read(0) = %q, want %q", got, []string{"a", "b", "c"})
	}

	rec, err = r.Read()
	if !errors.Is(err, ErrFieldCount) {
		t.Fatalf("Read(1) error = %v, want ErrFieldCount", err)
	}
	if got := rec.Strings(); !reflect.DeepEqual(got, []string{"1", "2"}) {
		t.Fatalf("Read(1) = %q, want %q", got, []string{"1", "2"})
	}
}

func TestReadFieldsPerRecordChunked(t *testing.T) {
	// Records split across buffer boundaries count fields, not lines.
	input := "\"multi\nline\",x\ny,z\n"
	r := NewReader(&oneByteReader{data: []byte(input)}, WithFieldsPerRecord(2))

	rec, err := r.Read()
	if err != nil {
		t.Fatalf("Read(0): %v", err)
	}
	want0 := []string{"multi\nline", "x"}
	if got := rec.Strings(); !reflect.DeepEqual(got, want0) {
		t.Fatalf("Read(0) = %q, want %q", got, want0)
	}

	rec, err = r.Read()
	if err != nil {
		t.Fatalf("Read(1): %v", err)
	}
	want1 := []string{"y", "z"}
	if got := rec.Strings(); !reflect.DeepEqual(got, want1) {
		t.Fatalf("Read(1) = %q, want %q", got, want1)
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

	if _, err := r.Read(); err != nil {
		t.Fatalf("Read(0): %v", err)
	}
	if _, err := r.Read(); !errors.Is(err, ErrFieldCount) {
		t.Fatalf("Read(1) error = %v, want ErrFieldCount", err)
	}
	if err := r.Error(); err != nil {
		t.Fatalf("Error() = %v after ErrFieldCount, want nil", err)
	}
	if _, err := r.Read(); err != io.EOF {
		t.Fatalf("Read(2) = %v, want io.EOF", err)
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
	if _, err := r.Read(); err != nil {
		t.Fatalf("Read: %v", err)
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

	rec, err := r.Read()
	if err != nil {
		t.Fatalf("Read(0): %v", err)
	}
	if got := rec.Strings(); !reflect.DeepEqual(got, []string{`a"b`, "c"}) {
		t.Fatalf("Read(0) = %q, want %q", got, []string{`a"b`, "c"})
	}

	rec, err = r.Read()
	if !errors.Is(err, ErrFieldCount) {
		t.Fatalf("Read(1) error = %v, want ErrFieldCount", err)
	}
	if got := rec.Strings(); !reflect.DeepEqual(got, []string{"1", "2", "3"}) {
		t.Fatalf("Read(1) = %q, want %q", got, []string{"1", "2", "3"})
	}

	if _, err := r.Read(); err != io.EOF {
		t.Fatalf("Read(2) = %v, want io.EOF", err)
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

func TestReaderGrowsBufferForOversizedRecord(t *testing.T) {
	// A single record larger than the default buffer forces it to grow; once that
	// record is consumed the high-watermark buffer is retained so subsequent records
	// parse with zero allocations without memory churn.
	huge := strings.Repeat("x", 256<<10)
	var input bytes.Buffer
	input.WriteString(`"`)
	input.WriteString(huge)
	input.WriteString(`"` + "\n")
	for i := 0; i < 10_000; i++ {
		input.WriteString("a,b\n")
	}
	r := NewReader(&chunkedReader{data: input.Bytes(), step: 4096}, WithFieldsPerRecord(-1))

	rec, err := r.Read()
	if err != nil {
		t.Fatalf("Read(huge): %v", err)
	}
	if rec.Len() != 1 || len(rec.String(0)) != len(huge) {
		t.Fatalf("huge record = Len %d, field %d bytes; want 1/%d", rec.Len(), len(rec.String(0)), len(huge))
	}
	if cap(r.buf) < len(huge) {
		t.Fatalf("buffer did not grow to fit record: cap=%d, want >= %d", cap(r.buf), len(huge))
	}

	n := 0
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read(small %d): %v", n, err)
		}
		if got := rec.Strings(); !reflect.DeepEqual(got, []string{"a", "b"}) {
			t.Fatalf("row %d = %q, want [a b]", n, got)
		}
		n++
	}
	if n != 10_000 {
		t.Fatalf("read %d small rows, want 10000", n)
	}
}

func TestReaderMaxBufferSize(t *testing.T) {
	// A record that fits within the cap parses normally; a record that would
	// need the buffer to grow past the cap fails with a sticky ErrRecordTooLarge
	// instead of allocating unbounded memory.
	input := strings.Repeat("a", 40) + "\n" + strings.Repeat("b", 70) + "\n" + strings.Repeat("c", 30) + "\n"
	r := NewReader(&chunkedReader{data: []byte(input), step: 8}, WithMaxBufferSize(64), WithFieldsPerRecord(-1))

	rec, err := r.Read()
	if err != nil {
		t.Fatalf("Read(0): %v", err)
	}
	if got := rec.Strings(); len(got[0]) != 40 {
		t.Fatalf("Read(0) field length = %d, want 40", len(got[0]))
	}
	if _, err := r.Read(); !errors.Is(err, ErrRecordTooLarge) {
		t.Fatalf("Read(1) error = %v, want ErrRecordTooLarge", err)
	}
	if err := r.Error(); !errors.Is(err, ErrRecordTooLarge) {
		t.Fatalf("Error() = %v, want ErrRecordTooLarge", err)
	}
	if _, err := r.Read(); !errors.Is(err, ErrRecordTooLarge) {
		t.Fatalf("Read(2) error = %v, want sticky ErrRecordTooLarge", err)
	}
}

func TestReaderWithBufferSize(t *testing.T) {
	// Custom initial buffer size is respected.
	r := NewReader(strings.NewReader("a,b,c\n"), WithBufferSize(64<<10))
	if _, err := r.Read(); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if cap(r.buf) != 64<<10 {
		t.Fatalf("cap(r.buf) = %d, want %d", cap(r.buf), 64<<10)
	}

	// BufferSize capped at maxBuf if maxBuf is smaller.
	r2 := NewReader(strings.NewReader("a,b,c\n"), WithBufferSize(64<<10), WithMaxBufferSize(16<<10))
	if _, err := r2.Read(); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if cap(r2.buf) != 16<<10 {
		t.Fatalf("cap(r2.buf) = %d, want %d", cap(r2.buf), 16<<10)
	}

	// Non-positive buffer size defaults to DefaultBufferSize (4096).
	r3 := NewReader(strings.NewReader("a,b,c\n"), WithBufferSize(-1))
	if _, err := r3.Read(); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if cap(r3.buf) != DefaultBufferSize {
		t.Fatalf("cap(r3.buf) = %d, want %d", cap(r3.buf), DefaultBufferSize)
	}
}

func TestReaderPreallocatedFields(t *testing.T) {
	// When fieldsPerRecord > 0, fields and fieldSpans are pre-allocated with matching capacity.
	r := NewReader(strings.NewReader("1,2,3,4,5,6\n"), WithFieldsPerRecord(6))
	if cap(r.fields) != 6 {
		t.Fatalf("cap(r.fields) = %d, want 6", cap(r.fields))
	}
	if cap(r.fieldSpans) != 6 {
		t.Fatalf("cap(r.fieldSpans) = %d, want 6", cap(r.fieldSpans))
	}
	rec, err := r.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if rec.Len() != 6 {
		t.Fatalf("rec.Len() = %d, want 6", rec.Len())
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
		rec, err := r.Read()
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
		res.rows = append(res.rows, rec.Strings())
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

func TestReaderUTF8AndEmojiContent(t *testing.T) {
	input := "id,name,text,emoji\n" +
		"1,Alice,Café & résumé,🚀🔥\n" +
		"2,Bob,\"Tokyo, 東京\",🇯🇵\n" +
		"3,Charlie,مرحبا بالعالم,🎉\n" +
		"4,David,\"Multi-line\n🌟 Sparkle\",✨\n"

	r := NewReader(strings.NewReader(input))

	var (
		id    int
		name  string
		text  string
		emoji string
	)

	// Header
	rec, err := r.Read()
	if err != nil {
		t.Fatalf("Read header: %v", err)
	}
	if want := []string{"id", "name", "text", "emoji"}; !reflect.DeepEqual(rec.Strings(), want) {
		t.Fatalf("header got %v, want %v", rec.Strings(), want)
	}

	// Row 1
	rec, err = r.Read()
	if err != nil {
		t.Fatalf("Read row 1: %v", err)
	}
	if err := rec.Scan(&id, &name, &text, &emoji); err != nil {
		t.Fatalf("Scan row 1: %v", err)
	}
	if id != 1 || name != "Alice" || text != "Café & résumé" || emoji != "🚀🔥" {
		t.Fatalf("row 1: got (%d, %q, %q, %q)", id, name, text, emoji)
	}

	// Row 2
	rec, err = r.Read()
	if err != nil {
		t.Fatalf("Read row 2: %v", err)
	}
	if err := rec.Scan(&id, &name, &text, &emoji); err != nil {
		t.Fatalf("Scan row 2: %v", err)
	}
	if id != 2 || name != "Bob" || text != "Tokyo, 東京" || emoji != "🇯🇵" {
		t.Fatalf("row 2: got (%d, %q, %q, %q)", id, name, text, emoji)
	}

	// Row 3
	rec, err = r.Read()
	if err != nil {
		t.Fatalf("Read row 3: %v", err)
	}
	if err := rec.Scan(&id, &name, &text, &emoji); err != nil {
		t.Fatalf("Scan row 3: %v", err)
	}
	if id != 3 || name != "Charlie" || text != "مرحبا بالعالم" || emoji != "🎉" {
		t.Fatalf("row 3: got (%d, %q, %q, %q)", id, name, text, emoji)
	}

	// Row 4 (Multiline UTF-8)
	rec, err = r.Read()
	if err != nil {
		t.Fatalf("Read row 4: %v", err)
	}
	if err := rec.Scan(&id, &name, &text, &emoji); err != nil {
		t.Fatalf("Scan row 4: %v", err)
	}
	if id != 4 || name != "David" || text != "Multi-line\n🌟 Sparkle" || emoji != "✨" {
		t.Fatalf("row 4: got (%d, %q, %q, %q)", id, name, text, emoji)
	}
}
