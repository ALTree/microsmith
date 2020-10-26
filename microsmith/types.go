package microsmith

import (
	"fmt"
	"go/ast"
	"math/rand"
	"strconv"
)

type Type interface {
	// The type name. This is what is used to actually differentiate
	// types.
	Name() string
	Sliceable() bool
}

// Name to use for variable of type t
func Ident(t Type) string {
	switch t := t.(type) {
	case BasicType:
		switch t.N {
		case "bool":
			return "b"
		case "uint":
			return "ui"
		case "int":
			return "i"
		case "float64":
			return "f"
		case "complex128":
			return "c"
		case "string":
			return "s"
		case "rune":
			return "r"
		default:
			panic("unhandled type: " + t.N)
		}
	case ArrayType:
		return "a" + Ident(t.Etype)
	case FuncType:
		return "fnc"
	case StructType:
		return "st"
	case ChanType:
		return "ch"
	case MapType:
		return "m"
	case PointerType:
		return "p" + Ident(t.Btype)
	default:
		panic("Ident: unknown type " + t.Name())
	}
}

func Addressable(t Type) bool {
	switch t := t.(type) {
	case BasicType, ArrayType, StructType, MapType, PointerType:
		return true
	case FuncType:
		// Pre-declared or external function cannot be assigned to,
		// local user-defined functions can.
		if t.Local {
			return true
		} else {
			return false
		}
	case ChanType:
		return false
	default:
		panic("Addressable: unknown type " + t.Name())
	}
}

// -------------------------------- //
//   basic                          //
// -------------------------------- //

type BasicType struct {
	N string
}

func (bt BasicType) Name() string {
	return bt.N
}

func (bt BasicType) Sliceable() bool {
	return bt.N == "string"
}

// -------------------------------- //
//   pointer                        //
// -------------------------------- //

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

// -------------------------------- //
//   array                          //
// -------------------------------- //

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

func ArrOf(t Type) ArrayType {
	return ArrayType{t}
}

// -------------------------------- //
//   struct                         //
// -------------------------------- //

const MaxStructFields = 6

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

func (st StructType) BuildAst() *ast.StructType {

	fields := make([]*ast.Field, 0, len(st.Fnames))

	for i := range st.Fnames {
		field := &ast.Field{
			Names: []*ast.Ident{&ast.Ident{Name: st.Fnames[i]}},
			Type:  TypeIdent(st.Ftypes[i].Name()),
		}
		fields = append(fields, field)
	}

	return &ast.StructType{
		Fields: &ast.FieldList{
			List: fields,
		},
	}
}

func RandStructType(EnabledTypes []Type) StructType {
	st := StructType{
		"ST",
		[]Type{},
		[]string{},
	}

	nfields := 2 + rand.Intn(MaxStructFields-1)
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

// -------------------------------- //
//   func                           //
// -------------------------------- //

type FuncType struct {
	N     string
	Args  []Type
	Ret   []Type
	Local bool
}

func (ft FuncType) Name() string {
	return ft.N
}

func (ft FuncType) Sliceable() bool {
	return false
}

// Build two ast.FieldList object (one for params, the other for
// resultss) from a FuncType, to use in function declarations and
// function literals. If named is true, it gives the function
// parameters names (p<s>, p<s+1>, ...)
func (ft FuncType) MakeFieldLists(named bool, s int) (*ast.FieldList, *ast.FieldList) {

	params := &ast.FieldList{
		List: make([]*ast.Field, 0, len(ft.Args)),
	}
	for i, arg := range ft.Args {
		p := ast.Field{
			Type: &ast.Ident{Name: arg.Name()},
		}
		if named {
			p.Names = []*ast.Ident{
				&ast.Ident{Name: fmt.Sprintf("p%d", s+i)},
			}
		}
		params.List = append(params.List, &p)
	}

	results := &ast.FieldList{
		List: make([]*ast.Field, 0, len(ft.Ret)),
	}
	for _, arg := range ft.Ret {
		results.List = append(
			results.List,
			&ast.Field{Type: &ast.Ident{Name: arg.Name()}},
		)
	}

	return params, results
}

func RandFuncType(EnabledTypes []Type) FuncType {
	args := make([]Type, 0, rand.Intn(6))
	for i := 0; i < cap(args); i++ {
		typ := RandType(EnabledTypes)
		if t, ok := typ.(BasicType); !ok {
			panic("RandFuncType: non basic type " + typ.Name())
		} else {
			args = append(args, t)
		}
	}
	ret := []Type{RandType(EnabledTypes)}
	return FuncType{"FU", args, ret, true}
}

var LenFun FuncType = FuncType{
	"len",
	nil, // custom handling
	[]Type{BasicType{"int"}},
	false,
}
var FloatConv FuncType = FuncType{
	"float64",
	[]Type{BasicType{"int"}},
	[]Type{BasicType{"float64"}},
	false,
}
var MathSqrt FuncType = FuncType{
	"math.Sqrt",
	[]Type{BasicType{"float64"}},
	[]Type{BasicType{"float64"}},
	false,
}
var MathMax FuncType = FuncType{
	"math.Max",
	[]Type{BasicType{"float64"}, BasicType{"float64"}},
	[]Type{BasicType{"float64"}},
	false,
}
var MathNaN FuncType = FuncType{
	"math.NaN",
	[]Type{},
	[]Type{BasicType{"float64"}},
	false,
}

// -------------------------------- //
//   chan                           //
// -------------------------------- //

type ChanType struct {
	T Type
}

func (ct ChanType) Name() string {
	return "chan " + ct.T.Name()
}

// given a chan type, it returns the corresponding base type
func (ct ChanType) Base() Type {
	return ct.T
}

func (ct ChanType) Sliceable() bool {
	return false
}

func ChanOf(t Type) ChanType {
	return ChanType{t}
}

// -------------------------------- //
//   map                            //
// -------------------------------- //

type MapType struct {
	KeyT, ValueT Type
}

func (mt MapType) Name() string {
	return "map[" + mt.KeyT.Name() + "]" + mt.ValueT.Name()
}

func (mt MapType) Sliceable() bool {
	return true
}

func MapOf(kt, vt Type) MapType {
	return MapType{kt, vt}
}

// ------------------------------------ //
//   preallocated                       //
// ------------------------------------ //

var BoolIdent = &ast.Ident{Name: "bool"}
var UintIdent = &ast.Ident{Name: "uint"}
var IntIdent = &ast.Ident{Name: "int"}
var FloatIdent = &ast.Ident{Name: "float64"}
var ComplexIdent = &ast.Ident{Name: "complex128"}
var StringIdent = &ast.Ident{Name: "string"}
var RuneIdent = &ast.Ident{Name: "rune"}

func TypeIdent(t string) *ast.Ident {
	switch t {
	case "bool":
		return BoolIdent
	case "uint":
		return UintIdent
	case "int":
		return IntIdent
	case "float64":
		return FloatIdent
	case "complex128":
		return ComplexIdent
	case "string":
		return StringIdent
	case "rune":
		return RuneIdent

	default:
		panic("TypeIdent: cannot handle type " + t)
	}
}

var LenIdent = &ast.Ident{Name: "len"}
var TrueIdent = &ast.Ident{Name: "true"}
var FalseIdent = &ast.Ident{Name: "false"}
