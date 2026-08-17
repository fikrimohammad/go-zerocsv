package zerocsv

import "errors"

// ErrInvalidDelim is returned when a delimiter that would corrupt the CSV
// structure is configured on a Writer or Reader.
var ErrInvalidDelim = errors.New("zerocsv: invalid field delimiter")

// Option configures a Writer or Reader at construction time.
type Option func(*options)

// options holds configuration shared by the Writer and Reader. Fields that
// only apply to one of the two are simply ignored by the other.
type options struct {
	delimiter  byte
	useCRLF    bool
	lazyQuotes bool
}

func defaultOptions() *options {
	return &options{delimiter: ','}
}

// WithDelimiter sets the field delimiter, for example ',' for CSV, '\t' for
// TSV, or ';' for semicolon-separated values. The NUL byte, '"', '\r' and
// '\n' are rejected: an invalid delimiter marks a Writer or Reader as failed,
// and Next, Write, WriteAll, Flush and Error report the error.
func WithDelimiter(d byte) Option {
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

func validDelim(c byte) bool {
	return c != 0 && c != '"' && c != '\r' && c != '\n'
}
