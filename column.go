package zerocsv

import (
	"math"
	"unsafe"
)

// ColumnKind identifies the payload type stored in a Column.
type ColumnKind uint8

const (
	ColumnKindString ColumnKind = iota
	ColumnKindBytes
	ColumnKindInt
	ColumnKindUint
	ColumnKindFloat
	ColumnKindFloat32
	ColumnKindBool
	ColumnKindValuer
)

const (
	columnString  = ColumnKindString
	columnBytes   = ColumnKindBytes
	columnInt     = ColumnKindInt
	columnUint    = ColumnKindUint
	columnFloat   = ColumnKindFloat
	columnFloat32 = ColumnKindFloat32
	columnBool    = ColumnKindBool
	columnValuer  = ColumnKindValuer
)

// Column is a tagged, value-typed CSV field. Pass it to Write by value;
// building and writing columns performs no heap allocation for all constructors.
type Column struct {
	valuer FieldValuer
	s      string
	n      uint64
	kind   ColumnKind
}

// Kind returns the ColumnKind of c.
func (c Column) Kind() ColumnKind {
	return c.kind
}

// ColumnString returns a Column containing s.
func ColumnString(s string) Column { return Column{kind: columnString, s: s} }

// ColumnBytes returns a Column containing b. The slice is written as-is, with
// no copy and no heap allocation.
func ColumnBytes(b []byte) Column {
	return Column{
		kind: columnBytes,
		s:    bytesToString(b),
	}
}

// bytesToString returns a string pointing to the same backing array as b
// without heap allocation.
func bytesToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return *(*string)(unsafe.Pointer(&b))
}

// ColumnInt returns a Column containing v.
func ColumnInt(v int) Column { return Column{kind: columnInt, n: uint64(int64(v))} }

// ColumnInt8 returns a Column containing v.
func ColumnInt8(v int8) Column { return Column{kind: columnInt, n: uint64(int64(v))} }

// ColumnInt16 returns a Column containing v.
func ColumnInt16(v int16) Column { return Column{kind: columnInt, n: uint64(int64(v))} }

// ColumnInt32 returns a Column containing v.
func ColumnInt32(v int32) Column { return Column{kind: columnInt, n: uint64(int64(v))} }

// ColumnInt64 returns a Column containing v.
func ColumnInt64(v int64) Column { return Column{kind: columnInt, n: uint64(v)} }

// ColumnUint returns a Column containing v.
func ColumnUint(v uint) Column { return Column{kind: columnUint, n: uint64(v)} }

// ColumnUint8 returns a Column containing v.
func ColumnUint8(v uint8) Column { return Column{kind: columnUint, n: uint64(v)} }

// ColumnUint16 returns a Column containing v.
func ColumnUint16(v uint16) Column { return Column{kind: columnUint, n: uint64(v)} }

// ColumnUint32 returns a Column containing v.
func ColumnUint32(v uint32) Column { return Column{kind: columnUint, n: uint64(v)} }

// ColumnUint64 returns a Column containing v.
func ColumnUint64(v uint64) Column { return Column{kind: columnUint, n: v} }

// ColumnUintptr returns a Column containing v.
func ColumnUintptr(v uintptr) Column { return Column{kind: columnUint, n: uint64(v)} }

// ColumnFloat32 returns a Column containing v.
func ColumnFloat32(v float32) Column {
	return Column{kind: columnFloat32, n: math.Float64bits(float64(v))}
}

// ColumnFloat64 returns a Column containing v.
func ColumnFloat64(v float64) Column { return Column{kind: columnFloat, n: math.Float64bits(v)} }

// ColumnBool returns a Column containing v, written as "true" or "false".
func ColumnBool(v bool) Column {
	var n uint64
	if v {
		n = 1
	}
	return Column{kind: columnBool, n: n}
}

// ColumnValuer returns a Column containing v, which appends its CSV
// representation with zero heap allocations.
func ColumnValuer(v FieldValuer) Column { return Column{kind: columnValuer, valuer: v} }
