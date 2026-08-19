package zerocsv

import (
	"bytes"
	"errors"
	"unicode/utf8"
)

// ErrInvalidDelim is returned when a delimiter that would corrupt the CSV
// structure is configured on a Writer or Reader.
var ErrInvalidDelim = errors.New("zerocsv: invalid field delimiter")

// Option configures a Writer or Reader at construction time.
type Option func(*options)

// options holds configuration shared by the Writer and Reader. Fields that
// only apply to one of the two are simply ignored by the other.
type options struct {
	delimiter       []byte
	useCRLF         bool
	lazyQuotes      bool
	fieldsPerRecord int
	maxBuf          int
}

func defaultOptions() *options {
	return &options{delimiter: []byte{','}}
}

// WithDelimiter sets a single-byte field delimiter, for example ',' for CSV,
// '\t' for TSV, or ';' for semicolon-separated values.
// The NUL byte, '"', '\r', and '\n' are rejected.
func WithDelimiter(d byte) Option {
	return func(o *options) {
		o.delimiter = []byte{d}
	}
}

// WithDelimiterRune sets the field delimiter to a single Unicode rune, supporting
// ASCII and multi-byte UTF-8 runes (such as '§', '·', '€', '▲').
// The NUL rune, '"', '\r', '\n', and invalid runes are rejected.
func WithDelimiterRune(r rune) Option {
	var buf [utf8.UTFMax]byte
	n := utf8.EncodeRune(buf[:], r)
	return WithDelimiterBytes(buf[:n])
}

// WithDelimiterString sets the field delimiter to a string, supporting single-character
// and multi-character string delimiters (such as "||", "~|~", "###").
// Delimiters containing NUL, '"', '\r', or '\n' are rejected.
func WithDelimiterString(s string) Option {
	return WithDelimiterBytes([]byte(s))
}

// WithDelimiterBytes sets the field delimiter to a raw byte slice.
// Delimiters containing NUL, '"', '\r', or '\n' are rejected.
func WithDelimiterBytes(b []byte) Option {
	d := append([]byte(nil), b...)
	return func(o *options) {
		o.delimiter = d
	}
}

// WithCRLF makes the Writer end each record with "\r\n" instead of "\n".
func WithCRLF() Option {
	return func(o *options) {
		o.useCRLF = true
	}
}

// WithLazyQuotes makes the Reader tolerate malformed quoting: a bare '"' in an
// unquoted field, or a non-doubled '"' in a quoted field, is treated as a
// literal character instead of returning a parse error.
func WithLazyQuotes() Option {
	return func(o *options) {
		o.lazyQuotes = true
	}
}

// WithFieldsPerRecord sets the expected number of fields per record, applying
// to both the Reader and the Writer.
//
// If n is positive, Read, ReadAll and Write require every record to have
// exactly n fields and return ErrFieldCount otherwise. If n is 0, the count is
// taken from the first record and enforced on all subsequent ones, like
// encoding/csv's default. If n is negative, no check is made and records may
// have a variable number of fields. Blank lines read by the Reader never take
// part in the check.
//
// Like encoding/csv, ErrFieldCount is non-fatal: the mismatched record is
// still returned (Reader) or written (Writer), and reading or writing may
// continue.
func WithFieldsPerRecord(n int) Option {
	return func(o *options) {
		o.fieldsPerRecord = n
	}
}

// WithMaxBuffer caps the Reader's internal buffer at n bytes. A record larger
// than n cannot be parsed in memory, so Read returns ErrRecordTooLarge rather
// than letting the buffer grow without bound. A non-positive n means no limit
// (the default).
func WithMaxBuffer(n int) Option {
	return func(o *options) {
		o.maxBuf = n
	}
}

// validDelim reports whether d is a usable delimiter.
func validDelim(d []byte) bool {
	if len(d) == 0 {
		return false
	}
	if bytes.ContainsAny(d, "\x00\"\r\n") {
		return false
	}
	if !utf8.Valid(d) {
		return false
	}
	return true
}
