package zerocsv

import "errors"

// Sentinel errors returned by Reader, Writer, and Record operations.
var (
	// ErrInvalidDelim is returned when a delimiter that would corrupt the CSV
	// structure is configured on a Writer or Reader.
	ErrInvalidDelim = errors.New("zerocsv: invalid field delimiter")

	// ErrEmptyRecord is returned when attempting to write an empty record or scan a record with no fields.
	ErrEmptyRecord = errors.New("zerocsv: empty record")

	// ErrRecordTooLarge is returned by Read or Write when a record is larger than the
	// maximum buffer size configured with WithMaxBufferSize and therefore cannot be
	// parsed or written.
	ErrRecordTooLarge = errors.New("zerocsv: record larger than the maximum buffer size")

	// ErrBareQuote is returned when a bare '"' appears in a non-quoted field.
	ErrBareQuote = errors.New("bare \" in non-quoted field")

	// ErrQuote is returned for an extraneous or missing '"' in a quoted field.
	ErrQuote = errors.New("extraneous or missing \" in quoted-field")

	// ErrFieldCount is returned by Read or Write when a record's field count does
	// not match the expected number of fields (see WithFieldsPerRecord). It is
	// non-fatal, like encoding/csv: the record is still returned or written and
	// reading or writing can continue.
	ErrFieldCount = errors.New("wrong number of fields")

	// ErrNilDestination is returned by Record.Scan when a nil destination argument is provided.
	ErrNilDestination = errors.New("cannot scan into nil destination")

	// ErrNilDestinationPointer is returned by Record.Scan when a typed pointer destination is nil.
	ErrNilDestinationPointer = errors.New("destination pointer is nil")
)
