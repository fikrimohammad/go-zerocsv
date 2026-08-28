package zerocsv

// DefaultDelimiter is the standard comma delimiter for CSV files.
const DefaultDelimiter = ','

// DefaultBufferSize is the initial capacity of the Reader's reusable buffer (4 KB).
const DefaultBufferSize = 4096

// maxDelimByte is the maximum single-byte ASCII code point allowed as a delimiter.
const maxDelimByte = 0x7f

// Option configures a Writer or Reader at construction time.
type Option func(*options)

// options holds configuration shared by the Writer and Reader. Fields that
// only apply to one of the two are simply ignored by the other.
type options struct {
	delimiter       byte
	useCRLF         bool
	lazyQuotes      bool
	fieldsPerRecord int
	maxBuf          int
	bufSize         int
}

func defaultOptions() *options {
	return &options{
		delimiter: DefaultDelimiter,
		bufSize:   DefaultBufferSize,
	}
}

// WithBufferSize sets the initial size of the Writer or Reader's buffer in
// bytes. A non-positive n uses DefaultBufferSize (4096 bytes). For a Reader,
// if WithMaxBufferSize is also configured and n exceeds maxBuf, the initial
// buffer size is capped at maxBuf.
func WithBufferSize(n int) Option {
	return func(o *options) {
		if n > 0 {
			o.bufSize = n
		} else {
			o.bufSize = DefaultBufferSize
		}
	}
}

// WithDelimiter sets the field delimiter, for example ',' for CSV, '\t' for
// TSV, or ';' for semicolon-separated values. Only single ASCII bytes are
// supported. The NUL byte, '"', '\r', '\n' and any byte above '\x7f' are
// rejected: an invalid delimiter marks a Writer or Reader as failed, and Read,
// ReadAll, Write, WriteAll, Flush and Error report the error.
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

// WithMaxBufferSize caps the Writer or Reader's internal buffer at n bytes.
// For a Reader, a record larger than n cannot be parsed in memory, so Read
// returns ErrRecordTooLarge rather than letting the buffer grow without bound.
// For a Writer, the internal buffer allocated for buffered I/O will not exceed
// n bytes. A non-positive n means no limit (the default).
func WithMaxBufferSize(n int) Option {
	return func(o *options) {
		o.maxBuf = n
	}
}

// validDelim reports whether c is a usable delimiter. Delimiters above 0x7f
// are rejected: the parser scans single bytes, so a multi-byte UTF-8 delimiter
// could never match, and a raw non-ASCII byte would silently corrupt the
// parsing of any multibyte text.
func validDelim(c byte) bool {
	return c != 0 && c != '"' && c != '\r' && c != '\n' && c <= maxDelimByte
}
