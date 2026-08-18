package zerocsv

import (
	"errors"
	"io"
)

// Reader reads CSV records with zero allocations per record.
//
// The input buffer and the field slice are allocated once in NewReader and
// reused for the lifetime of the Reader. Read returns records lazily one at
// a time with zero heap allocations, while ReadAll eagerly reads all remaining
// records into a slice of owned Records.
type Reader struct {
	r          io.Reader
	delimiter  byte
	lazyQuotes bool

	fieldsPerRecord int // expected fields per record; see WithFieldsPerRecord
	maxBuf          int // buffer size cap; see WithMaxBuffer
	recordsRead     int // records returned so far; backs IsFirst

	buf   []byte // reusable input buffer
	start int    // first unprocessed byte
	end   int    // number of valid bytes in buf
	eof   bool

	err        error       // sticky configuration error
	rec        rawRecord   // reused record wrapper
	fields     [][]byte    // reused field views
	fieldSpans []fieldSpan // field boundaries recorded by scan, consumed by decode
}

// fieldSpan describes one field of the record currently being scanned, as a
// half-open interval [start, end) into buf. closed reports whether the field
// ends with a genuine closing quote, which is excluded from the decoded value.
type fieldSpan struct {
	start  int
	end    int
	closed bool
}

// rawRecord is an unexported parsed CSV record backing Reader.next.
type rawRecord struct {
	fields [][]byte
}

func (r *rawRecord) len() int {
	return len(r.fields)
}

// NewReader returns a Reader that parses CSV records from r, applying opts.
// An invalid delimiter marks the Reader as failed; Read, ReadAll and Error
// report the error.
func NewReader(r io.Reader, opts ...Option) *Reader {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	rd := &Reader{
		r:               r,
		delimiter:       o.delimiter,
		lazyQuotes:      o.lazyQuotes,
		fieldsPerRecord: o.fieldsPerRecord,
		maxBuf:          o.maxBuf,
	}
	if !validDelim(o.delimiter) {
		rd.err = ErrInvalidDelim
	}
	return rd
}

// next parses and returns the next raw record, or io.EOF when no records remain.
func (r *Reader) next() (*rawRecord, error) {
	r.rec.fields = nil
	if r.err != nil {
		return nil, r.err
	}
	for {
		recEnd, nlLen, complete, err := r.scan()
		if err != nil {
			r.err = err
			return nil, err
		}
		if complete {
			if recEnd == r.start {
				// Blank line: skip, like the standard library does.
				r.start = recEnd + nlLen
				continue
			}
			r.rec.fields = r.decode()
			r.start = recEnd + nlLen
			r.recordsRead++
			if err := r.checkFieldCount(r.rec.len()); err != nil {
				return &r.rec, err
			}
			return &r.rec, nil
		}
		if r.eof {
			return nil, io.EOF
		}
		if err := r.fill(); err != nil {
			r.err = err
			return nil, err
		}
	}
}

// Read reads one record from r. The returned Record provides zero-allocation
// access to fields via Scan, or safe access via String, Bytes, and Strings.
//
// If the record has an unexpected number of fields (see WithFieldsPerRecord),
// Read returns the Record along with ErrFieldCount. Like encoding/csv, this
// error is non-fatal: the record is usable and subsequent calls to Read
// continue reading the stream.
//
// If no more records remain, Read returns a zero Record with io.EOF.
func (r *Reader) Read() (Record, error) {
	rec, err := r.next()
	if rec == nil {
		return Record{}, err
	}
	return Record{
		err:     err,
		isFirst: r.recordsRead == 1,
		fields:  rec.fields,
	}, err
}

// ReadAll reads all remaining records from r into a slice of Records.
// Each Record in the returned slice owns its field data and remains valid
// indefinitely. A successful call returns err == nil (io.EOF is treated as
// normal completion).
//
// If an error (such as ErrFieldCount or a parse error) is encountered,
// ReadAll stops immediately and returns the records read so far along with
// the error, matching encoding/csv.ReadAll semantics.
func (r *Reader) ReadAll() ([]Record, error) {
	if r.err != nil {
		return nil, r.err
	}
	var records []Record
	for {
		rec, err := r.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return records, nil
			}
			if errors.Is(err, ErrFieldCount) {
				records = append(records, rec.clone())
			}
			return records, err
		}
		records = append(records, rec.clone())
	}
}

// checkFieldCount enforces the expected fields per record. A positive count
// requires every record to match it and returns ErrFieldCount otherwise; a
// zero count is learned from the first record. Blank lines never reach this
// check because next skips them before decode.
func (r *Reader) checkFieldCount(n int) error {
	if r.fieldsPerRecord > 0 {
		if n != r.fieldsPerRecord {
			return ErrFieldCount
		}
	} else if r.fieldsPerRecord == 0 {
		r.fieldsPerRecord = n
	}
	return nil
}

