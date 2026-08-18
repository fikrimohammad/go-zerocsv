package main

import (
	"fmt"
	"io"
	"os"
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

	var dateBuf [32]byte
	_ = w.Write(
		zerocsv.ColumnBytes(time.Unix(0, 0).UTC().AppendFormat(dateBuf[:0], time.RFC3339)),
		zerocsv.ColumnString(time.Now().Format("2006-01-02")),
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
