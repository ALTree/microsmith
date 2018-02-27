package microsmith

import "strings"

type Type interface {
	// The type name. This is what is used to actually differentiate
	// types.
	Name() string

	// String to use for variable names of this type.
	Ident() string

	Sliceable() bool
}

// Returns an ArrayType which base type is the receiver.
func ArrOf(t Type) ArrayType {
	return ArrayType{t}
}

// ---------------- //
//       basic      //
// ---------------- //

type BasicType struct {
	N string
}

func (bt BasicType) Name() string {
	return bt.N
}

func (bt BasicType) Ident() string {
	return strings.ToUpper(bt.N[:1])
}

func (bt BasicType) Sliceable() bool {
	return bt.N == "string"
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

// ---------------- //
//       func       //
// ---------------- //

type FuncType struct {
	N    string
	Args []Type
	Ret  []Type
}

func (ft FuncType) Name() string {
	return ft.N
}

func (ft FuncType) Ident() string {
	return "F"
}

func (ft FuncType) Sliceable() bool {
	return false
}

var LenFun FuncType = FuncType{"len", nil, []Type{BasicType{"int"}}}