// Error returns the first error encountered while reading, or nil if none has
// occurred. io.EOF is normal termination and is not treated as an error, so
// Error returns nil after a record stream has been read to completion. A
// Reader configured with an invalid delimiter is failed from the start.
func (r *Reader) Error() error {
	return r.err
}

// FieldsPerRecord returns the expected number of fields per record. It
// reflects the value configured with WithFieldsPerRecord: with
// auto-detection (the default) it is 0 until the first record is read, after
// which it is the field count learned from that record; a negative value
// means no check is in effect.
func (r *Reader) FieldsPerRecord() int {
	return r.fieldsPerRecord
}

// scan locates the end of the next record starting at r.start, recording the
// boundary of each field into r.fieldSpans as it goes. It returns the index
// of the line terminator, the terminator length (1 or 2 for CRLF), and
// whether a complete record was found. A (false, nil) result means the record
// spans the current buffer and more data is needed; when eof is set it means
// no data remains and Read returns io.EOF.
func (r *Reader) scan() (recEnd, nlLen int, complete bool, err error) {
	r.fieldSpans = r.fieldSpans[:0]
	pos := r.start
	fieldStart := r.start
	inQuotes := false
	closedAt := -1 // position of the current field's closing quote, if closed

	for pos < r.end {
		c := r.buf[pos]

		if inQuotes {
			if c != '"' {
				if c == '\r' && pos+1 == r.end && r.eof {
					// Trailing '\r' at EOF is a stripped terminator, even
					// inside quotes, matching the standard library.
					if r.lazyQuotes {
						return r.recordField(pos, 1, fieldStart, pos, closedAt == pos-1)
					}
					return 0, 0, false, ErrQuote
				}
				pos++
				continue
			}
			// Decide what this quote does by peeking at the next byte.
			if pos+1 < r.end {
				switch r.buf[pos+1] {
				case '"':
					pos += 2 // escaped quote
					continue
				case r.delimiter, '\n':
					inQuotes = false // closing quote
					closedAt = pos
					pos++
					continue
				case '\r':
					if pos+2 < r.end {
						if r.buf[pos+2] == '\n' {
							inQuotes = false // closing quote before CRLF
							closedAt = pos
							pos++
							continue
						}
						if r.lazyQuotes {
							pos++ // literal quote, field stays quoted
							continue
						}
						return 0, 0, false, ErrQuote
					}
					if r.eof {
						// The '\r' is the last byte of input; the stdlib
						// strips it, so the quote closes the field.
						inQuotes = false
						closedAt = pos
						pos++
						continue
					}
					return 0, 0, false, nil // need more data
				default:
					if r.lazyQuotes {
						pos++ // literal quote, field stays quoted
						continue
					}
					return 0, 0, false, ErrQuote
				}
			}
			// The quote is the last buffered byte; a following byte may still
			// arrive. At EOF it closes the field.
			if r.eof {
				inQuotes = false
				closedAt = pos
				pos++
				continue
			}
			return 0, 0, false, nil // need more data
		}

		switch c {
		case '"':
			if pos == fieldStart {
				inQuotes = true
				pos++
				continue
			}
			if r.lazyQuotes {
				pos++
				continue
			}
			return 0, 0, false, ErrBareQuote
		case r.delimiter:
			r.recordSpan(fieldStart, pos, closedAt == pos-1)
			pos++
			fieldStart = pos
			closedAt = -1
		case '\n':
			return r.recordField(pos, 1, fieldStart, pos, closedAt == pos-1)
		case '\r':
			if pos+1 < r.end {
				if r.buf[pos+1] == '\n' {
					return r.recordField(pos, 2, fieldStart, pos, closedAt == pos-1)
				}
				pos++ // bare '\r' is a regular byte, not a terminator
				continue
			}
			// '\r' is the last buffered byte; a following '\n' may arrive in
			// the next chunk. At EOF it is a stripped terminator.
			if r.eof {
				return r.recordField(pos, 1, fieldStart, pos, closedAt == pos-1)
			}
			return 0, 0, false, nil // need more data
		default:
			pos++
		}
	}

	if r.eof {
		if pos == r.start {
			// No data at all in this buffer; Read reports io.EOF.
			return 0, 0, false, nil
		}
		if inQuotes {
			if r.lazyQuotes {
				return r.recordField(r.end, 0, fieldStart, r.end, closedAt == r.end-1)
			}
			return 0, 0, false, ErrQuote
		}
		return r.recordField(r.end, 0, fieldStart, r.end, closedAt == r.end-1)
	}
	return 0, 0, false, nil
}

// recordSpan records the field buf[fieldStart:pos] into r.fieldSpans. closed
// reports whether the field ends with a genuine closing quote.
func (r *Reader) recordSpan(fieldStart, pos int, closed bool) {
	r.fieldSpans = append(r.fieldSpans, fieldSpan{start: fieldStart, end: pos, closed: closed})
}

