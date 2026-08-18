package zerocsv

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"unicode"
	"unicode/utf8"
)

// ErrEmptyRecord is returned by Write when no columns are provided.
var ErrEmptyRecord = errors.New("zerocsv: empty record")

// initialScratchSize pre-sizes the numeric/time scratch buffer. It covers the
// longest common fields — int64/uint64 min/max (20 bytes) and RFC3339 time
// (25 bytes) — so the first Write performs no allocation. Floats formatted
// with 'f' can exceed this and grow the buffer lazily.
const initialScratchSize = 32

// bufioDefaultSize is bufio.NewWriter's default buffer size. A *bufio.Writer
// at least this large is reused directly instead of being wrapped again.
const bufioDefaultSize = 4096

// Writer writes CSV records with zero allocations per write.
//
// The bufio.Writer and the numeric scratch buffer are allocated once in
// NewWriter and reused for the lifetime of the Writer. Write/WriteAll perform
// no heap allocations on the hot path as long as the caller reuses a []Column
// slice (e.g. Write(row...)) rather than passing freshly constructed variadic
// args.
type Writer struct {
	w               *bufio.Writer
	err             error
	comma           byte
	useCRLF         bool
	fieldsPerRecord int // expected fields per record; see WithFieldsPerRecord
	scratch         []byte
}

// NewWriter returns a Writer that writes CSV records to w, applying opts. If
// w is already a *bufio.Writer with a buffer at least as large as the default
// (4096 bytes), it is reused directly rather than being wrapped again.
func NewWriter(w io.Writer, opts ...Option) *Writer {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	wr := &Writer{
		comma:           o.delimiter,
		useCRLF:         o.useCRLF,
		fieldsPerRecord: o.fieldsPerRecord,
		scratch:         make([]byte, 0, initialScratchSize),
	}
	if bw, ok := w.(*bufio.Writer); ok && bw.Size() >= bufioDefaultSize {
		wr.w = bw
	} else {
		wr.w = bufio.NewWriter(w)
	}
	if !validDelim(o.delimiter) {
		wr.err = ErrInvalidDelim
	}
	return wr
}

// Write writes cols as a single CSV record to the underlying writer. It
// returns ErrEmptyRecord if cols is empty, or the first error encountered
// while writing the record.
//
// If a field count is in effect (see WithFieldsPerRecord) and cols has a
// different number of fields, Write returns ErrFieldCount. Like encoding/csv,
// the error is non-fatal: the record is still written and writing may
// continue.
func (w *Writer) Write(cols ...Column) error {
	if w.err != nil {
		return w.err
	}
	if len(cols) == 0 {
		return ErrEmptyRecord
	}
	countErr := w.checkFieldCount(len(cols))
	for i := range cols {
		if i > 0 {
			w.writeByte(w.comma)
		}
		w.writeColumn(&cols[i])
	}
	if w.useCRLF {
		w.writeByte('\r')
	}
	w.writeByte('\n')
	if w.err != nil {
		return w.err
	}
	return countErr
}

// checkFieldCount enforces the expected fields per record. A positive count
// requires every record to match it and returns ErrFieldCount otherwise; a
// zero count is learned from the first record written. Empty records never
// reach this check because Write rejects them beforehand.
func (w *Writer) checkFieldCount(n int) error {
	if w.fieldsPerRecord > 0 {
		if n != w.fieldsPerRecord {
			return ErrFieldCount
		}
	} else if w.fieldsPerRecord == 0 {
		w.fieldsPerRecord = n
	}
	return nil
}

// WriteAll writes each row of rows as a CSV record to the underlying writer,
// flushes any buffered data, and returns the first error encountered, if any.
func (w *Writer) WriteAll(rows [][]Column) error {
	for _, row := range rows {
		if err := w.Write(row...); err != nil {
			return err
		}
	}
	return w.Flush()
}

