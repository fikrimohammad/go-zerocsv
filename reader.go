package zerocsv

import (
	"errors"
	"io"
)

// Reader reads CSV records with zero allocations per record.
//
// The input buffer and the field slice are allocated once in NewReader and
// reused for the lifetime of the Reader. The fields of a Record returned by
// Next are zero-copy views into that buffer, so they are valid only until the
// next call to Next.
type Reader struct {
	r          io.Reader
	delimiter  byte
	lazyQuotes bool

	buf   []byte // reusable input buffer
	start int    // first unprocessed byte
	end   int    // number of valid bytes in buf
	eof   bool

	rec    Record   // reused record wrapper
	fields [][]byte // reused field views
}

// Record is a parsed CSV record. The Reader reuses a single Record and
// overwrites it on every call to Next, so a Record is valid only until the
// Reader is advanced again. Its fields are zero-copy views into the Reader's
// internal buffer; copy one with slices.Clone, or convert it to a string
// (which copies), if it must outlive the read loop.
type Record struct {
	fields [][]byte
}

// Len returns the number of fields in the record.
func (r *Record) Len() int {
	return len(r.fields)
}

// ValueAt returns the field at index idx as a zero-copy view into the
// Reader's internal buffer. The slice is valid only until the next call to
// Next on the Reader that produced the Record. Convert it to a string to get
// an owned copy, since string(v) always copies.
func (r *Record) ValueAt(idx int) []byte {
	return r.fields[idx]
}

// NewReader returns a Reader that parses CSV records from r, applying opts.
func NewReader(r io.Reader, opts ...Option) *Reader {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return &Reader{
		r:          r,
		delimiter:  o.delimiter,
		lazyQuotes: o.lazyQuotes,
	}
}

// Next parses and returns the next record, or io.EOF when no records remain.
// It returns ErrBareQuote or ErrQuote for malformed quoting, unless
// WithLazyQuotes is in effect, and propagates any error from the underlying
// reader.
//
// The returned Record is reused by the Reader and overwritten by each call to
// Next. Its fields are zero-copy views into the Reader's internal buffer and
// are valid only until the next call to Next, like encoding/csv's ReuseRecord
// mode. Copy a field with slices.Clone, or convert it to a string, if it must
// outlive the read loop.
func (r *Reader) Next() (*Record, error) {
	r.fields = r.fields[:0]
	for {
		recEnd, nlLen, complete, err := r.scan()
		if err != nil {
			return nil, err
		}
		if complete {
			if recEnd == r.start {
				// Blank line: skip, like the standard library does.
				r.start = recEnd + nlLen
				continue
			}
			r.rec.fields = r.decode(r.start, recEnd)
			r.start = recEnd + nlLen
			return &r.rec, nil
		}
		if r.eof {
			if r.start == r.end {
				return nil, io.EOF
			}
			// Final record without a trailing newline.
			r.rec.fields = r.decode(r.start, r.end)
			r.start = r.end
			return &r.rec, nil
		}
		if err := r.fill(); err != nil {
			return nil, err
		}
	}
}

// scan locates the end of the next record starting at r.start. It returns the
// index of the line terminator, the terminator length (1 or 2 for CRLF), and
// whether a complete record was found. A (false, nil) result means the record
// spans the current buffer and more data is needed.
func (r *Reader) scan() (recEnd, nlLen int, complete bool, err error) {
	pos := r.start
	fieldStart := r.start
	inQuotes := false

	for pos < r.end {
		c := r.buf[pos]

		if inQuotes {
			if c != '"' {
				if c == '\r' && pos+1 == r.end && r.eof {
					// Trailing '\r' at EOF is a stripped terminator, even
					// inside quotes, matching the standard library.
					if r.lazyQuotes {
						return pos, 1, true, nil
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
					pos++
					continue
				case '\r':
					if pos+2 < r.end {
						if r.buf[pos+2] == '\n' {
							inQuotes = false // closing quote before CRLF
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
			pos++
			fieldStart = pos
		case '\n':
			return pos, 1, true, nil
		case '\r':
			if pos+1 < r.end {
				if r.buf[pos+1] == '\n' {
					return pos, 2, true, nil
				}
				pos++ // bare '\r' is a regular byte, not a terminator
				continue
			}
			// '\r' is the last buffered byte; a following '\n' may arrive in
			// the next chunk. At EOF it is a stripped terminator.
			if r.eof {
				return pos, 1, true, nil
			}
			return 0, 0, false, nil // need more data
		default:
			pos++
		}
	}

	if inQuotes {
		if r.eof {
			if r.lazyQuotes {
				return 0, 0, false, nil // unterminated quote tolerated
			}
			return 0, 0, false, ErrQuote
		}
		return 0, 0, false, nil
	}
	return 0, 0, false, nil
}

// decode parses fields of the complete record buf[start:recEnd] into r.fields,
// decoding quoted fields in place. The quote handling mirrors scan so that
// field boundaries are identical.
func (r *Reader) decode(start, recEnd int) [][]byte {
	r.fields = r.fields[:0]
	pos := start
	fieldStart := start
	inQuotes := false
	closedAt := -1 // position of the current field's closing quote, if closed

	for pos < recEnd {
		c := r.buf[pos]
		if inQuotes {
			if c != '"' {
				pos++
				continue
			}
			if pos+1 < recEnd && r.buf[pos+1] == '"' {
				pos += 2 // escaped quote
				continue
			}
			if pos+1 < recEnd {
				switch r.buf[pos+1] {
				case r.delimiter, '\n':
					inQuotes = false
					closedAt = pos
					pos++
					continue
				case '\r':
					if pos+2 < recEnd && r.buf[pos+2] == '\n' {
						inQuotes = false
						closedAt = pos
						pos++
						continue
					}
					if r.lazyQuotes {
						pos++
						continue
					}
					inQuotes = false
					closedAt = pos
					pos++
					continue
				default:
					if r.lazyQuotes {
						pos++
						continue
					}
					inQuotes = false
					closedAt = pos
					pos++
					continue
				}
			}
			inQuotes = false // closing quote at end of record
			closedAt = pos
			pos++
			continue
		}
		switch c {
		case '"':
			if pos == fieldStart {
				inQuotes = true
			}
			pos++
		case r.delimiter:
			r.appendField(fieldStart, pos, closedAt == pos-1)
			pos++
			fieldStart = pos
			closedAt = -1
		default:
			pos++
		}
	}
	r.appendField(fieldStart, recEnd, closedAt == recEnd-1)
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

// fill moves unprocessed bytes to the front of the buffer and reads more data.
func (r *Reader) fill() error {
	if r.start > 0 {
		n := copy(r.buf, r.buf[r.start:r.end])
		r.end = n
		r.start = 0
	}
	if r.end == len(r.buf) {
		if len(r.buf) == 0 {
			r.buf = make([]byte, 4096)
		} else {
			r.buf = append(r.buf, make([]byte, len(r.buf))...)
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
	return nil
}

// ErrBareQuote is returned when a bare '"' appears in a non-quoted field.
var ErrBareQuote = errors.New("bare \" in non-quoted field")

// ErrQuote is returned for an extraneous or missing '"' in a quoted field.
var ErrQuote = errors.New("extraneous or missing \" in quoted-field")
