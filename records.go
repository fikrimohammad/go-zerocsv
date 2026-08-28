package zerocsv

import (
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

// IsFirst reports whether this is the first data record produced by the Reader.
func (rec Record) IsFirst() bool {
	return rec.isFirst
}

// Error returns the record-level error (for example ErrFieldCount), or nil
// if the record was read without error.
func (rec Record) Error() error {
	return rec.err
}

// String returns field i as a string. It returns "" if i is out of range.
// Note that constructing the return string allocates heap memory. For zero
// allocations on the hot path, use Scan with typed values or Bytes with a
// reusable slice.
func (rec Record) String(i int) string {
	if i < 0 || i >= len(rec.fields) {
		return ""
	}
	return string(rec.fields[i])
}

// Bytes copies field i into dst[:len(field)] and returns the subslice,
// growing dst only if cap(dst) is too small. If i is out of range it returns
// dst[:0]. When dst has sufficient capacity, Bytes performs zero heap
// allocations.
func (rec Record) Bytes(i int, dst []byte) []byte {
	if i < 0 || i >= len(rec.fields) {
		return dst[:0]
	}
	f := rec.fields[i]
	if cap(dst) < len(f) {
		dst = make([]byte, len(f))
	} else {
		dst = dst[:len(f)]
	}
	copy(dst, f)
	return dst
}

// Strings returns all fields as a slice of newly allocated strings.
func (rec Record) Strings() []string {
	if len(rec.fields) == 0 {
		return nil
	}
	out := make([]string, len(rec.fields))
	for i, f := range rec.fields {
		out[i] = string(f)
	}
	return out
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

// Scan unpacks the record's fields into the destination pointers dst in order.
// The number of destinations must match Len() exactly.
//
// Supported destination types:
//   - *string: allocates a string (note: use *[]byte or primitives for 0 allocs)
//   - *[]byte: copies bytes into existing slice capacity without allocation
//   - *bool: parses bool ("true", "false", "1", "0", etc.) in-place (0 allocs)
//   - *int, *int8, *int16, *int32, *int64: parses integer in-place (0 allocs)
//   - *uint, *uint8, *uint16, *uint32, *uint64, *uintptr: parses unsigned integer in-place (0 allocs)
//   - *float32, *float64: parses float in-place (0 allocs)
//   - FieldScanner: delegates parsing to custom ScanCSV method (0 allocs)
//
// Scan returns an error if the number of destinations does not match Len(), if
// any destination pointer is nil, or if parsing fails.
func (rec Record) Scan(dst ...any) error {
	if len(rec.fields) == 0 && rec.err == nil {
		return ErrEmptyRecord
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
		return ErrNilDestination
	}
	switch p := dst.(type) {
	case *string:
		if p == nil {
			return ErrNilDestinationPointer
		}
		*p = string(field)
	case *[]byte:
		if p == nil {
			return ErrNilDestinationPointer
		}
		*p = append((*p)[:0], field...)
	case *bool:
		if p == nil {
			return ErrNilDestinationPointer
		}
		v, err := strconv.ParseBool(string(field))
		if err != nil {
			return err
		}
		*p = v
	case *int:
		if p == nil {
			return ErrNilDestinationPointer
		}
		v, err := strconv.ParseInt(string(field), 10, 0)
		if err != nil {
			return err
		}
		*p = int(v)
	case *int8:
		if p == nil {
			return ErrNilDestinationPointer
		}
		v, err := strconv.ParseInt(string(field), 10, 8)
		if err != nil {
			return err
		}
		*p = int8(v)
	case *int16:
		if p == nil {
			return ErrNilDestinationPointer
		}
		v, err := strconv.ParseInt(string(field), 10, 16)
		if err != nil {
			return err
		}
		*p = int16(v)
	case *int32:
		if p == nil {
			return ErrNilDestinationPointer
		}
		v, err := strconv.ParseInt(string(field), 10, 32)
		if err != nil {
			return err
		}
		*p = int32(v)
	case *int64:
		if p == nil {
			return ErrNilDestinationPointer
		}
		v, err := strconv.ParseInt(string(field), 10, 64)
		if err != nil {
			return err
		}
		*p = v
	case *uint:
		if p == nil {
			return ErrNilDestinationPointer
		}
		v, err := strconv.ParseUint(string(field), 10, 0)
		if err != nil {
			return err
		}
		*p = uint(v)
	case *uint8:
		if p == nil {
			return ErrNilDestinationPointer
		}
		v, err := strconv.ParseUint(string(field), 10, 8)
		if err != nil {
			return err
		}
		*p = uint8(v)
	case *uint16:
		if p == nil {
			return ErrNilDestinationPointer
		}
		v, err := strconv.ParseUint(string(field), 10, 16)
		if err != nil {
			return err
		}
		*p = uint16(v)
	case *uint32:
		if p == nil {
			return ErrNilDestinationPointer
		}
		v, err := strconv.ParseUint(string(field), 10, 32)
		if err != nil {
			return err
		}
		*p = uint32(v)
	case *uint64:
		if p == nil {
			return ErrNilDestinationPointer
		}
		v, err := strconv.ParseUint(string(field), 10, 64)
		if err != nil {
			return err
		}
		*p = v
	case *uintptr:
		if p == nil {
			return ErrNilDestinationPointer
		}
		v, err := strconv.ParseUint(string(field), 10, 0)
		if err != nil {
			return err
		}
		*p = uintptr(v)
	case *float32:
		if p == nil {
			return ErrNilDestinationPointer
		}
		v, err := strconv.ParseFloat(string(field), 32)
		if err != nil {
			return err
		}
		*p = float32(v)
	case *float64:
		if p == nil {
			return ErrNilDestinationPointer
		}
		v, err := strconv.ParseFloat(string(field), 64)
		if err != nil {
			return err
		}
		*p = v
	case FieldScanner:
		if p == nil || (reflect.ValueOf(p).Kind() == reflect.Pointer && reflect.ValueOf(p).IsNil()) {
			return ErrNilDestinationPointer
		}
		return p.ScanCSV(field)
	default:
		return fmt.Errorf("cannot scan into %T", dst)
	}
	return nil
}
