package microsmith

import (
	"go/ast"
	"math/rand"
	"strconv"
	"strings"
)

type Type interface {
	// The type name. This is what is used to actually differentiate
	// types.
	Name() string

	// Ident() string

	Sliceable() bool
}

// Returns an ArrayType which base type is the receiver.
func ArrOf(t Type) ArrayType {
	return ArrayType{t}
}

// String to use for variable names of this type.
func Ident(t Type) string {
	switch t := t.(type) {
	case BasicType:
		return strings.ToUpper(t.N[:1])
	case ArrayType:
		return "A" + Ident(t.Etype)
	case FuncType:
		return "FNC"
	case StructType: // TODO(alb): structs needs a better naming system
		return "ST"
	case PointerType:
		return "P" + Ident(t.Btype)
	default:
		panic("Ident: unknown type " + t.Name())
	}
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

func (bt BasicType) Sliceable() bool {
	return bt.N == "string"
}

// ---------------- //
//      pointer     //
// ---------------- //
type PointerType struct {
	Btype Type
}

func (pt PointerType) Name() string {
	return "*" + pt.Btype.Name()
}

func (pt PointerType) Base() Type {
	return pt.Btype
}

func (pt PointerType) Sliceable() bool {
	return false
}

func PointerOf(t Type) PointerType {
	return PointerType{t}
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

const MaxStructFields = 4

type StructType struct {
	N      string
	Ftypes []Type   // fields types
	Fnames []string // field names
}

func (st StructType) Name() string {
	return st.N
}
func (st StructType) Sliceable() bool {
	return false
}

func (st StructType) String() string {
	s := st.N + "\n"
	for i := 0; i < len(st.Ftypes); i++ {
		s += "  " + st.Fnames[i] + " " + st.Ftypes[i].Name() + "\n"
	}
	return s
}

func RandStructType(EnabledTypes []Type) StructType {
	st := StructType{
		"ST",
		[]Type{},
		[]string{},
	}

	nfields := 1 + rand.Intn(MaxStructFields)
	for i := 0; i < nfields; i++ {
		typ := RandType(EnabledTypes)
		if t, ok := typ.(BasicType); !ok {
			panic("RandStructType: non basic type " + typ.Name())
		} else {
			st.Ftypes = append(st.Ftypes, t)
			st.Fnames = append(st.Fnames, Ident(t)+strconv.Itoa(i))
		}
	}

	return st
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

func (ft FuncType) Sliceable() bool {
	return false
}

var LenFun FuncType = FuncType{
	"len",
	nil, // len args are handled separately
	[]Type{BasicType{"int"}},
}

var FloatConv FuncType = FuncType{
	"float64",
	[]Type{BasicType{"int"}}, // for now we only convert ints
	[]Type{BasicType{"float64"}},
}

var IntConv FuncType = FuncType{
	"int",
	[]Type{BasicType{"float64"}}, // for now we only convert ints
	[]Type{BasicType{"int"}},
}

var MathSqrt FuncType = FuncType{
	"math.Sqrt",
	[]Type{BasicType{"float64"}},
	[]Type{BasicType{"float64"}},
}

var BoolIdent = &ast.Ident{Name: "bool"}
var IntIdent = &ast.Ident{Name: "int"}
var FloatIdent = &ast.Ident{Name: "float64"}
var ComplexIdent = &ast.Ident{Name: "complex128"}
var StringIdent = &ast.Ident{Name: "string"}

func TypeIdent(t string) *ast.Ident {
	switch t {
	case "bool":
		return BoolIdent
	case "int":
		return IntIdent
	case "float64":
		return FloatIdent
	case "complex128":
		return ComplexIdent
	case "string":
		return StringIdent
	default:
		panic("TypeIdent: cannot handle type " + t)
	}
}

var LenIdent = &ast.Ident{Name: "len"}
var TrueIdent = &ast.Ident{Name: "true"}
var FalseIdent = &ast.Ident{Name: "false"}
