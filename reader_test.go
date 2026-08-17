package zerocsv

import (
	"encoding/csv"
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
		rec, err := r.Next()
		if err == io.EOF {
			return rows, nil
		}
		if err != nil {
			return rows, err
		}
		row := make([]string, rec.Len())
		for i := 0; i < rec.Len(); i++ {
			row[i] = string(rec.ValueAt(i)) // string() copies: owned
		}
		rows = append(rows, row)
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
	rows, err := readAllZerocsv(t, "a,,c\n,,\n\"\",x\n")
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
		rows, err := readAllZerocsv(t, c.input)
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
		gotRows, gotErr := readAllZerocsv(t, string(data), WithLazyQuotes())

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
		gotRows, gotErr := readAllZerocsv(t, string(data))

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
