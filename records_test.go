package zerocsv

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestReadAllAndScan(t *testing.T) {
	input := "id,name,score,active\n1,alice,3.14,true\n2,bob,2.5,false\n"
	records, err := NewReader(strings.NewReader(input)).ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("got %d records, want 3", len(records))
	}

	// First record is the header.
	if !records[0].IsFirst() {
		t.Fatal("first record: IsFirst = false, want true")
	}
	var h1, h2, h3, h4 string
	if err := records[0].Scan(&h1, &h2, &h3, &h4); err != nil {
		t.Fatalf("Scan header: %v", err)
	}
	if h1 != "id" || h2 != "name" || h3 != "score" || h4 != "active" {
		t.Fatalf("header = %q/%q/%q/%q", h1, h2, h3, h4)
	}

	// Data rows.
	for i, rec := range records[1:] {
		if rec.IsFirst() {
			t.Fatalf("data row %d: IsFirst = true, want false", i)
		}
		var id int
		var name string
		var score float64
		var active bool
		if err := rec.Scan(&id, &name, &score, &active); err != nil {
			t.Fatalf("Scan row %d: %v", i, err)
		}
		if i == 0 {
			if id != 1 || name != "alice" || score != 3.14 || active != true {
				t.Fatalf("row 1 = %d/%q/%f/%t", id, name, score, active)
			}
		} else {
			if id != 2 || name != "bob" || score != 2.5 || active != false {
				t.Fatalf("row 2 = %d/%q/%f/%t", id, name, score, active)
			}
		}
	}
}

func TestReadAllScanAllPrimitiveTypes(t *testing.T) {
	input := "s,by,bs,i,i8,i16,i32,i64,u,u8,u16,u32,u64,p,f32,f64,b,a\n" +
		"x,\"\",bytes,7,-8,300,-70000,5000000000,9,200,60000,4000000000,123,456,1.5,2.25,true,\"y\"\n"
	records, err := NewReader(strings.NewReader(input)).ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	var (
		s, by string
		bs    []byte
		i     int
		i8    int8
		i16   int16
		i32   int32
		i64   int64
		u     uint
		u8    uint8
		u16   uint16
		u32   uint32
		u64   uint64
		p     uintptr
		f32   float32
		f64   float64
		b     bool
		a     any
	)
	if err := records[1].Scan(&s, &by, &bs, &i, &i8, &i16, &i32, &i64, &u, &u8, &u16, &u32, &u64, &p, &f32, &f64, &b, &a); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if s != "x" || by != "" || string(bs) != "bytes" {
		t.Fatalf("strings: %q %q %q", s, by, bs)
	}
	if i != 7 || i8 != -8 || i16 != 300 || i32 != -70000 || i64 != 5e9 {
		t.Fatalf("ints: %d %d %d %d %d", i, i8, i16, i32, i64)
	}
	if u != 9 || u8 != 200 || u16 != 60000 || u32 != 4e9 || u64 != 123 || p != 456 {
		t.Fatalf("uints: %d %d %d %d %d %d", u, u8, u16, u32, u64, p)
	}
	if f32 != 1.5 || f64 != 2.25 {
		t.Fatalf("floats: %v %v", f32, f64)
	}
	if b != true {
		t.Fatalf("bool: %v", b)
	}
	if a != "y" {
		t.Fatalf("any: %v", a)
	}
}

func TestRecordAccessors(t *testing.T) {
	r := NewReader(strings.NewReader("alpha,beta,gamma\n"))
	rec, err := r.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if rec.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", rec.Len())
	}
	if got := rec.String(0); got != "alpha" {
		t.Fatalf("String(0) = %q, want alpha", got)
	}
	if got := rec.String(2); got != "gamma" {
		t.Fatalf("String(2) = %q, want gamma", got)
	}
	buf := make([]byte, 0, 16)
	buf = rec.Bytes(1, buf)
	if string(buf) != "beta" {
		t.Fatalf("Bytes(1) = %q, want beta", string(buf))
	}
	if want := []string{"alpha", "beta", "gamma"}; !reflect.DeepEqual(rec.Strings(), want) {
		t.Fatalf("Strings() = %q, want %q", rec.Strings(), want)
	}
}

func TestRecordScanNilPointers(t *testing.T) {
	r := NewReader(strings.NewReader("1,alice,3.14\n"))
	rec, err := r.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	var nilInt *int
	var nilStr *string
	var nilFloat *float64

	// Each nil pointer should return an error, not panic
	if err := rec.Scan(nilInt, &nilStr, &nilFloat); err == nil {
		t.Fatal("Scan with nil *int destination: want error, got nil")
	}
	if err := rec.Scan(nil, nil, nil); err == nil {
		t.Fatal("Scan with untyped nil destinations: want error, got nil")
	}
}

func TestScanDestCountMismatch(t *testing.T) {
	records, _ := NewReader(strings.NewReader("a,b\n1,2\n")).ReadAll()
	var a, b, c string
	err := records[1].Scan(&a, &b, &c)
	if err == nil {
		t.Fatal("Scan with 3 dests for 2 fields: want error")
	}
	if !strings.Contains(err.Error(), "got 3 destinations") {
		t.Fatalf("error = %v, want destination-count error", err)
	}
}

func TestScanParseErrors(t *testing.T) {
	records, _ := NewReader(strings.NewReader("1,bad,c\n")).ReadAll()
	var i int
	var f float64
	if err := records[0].Scan(&i, &f); err == nil {
		t.Fatal("Scan of bad dest count: want error")
	}
	var f2 float64
	var s string
	if err := records[0].Scan(&i, &f2, &s); err == nil {
		t.Fatal("Scan of bad float: want error")
	}
}

