package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	zerocsv "github.com/fikrimohammad/go-zerocsv"
)

func main() {
	basic()
	mixed()
	writeAll()
	timeColumn()
	customDelimiter()
	crlf()
	zeroAllocation()
	readBasic()
	readTyped()
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
		zerocsv.ColumnTime(time.Unix(0, 0).UTC(), time.RFC3339),
		zerocsv.ColumnTime(time.Now(), "2006-01-02"),
	)
	_ = w.Flush()
}

func customDelimiter() {
	fmt.Println("--- TSV delimiter ---")
	w := zerocsv.NewWriter(os.Stdout, zerocsv.WithDelimiter('\t'))

	_ = w.Write(zerocsv.ColumnString("a"), zerocsv.ColumnInt(1))
	_ = w.Flush()
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
		rec, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		// Fields are zero-copy views into the reader's buffer: convert them
		// to strings (which copies) if they must outlive the next Next.
		fmt.Printf("record: %d fields\n", rec.Len())
		for i := 0; i < rec.Len(); i++ {
			fmt.Printf("  %d: %q\n", i, rec.ValueAt(i))
		}
	}
}

func readTyped() {
	fmt.Println("--- read typed ---")
	input := "id,name,score,active\n1,alice,3.14,true\n"
	r := zerocsv.NewReader(strings.NewReader(input))
	_, _ = r.Next() // skip header
	rec, err := r.Next()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	id, _ := strconv.ParseInt(string(rec.ValueAt(0)), 10, 64)
	score, _ := strconv.ParseFloat(string(rec.ValueAt(2)), 64)
	active, _ := strconv.ParseBool(string(rec.ValueAt(3)))
	fmt.Printf("id=%d name=%s score=%f active=%t\n", id, rec.ValueAt(1), score, active)
}