// Flush writes any buffered data to the underlying writer and returns the
// first error encountered during Write, WriteAll or Flush, if any.
func (w *Writer) Flush() error {
	if w.err != nil {
		return w.err
	}
	w.err = w.w.Flush()
	return w.err
}

// Error returns the first error encountered during Write, WriteAll or Flush,
// or nil if none has occurred.
func (w *Writer) Error() error {
	return w.err
}

// FieldsPerRecord returns the expected number of fields per record, as
// configured with WithFieldsPerRecord. With auto-detection (the default) it
// is 0 until the first record is written, after which it is the field count
// learned from that record; a negative value means no check is in effect.
func (w *Writer) FieldsPerRecord() int {
	return w.fieldsPerRecord
}

func (w *Writer) writeColumn(c *Column) {
	switch c.kind {
	case columnString:
		w.writeField(c.s)
	case columnBytes:
		w.writeFieldBytes(c.bs)
	case columnInt:
		w.scratch = strconv.AppendInt(w.scratch[:0], int64(c.n), 10)
		w.writeBytes(w.scratch)
	case columnUint:
		w.scratch = strconv.AppendUint(w.scratch[:0], c.n, 10)
		w.writeBytes(w.scratch)
	case columnFloat:
		w.scratch = strconv.AppendFloat(w.scratch[:0], math.Float64frombits(c.n), 'f', -1, 64)
		w.writeBytes(w.scratch)
	case columnFloat32:
		w.scratch = strconv.AppendFloat(w.scratch[:0], math.Float64frombits(c.n), 'f', -1, 32)
		w.writeBytes(w.scratch)
	case columnBool:
		if c.n != 0 {
			w.writeString("true")
		} else {
			w.writeString("false")
		}
	case columnTime:
		w.scratch = c.t.AppendFormat(w.scratch[:0], c.s)
		w.writeFieldBytes(w.scratch)
	case columnAny:
		if c.v == nil {
			w.writeField("")
		} else {
			w.writeField(fmt.Sprint(c.v))
		}
	}
}

func (w *Writer) writeField(field string) {
	if w.err != nil {
		return
	}
	if !fieldNeedsQuotes(field, w.comma) {
		w.writeString(field)
		return
	}
	w.writeByte('"')
	for i := 0; i < len(field); i++ {
		c := field[i]
		if c == '"' {
			w.writeByte('"')
		}
		w.writeByte(c)
	}
	w.writeByte('"')
}

func (w *Writer) writeFieldBytes(field []byte) {
	if w.err != nil {
		return
	}
	if !bytesNeedsQuotes(field, w.comma) {
		w.writeBytes(field)
		return
	}
	w.writeByte('"')
	for _, c := range field {
		if c == '"' {
			w.writeByte('"')
		}
		w.writeByte(c)
	}
	w.writeByte('"')
}

func fieldNeedsQuotes(field string, comma byte) bool {
	if field == "" {
		return false
	}
	if field == `\.` {
		return true
	}
	for i := 0; i < len(field); i++ {
		switch field[i] {
		case comma, '"', '\r', '\n':
			return true
		}
	}
	r, _ := utf8.DecodeRuneInString(field)
	return unicode.IsSpace(r)
}

func bytesNeedsQuotes(field []byte, comma byte) bool {
	if len(field) == 0 {
		return false
	}
	if len(field) == 2 && field[0] == '\\' && field[1] == '.' {
		return true
	}
	for _, c := range field {
		switch c {
		case comma, '"', '\r', '\n':
			return true
		}
	}
	r, _ := utf8.DecodeRune(field)
	return unicode.IsSpace(r)
}

func (w *Writer) writeString(s string) {
	if w.err != nil {
		return
	}
	_, w.err = w.w.WriteString(s)
}

func (w *Writer) writeBytes(p []byte) {
	if w.err != nil {
		return
	}
	_, w.err = w.w.Write(p)
}

func (w *Writer) writeByte(c byte) {
	if w.err != nil {
		return
	}
	w.err = w.w.WriteByte(c)
}