// recordField records the final field of the record ending at recEnd with a
// terminator of length nlLen (0 for a final record without a trailing
// newline) and returns a complete record. A blank line records no fields and
// is skipped by Read.
func (r *Reader) recordField(recEnd, nlLen int, fieldStart, pos int, closed bool) (int, int, bool, error) {
	if pos != fieldStart || len(r.fieldSpans) != 0 {
		r.recordSpan(fieldStart, pos, closed)
	}
	return recEnd, nlLen, true, nil
}

// decode converts the spans recorded by scan into r.fields, decoding quoted
// fields in place.
func (r *Reader) decode() [][]byte {
	r.fields = r.fields[:0]
	for i := range r.fieldSpans {
		s := &r.fieldSpans[i]
		r.appendField(s.start, s.end, s.closed)
	}
	return r.fields
}

// appendField stores the field buf[fstart:fend] as a string view, decoding it
// in place if it is quoted. stripQuote is true when the field ends with a
// genuine closing quote (which is therefore excluded from the value).
func (r *Reader) appendField(fstart, fend int, stripQuote bool) {
	var s []byte
	switch {
	case fstart < fend && r.buf[fstart] == '"':
		src := fstart + 1
		stop := fend
		if stripQuote {
			stop--
		}
		dst := fstart
		for src < stop {
			c := r.buf[src]
			if c == '"' && src+1 < stop && r.buf[src+1] == '"' {
				src += 2 // collapsed ""
			} else if c == '\r' && src+1 < stop && r.buf[src+1] == '\n' {
				src++ // normalize \r\n to \n, like the standard library
				continue
			} else {
				src++
			}
			r.buf[dst] = c
			dst++
		}
		s = r.buf[fstart:dst]
	case fstart < fend:
		s = r.buf[fstart:fend]
	}
	r.fields = append(r.fields, s)
}

// defaultBufSize is the initial size of the reader's reusable buffer.
const defaultBufSize = 4096

// minTrimSize is the smallest buffer the reader bothers reclaiming. A buffer
// at or below this size is kept as-is even after a large record is consumed:
// reclaiming it would force the next similar-sized record to grow it again,
// churning allocations for negligible memory savings.
const minTrimSize = 256 << 10

// fill moves unprocessed bytes to the front of the buffer and reads more data.
func (r *Reader) fill() error {
	if r.start > 0 {
		tail := r.end - r.start
		if tail <= len(r.buf)/4 && len(r.buf) > minTrimSize {
			// A record much larger than the buffer was just consumed; release
			// the oversized buffer so memory does not stay pinned at the peak
			// record size. The tail fits comfortably in a fresh buffer, so
			// copy it there instead of compacting in place.
			size := defaultBufSize
			for size <= tail {
				size *= 2
			}
			if r.maxBuf > 0 && size > r.maxBuf {
				size = r.maxBuf
			}
			nb := make([]byte, size)
			copy(nb, r.buf[r.start:r.end])
			r.buf = nb
			r.end = tail
		} else {
			n := copy(r.buf, r.buf[r.start:r.end])
			r.end = n
		}
		r.start = 0
	}
	if r.end == len(r.buf) {
		if len(r.buf) == 0 {
			size := defaultBufSize
			if r.maxBuf > 0 && size > r.maxBuf {
				size = r.maxBuf
			}
			r.buf = make([]byte, size)
		} else {
			if r.maxBuf > 0 && len(r.buf) >= r.maxBuf {
				// The current record spans the whole buffer and the cap would
				// not let it grow any further; it cannot be parsed in memory.
				return ErrRecordTooLarge
			}
			size := len(r.buf) * 2
			if r.maxBuf > 0 && size > r.maxBuf {
				size = r.maxBuf
			}
			nb := make([]byte, size)
			copy(nb, r.buf)
			r.buf = nb
		}
	}
	n, err := r.r.Read(r.buf[r.end:])
	r.end += n
	if err != nil {
		if err == io.EOF {
			r.eof = true
			return nil
		}
		return err
	}
	if n == 0 {
		return io.ErrNoProgress
	}
	return nil
}

// ErrBareQuote is returned when a bare '"' appears in a non-quoted field.
var ErrBareQuote = errors.New("bare \" in non-quoted field")

// ErrRecordTooLarge is returned by Read when a record is larger than the
// maximum buffer size configured with WithMaxBuffer and therefore cannot be
// parsed in memory. Reading cannot continue past the record.
var ErrRecordTooLarge = errors.New("zerocsv: record larger than the maximum buffer size")

// ErrQuote is returned for an extraneous or missing '"' in a quoted field.
var ErrQuote = errors.New("extraneous or missing \" in quoted-field")

// ErrFieldCount is returned by Read or Write when a record's field count does
// not match the expected number of fields (see WithFieldsPerRecord). It is
// non-fatal, like encoding/csv: the record is still returned or written and
// reading or writing can continue.
var ErrFieldCount = errors.New("wrong number of fields")
