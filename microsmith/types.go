package microsmith

import "strings"

type Type interface {
	// The type name. This is what is used to actually differentiate
	// types.
	Name() string

	// String to use for variable names of this type.
	Ident() string

	// Returns an ArrayType which base type is the receiver.
	Arr() ArrayType

	Sliceable() bool
}

// ---------------- //
//       basic      //
// ---------------- //

type BasicType struct {
	n string
}

func (bt BasicType) Name() string {
	return bt.n
}

func (bt BasicType) Ident() string {
	return strings.ToUpper(bt.n[:1])
}

func (bt BasicType) Arr() ArrayType {
	return ArrayType{bt}
}

func (bt BasicType) Sliceable() bool {
	return bt.n == "string"
}

// ---------------- //
//       array      //
// ---------------- //

type ArrayType struct {
	Etype Type
}

func (at ArrayType) Name() string {
	return "[]" + at.Etype.Name()
}

func (at ArrayType) Ident() string {
	return "A" + at.Etype.Ident()
}

func (at ArrayType) Arr() ArrayType {
	return ArrayType{at}
}

// given an array type, it returns the corresponding base type
func (at ArrayType) Base() Type {
	return at.Etype
}

func (bt ArrayType) Sliceable() bool {
	return true
}

// ---------------- //
//      struct      //
// ---------------- //

type StructType struct {
	Ftypes []Type   // fields types
	Fnames []string // field names
}
