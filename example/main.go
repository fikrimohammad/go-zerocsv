package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	zerocsv "github.com/fikrimohammad/go-zerocsv"
)

// Date demonstrates a custom domain type implementing both FieldScanner and FieldValuer
// for zero-allocation CSV input and output.
type Date time.Time

// ScanCSV implements zerocsv.FieldScanner for zero-allocation CSV reading.
func (d *Date) ScanCSV(field []byte) error {
	t, err := time.Parse("2006-01-02", string(field))
	if err != nil {
		return err
	}
	*d = Date(t)
	return nil
}

// AppendCSV implements zerocsv.FieldValuer for zero-allocation CSV writing.
func (d Date) AppendCSV(dst []byte) ([]byte, error) {
	return time.Time(d).AppendFormat(dst, "2006-01-02"), nil
}

func main() {
	basic()
	mixed()
	writeAll()
	timeColumn()
	customValuerWrite()
	customDelimiter()
	multiByteDelimiter()
	crlf()
	zeroAllocation()
	readBasic()
	readTyped()
	customScannerRead()
}

func basic() {
	fmt.Println("--- basic ---")
	w := zerocsv.NewWriter(os.Stdout)

	_ = w.Write(zerocsv.ColumnString("name"), zerocsv.ColumnString("age"))
	_ = w.Write(zerocsv.ColumnString("alice"), zerocsv.ColumnInt(30))
	_ = w.Flush()
}

func mixed() {
	fmt.Println("--- mixed types ---")
	w := zerocsv.NewWriter(os.Stdout)

	_ = w.Write(
		zerocsv.ColumnInt(42),
		zerocsv.ColumnFloat64(3.14),
		zerocsv.ColumnBool(true),
		zerocsv.ColumnString("comma,inside"),
	)
	_ = w.Flush()
}

func writeAll() {
	fmt.Println("--- WriteAll ---")
	w := zerocsv.NewWriter(os.Stdout)

	_ = w.WriteAll([][]zerocsv.Column{
		{zerocsv.ColumnString("a"), zerocsv.ColumnInt(1)},
		{zerocsv.ColumnString("b"), zerocsv.ColumnInt(2)},
	})
	_ = w.Flush()
}

func timeColumn() {
	fmt.Println("--- time ---")
	w := zerocsv.NewWriter(os.Stdout)

	_ = w.Write(
		zerocsv.ColumnString(time.Unix(0, 0).UTC().Format(time.RFC3339)),
		zerocsv.ColumnString(time.Now().Format("2006-01-02")),
	)
	_ = w.Flush()
}

func customValuerWrite() {
	fmt.Println("--- custom FieldValuer ---")
	w := zerocsv.NewWriter(os.Stdout)

	d := Date(time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC))
	_ = w.Write(
		zerocsv.ColumnString("alice"),
		zerocsv.ColumnValuer(d),
	)
	_ = w.Flush()
}

func customDelimiter() {
	fmt.Println("--- TSV delimiter ---")
	w := zerocsv.NewWriter(os.Stdout, zerocsv.WithDelimiter('\t'))

	_ = w.Write(zerocsv.ColumnString("a"), zerocsv.ColumnInt(1))
	_ = w.Flush()
}

func multiByteDelimiter() {
	fmt.Println("--- multi-byte delimiter (rune & string) ---")
	var buf bytes.Buffer

	// Write with multi-character delimiter "||"
	w := zerocsv.NewWriter(&buf, zerocsv.WithDelimiterString("||"))
	_ = w.Write(zerocsv.ColumnInt(1), zerocsv.ColumnString("Alice"), zerocsv.ColumnFloat64(98.5))
	_ = w.Write(zerocsv.ColumnInt(2), zerocsv.ColumnString("Bob||Smith"), zerocsv.ColumnFloat64(87.0))
	_ = w.Flush()

	// Read back with zero allocations
	r := zerocsv.NewReader(&buf, zerocsv.WithDelimiterString("||"))
	var (
		id    int
		name  string
		score float64
	)
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		_ = rec.Scan(&id, &name, &score)
		fmt.Printf("id=%d, name=%q, score=%.1f\n", id, name, score)
	}
}

func crlf() {
	fmt.Println("--- CRLF ---")
	w := zerocsv.NewWriter(os.Stdout, zerocsv.WithCRLF())

	_ = w.Write(zerocsv.ColumnString("a"), zerocsv.ColumnString("b"))
	_ = w.Flush()
}

func zeroAllocation() {
	fmt.Println("--- zero allocation (reused row) ---")
	w := zerocsv.NewWriter(os.Stdout)

	// Reuse a single []Column across writes to keep the hot path
	// allocation-free.
	row := make([]zerocsv.Column, 3)
	for i := 0; i < 3; i++ {
		row[0] = zerocsv.ColumnInt(i)
		row[1] = zerocsv.ColumnString("x")
		row[2] = zerocsv.ColumnBool(i%2 == 0)
		_ = w.Write(row...)
	}
	_ = w.Flush()
}

func readBasic() {
	fmt.Println("--- read ---")
	input := `name,age
"alice, a.",30
bob,40
`
	r := zerocsv.NewReader(strings.NewReader(input))
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		fmt.Printf("record: %d fields\n", rec.Len())
		for i := 0; i < rec.Len(); i++ {
			fmt.Printf("  %d: %q\n", i, rec.String(i))
		}
	}
}

func readTyped() {
	fmt.Println("--- read typed ---")
	input := "id,name,score,active\n1,alice,3.14,true\n2,bob,2.5,false\n"
	r := zerocsv.NewReader(strings.NewReader(input))
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		if rec.IsFirst() {
			fmt.Printf("header: %s\n", strings.Join(rec.Strings(), ", "))
			continue
		}
		var (
			id     int
			name   string
			score  float64
			active bool
		)
		if err := rec.Scan(&id, &name, &score, &active); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		fmt.Printf("id=%d name=%s score=%.2f active=%t\n", id, name, score, active)
	}
}

func customScannerRead() {
	fmt.Println("--- custom FieldScanner ---")
	input := "id,name,birthday\n1,alice,2026-08-18\n2,bob,1990-01-01\n"
	r := zerocsv.NewReader(strings.NewReader(input))
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		if rec.IsFirst() {
			continue
		}
		var (
			id       int
			name     string
			birthday Date
		)
		if err := rec.Scan(&id, &name, &birthday); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		fmt.Printf("id=%d name=%s birthday=%s\n", id, name, time.Time(birthday).Format("Jan 02, 2006"))
	}
}