func TestReadAllEmptyInput(t *testing.T) {
	records, err := NewReader(strings.NewReader("")).ReadAll()
	if err != nil {
		t.Fatalf("ReadAll empty: %v", err)
	}
	if records != nil {
		t.Fatalf("ReadAll empty: got %v, want nil", records)
	}
}

func TestReadAllFatalError(t *testing.T) {
	_, err := NewReader(strings.NewReader("a\nb\"c\n")).ReadAll()
	if !errors.Is(err, ErrBareQuote) {
		t.Fatalf("ReadAll = %v, want ErrBareQuote", err)
	}
}

func TestReadAllFieldCountStopsAtError(t *testing.T) {
	input := "a,b\n1,2\n3\nx,y\n"
	records, err := NewReader(strings.NewReader(input), WithFieldsPerRecord(2)).ReadAll()
	if !errors.Is(err, ErrFieldCount) {
		t.Fatalf("ReadAll error = %v, want ErrFieldCount", err)
	}
	// encoding/csv.ReadAll stops on ErrFieldCount and returns records read up to the error
	if len(records) != 3 {
		t.Fatalf("got %d records, want 3 (including the mismatched record)", len(records))
	}
}

func TestReadFieldCountReturnsRecordAndError(t *testing.T) {
	input := "a,b\n1,2\nx\ny,z\n"
	r := NewReader(strings.NewReader(input), WithFieldsPerRecord(2))

	// Row 0
	rec0, err := r.Read()
	if err != nil {
		t.Fatalf("Read(0): %v", err)
	}
	if got := rec0.Strings(); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("rec0 = %q, want [a b]", got)
	}

	// Row 1
	rec1, err := r.Read()
	if err != nil {
		t.Fatalf("Read(1): %v", err)
	}
	if got := rec1.Strings(); !reflect.DeepEqual(got, []string{"1", "2"}) {
		t.Fatalf("rec1 = %q, want [1 2]", got)
	}

	// Row 2 (mismatched)
	rec2, err := r.Read()
	if !errors.Is(err, ErrFieldCount) {
		t.Fatalf("Read(2) error = %v, want ErrFieldCount", err)
	}
	if got := rec2.Strings(); !reflect.DeepEqual(got, []string{"x"}) {
		t.Fatalf("rec2 = %q, want [x]", got)
	}
	if !errors.Is(rec2.Error(), ErrFieldCount) {
		t.Fatalf("rec2.Error() = %v, want ErrFieldCount", rec2.Error())
	}

	// Row 3 (reading continues)
	rec3, err := r.Read()
	if err != nil {
		t.Fatalf("Read(3): %v", err)
	}
	if got := rec3.Strings(); !reflect.DeepEqual(got, []string{"y", "z"}) {
		t.Fatalf("rec3 = %q, want [y z]", got)
	}

	// EOF
	_, err = r.Read()
	if err != io.EOF {
		t.Fatalf("Read(4) past EOF = %v, want io.EOF", err)
	}
}

func TestReadAllThenReadContinues(t *testing.T) {
	r := NewReader(strings.NewReader("a\nb\nc\n"))
	rec, err := r.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if rec.String(0) != "a" {
		t.Fatalf("first record = %q, want a", rec.String(0))
	}
	rest, err := r.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(rest) != 2 {
		t.Fatalf("got %d remaining records, want 2", len(rest))
	}
	if rest[0].String(0) != "b" || rest[1].String(0) != "c" {
		t.Fatalf("remaining = %q/%q, want b/c", rest[0].String(0), rest[1].String(0))
	}
}

func TestReadAllRoundTripWithWriter(t *testing.T) {
	rows := [][]Column{
		{ColumnString("id"), ColumnString("name"), ColumnString("score"), ColumnString("active")},
		{ColumnInt(1), ColumnString("alice"), ColumnFloat64(3.14), ColumnBool(true)},
		{ColumnInt(2), ColumnString("bob"), ColumnFloat64(2.5), ColumnBool(false)},
	}
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.WriteAll(rows); err != nil {
		t.Fatalf("WriteAll: %v", err)
	}

	records, err := NewReader(bytes.NewReader(buf.Bytes())).ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("got %d records, want 3", len(records))
	}
	var id int
	var name string
	var score float64
	var active bool
	if err := records[1].Scan(&id, &name, &score, &active); err != nil {
		t.Fatalf("Scan row 1: %v", err)
	}
	if id != 1 || name != "alice" || score != 3.14 || active != true {
		t.Fatalf("row 1 = %d/%q/%f/%t", id, name, score, active)
	}
}

func FuzzRecordsConformance(f *testing.F) {
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
		records, gotErr := NewReader(strings.NewReader(string(data)), WithFieldsPerRecord(-1)).ReadAll()

		if wantErr != nil {
			if gotErr == nil {
				t.Fatalf("stdlib errored (%v) but zerocsv succeeded: input=%q", wantErr, data)
			}
			return
		}
		if gotErr != nil {
			t.Fatalf("stdlib succeeded but zerocsv errored (%v): input=%q", gotErr, data)
		}
		var gotRows [][]string
		for _, rec := range records {
			gotRows = append(gotRows, rec.Strings())
		}
		if !reflect.DeepEqual(wantRows, gotRows) {
			t.Fatalf("mismatch: input=%q\n std=%q\n got=%q", data, wantRows, gotRows)
		}
	})
}
