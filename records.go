package zerocsv

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
)

// FieldScanner is implemented by custom types that can scan their value directly
// from a raw CSV field byte slice.
type FieldScanner interface {
	ScanCSV(field []byte) error
}

// Record is a single CSV record parsed by Reader.
//
// A Record obtained from Read() provides safe, encapsulated access to the
// parsed fields. To achieve zero heap allocations on the hot path, field data
// is accessed via Scan (for typed values or reusable []byte buffers), String,
// Bytes, or Strings.
type Record struct {
	err     error
	isFirst bool
	fields  [][]byte
}

// Len returns the number of fields in the record.
func (rec Record) Len() int {
	return len(rec.fields)
}

// String returns the field at index idx as a string. It panics if idx is out
// of range [0, Len()).
func (rec Record) String(idx int) string {
	return string(rec.fields[idx])
}

// Bytes copies the field at index idx into dst and returns the resulting slice.
// If cap(dst) is large enough, Bytes performs zero heap allocations. It panics
// if idx is out of range [0, Len()).
func (rec Record) Bytes(idx int, dst []byte) []byte {
	return append(dst[:0], rec.fields[idx]...)
}

// Strings returns the record's fields as a new slice of strings.
func (rec Record) Strings() []string {
	out := make([]string, len(rec.fields))
	for i, f := range rec.fields {
		out[i] = string(f)
	}
	return out
}

// IsFirst reports whether this record is the first non-blank record read from
// the stream, useful for header detection.
func (rec Record) IsFirst() bool {
	return rec.isFirst
}

// Error returns the non-fatal error associated with this record (e.g.
// ErrFieldCount), or nil if the record had no errors.
func (rec Record) Error() error {
	return rec.err
}

// clone returns an owned copy of the record where all field byte slices are
// duplicated, ensuring the record remains valid independently of the reader.
func (rec Record) clone() Record {
	owned := make([][]byte, len(rec.fields))
	for i, f := range rec.fields {
		owned[i] = append([]byte(nil), f...)
	}
	return Record{
		err:     rec.err,
		isFirst: rec.isFirst,
		fields:  owned,
	}
}

// Scan parses the record's fields into the destination pointers, one per field,
// in order.
//
// Supported destination types:
//   - *string: copies field as a string
//   - *[]byte: copies field into caller's slice capacity (0 allocs if capacity suffices)
//   - *bool: parses boolean ("true", "false", "1", "0", ...) in-place (0 allocs)
//   - *int, *int8, *int16, *int32, *int64: parses integer in-place (0 allocs)
//   - *uint, *uint8, *uint16, *uint32, *uint64, *uintptr: parses unsigned integer in-place (0 allocs)
//   - *float32, *float64: parses float in-place (0 allocs)
//   - FieldScanner: delegates parsing to custom ScanCSV method (0 allocs)
//
// Scan returns an error if the number of destinations does not match Len(), if
// any destination pointer is nil, or if parsing fails.
func (rec Record) Scan(dst ...any) error {
	if len(rec.fields) == 0 && rec.err == nil {
		return errors.New("zerocsv: scan: no current record")
	}
	if len(dst) != len(rec.fields) {
		return fmt.Errorf("zerocsv: scan: got %d destinations, want %d fields", len(dst), len(rec.fields))
	}
	for i, d := range dst {
		if err := scanField(d, rec.fields[i]); err != nil {
			return fmt.Errorf("zerocsv: scan field %d: %w", i, err)
		}
	}
	return nil
}

func scanField(dst any, field []byte) error {
	if dst == nil {
		return errors.New("cannot scan into nil destination")
	}
	switch p := dst.(type) {
	case *string:
		if p == nil {
			return errors.New("destination pointer is nil")
		}
		*p = string(field)
	case *[]byte:
		if p == nil {
			return errors.New("destination pointer is nil")
		}
		*p = append((*p)[:0], field...)
	case *bool:
		if p == nil {
			return errors.New("destination pointer is nil")
		}
		v, err := strconv.ParseBool(string(field))
		if err != nil {
			return err
		}
		*p = v
	case *int:
		if p == nil {
			return errors.New("destination pointer is nil")
		}
		v, err := strconv.ParseInt(string(field), 10, 0)
		if err != nil {
			return err
		}
		*p = int(v)
	case *int8:
		if p == nil {
			return errors.New("destination pointer is nil")
		}
		v, err := strconv.ParseInt(string(field), 10, 8)
		if err != nil {
			return err
		}
		*p = int8(v)
	case *int16:
		if p == nil {
			return errors.New("destination pointer is nil")
		}
		v, err := strconv.ParseInt(string(field), 10, 16)
		if err != nil {
			return err
		}
		*p = int16(v)
	case *int32:
		if p == nil {
			return errors.New("destination pointer is nil")
		}
		v, err := strconv.ParseInt(string(field), 10, 32)
		if err != nil {
			return err
		}
		*p = int32(v)
	case *int64:
		if p == nil {
			return errors.New("destination pointer is nil")
		}
		v, err := strconv.ParseInt(string(field), 10, 64)
		if err != nil {
			return err
		}
		*p = v
	case *uint:
		if p == nil {
			return errors.New("destination pointer is nil")
		}
		v, err := strconv.ParseUint(string(field), 10, 0)
		if err != nil {
			return err
		}
		*p = uint(v)
	case *uint8:
		if p == nil {
			return errors.New("destination pointer is nil")
		}
		v, err := strconv.ParseUint(string(field), 10, 8)
		if err != nil {
			return err
		}
		*p = uint8(v)
	case *uint16:
		if p == nil {
			return errors.New("destination pointer is nil")
		}
		v, err := strconv.ParseUint(string(field), 10, 16)
		if err != nil {
			return err
		}
		*p = uint16(v)
	case *uint32:
		if p == nil {
			return errors.New("destination pointer is nil")
		}
		v, err := strconv.ParseUint(string(field), 10, 32)
		if err != nil {
			return err
		}
		*p = uint32(v)
	case *uint64:
		if p == nil {
			return errors.New("destination pointer is nil")
		}
		v, err := strconv.ParseUint(string(field), 10, 64)
		if err != nil {
			return err
		}
		*p = v
	case *uintptr:
		if p == nil {
			return errors.New("destination pointer is nil")
		}
		v, err := strconv.ParseUint(string(field), 10, 0)
		if err != nil {
			return err
		}
		*p = uintptr(v)
	case *float32:
		if p == nil {
			return errors.New("destination pointer is nil")
		}
		v, err := strconv.ParseFloat(string(field), 32)
		if err != nil {
			return err
		}
		*p = float32(v)
	case *float64:
		if p == nil {
			return errors.New("destination pointer is nil")
		}
		v, err := strconv.ParseFloat(string(field), 64)
		if err != nil {
			return err
		}
		*p = v
	case FieldScanner:
		if p == nil || (reflect.ValueOf(p).Kind() == reflect.Pointer && reflect.ValueOf(p).IsNil()) {
			return errors.New("destination pointer is nil")
		}
		return p.ScanCSV(field)
	default:
		return fmt.Errorf("cannot scan into %T", dst)
	}
	return nil
}
