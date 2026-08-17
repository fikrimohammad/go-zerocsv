package zerocsv

import (
	"math"
	"time"
)

// ColumnKind identifies the payload type stored in a Column.
type ColumnKind uint8

const (
	columnString ColumnKind = iota
	columnBytes
	columnInt
	columnUint
	columnFloat
	columnFloat32
	columnBool
	columnTime
	columnAny
)

// Column is a tagged, value-typed CSV field. Pass it to Write by value; it
// holds no pointers that escape, so building and writing columns performs no
// heap allocation.
type Column struct {
	kind ColumnKind
	s    string
	bs   []byte
	n    uint64
	v    any
	t    time.Time
}

// ColumnString returns a Column containing s.
func ColumnString(s string) Column { return Column{kind: columnString, s: s} }

// ColumnBytes returns a Column containing b. The slice is written as-is, with
// no copy.
func ColumnBytes(b []byte) Column { return Column{kind: columnBytes, bs: b} }

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

// ColumnTime returns a Column containing t, formatted with layout when the
// record is written.
func ColumnTime(t time.Time, layout string) Column {
	return Column{kind: columnTime, t: t, s: layout}
}

// ColumnAny returns a Column containing v, rendered with fmt.Sprint when the
// record is written. Unlike the typed constructors, this may allocate.
func ColumnAny(v any) Column { return Column{kind: columnAny, v: v} }
