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
		case "byte":
			return "by"
		case "int8":
			return "i8_"
		case "int16":
			return "i16_"
		case "int32":
			return "i32_"
		case "int64":
			return "i64_"
		case "int":
			return "i"
		case "uint":
			return "u"
		case "float32":
			return "h"
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

func IsInt(t Type) bool {
	switch t.Name() {
	case "int", "int8", "int16", "int32", "int64":
		return true
	default:
		return false
	}
}

func IsUint(t Type) bool {
	switch t.Name() {
	case "byte", "uint", "uint8", "uint16", "uint32", "uint64":
		return true
	default:
		return false
	}
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

func (at ArrayType) Sliceable() bool {
	return true
}

func ArrOf(t Type) ArrayType {
	return ArrayType{t}
}

func (at ArrayType) TypeAst() *ast.ArrayType {
	return &ast.ArrayType{
		Elt: &ast.Ident{Name: at.Base().Name()},
	}
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

func (st StructType) TypeAst() *ast.StructType {

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
	N:    "len",
	Args: nil, // custom handling
	Ret:  []Type{BasicType{"int"}},
}
var Float32Float64Conv FuncType = FuncType{
	N:    "float32",
	Args: []Type{BasicType{"float64"}},
	Ret:  []Type{BasicType{"float32"}},
}
var Float64Float32Conv FuncType = FuncType{
	N:    "float64",
	Args: []Type{BasicType{"float32"}},
	Ret:  []Type{BasicType{"float64"}},
}
var IntFloat64Conv FuncType = FuncType{
	N:    "float64",
	Args: []Type{BasicType{"int"}},
	Ret:  []Type{BasicType{"float64"}},
}
var IntUintConv FuncType = FuncType{
	N:    "int",
	Args: []Type{BasicType{"uint"}},
	Ret:  []Type{BasicType{"int"}},
}
var UintIntConv FuncType = FuncType{
	N:    "uint",
	Args: []Type{BasicType{"int"}},
	Ret:  []Type{BasicType{"uint"}},
}
var Int16IntConv FuncType = FuncType{
	N:    "int16",
	Args: []Type{BasicType{"int"}},
	Ret:  []Type{BasicType{"int16"}},
}
var IntInt16Conv FuncType = FuncType{
	N:    "int",
	Args: []Type{BasicType{"int16"}},
	Ret:  []Type{BasicType{"int"}},
}
var Int8Int32Conv FuncType = FuncType{
	N:    "int8",
	Args: []Type{BasicType{"int32"}},
	Ret:  []Type{BasicType{"int8"}},
}
var Int32Int8Conv FuncType = FuncType{
	N:    "int32",
	Args: []Type{BasicType{"int8"}},
	Ret:  []Type{BasicType{"int32"}},
}
var Int8UintConv FuncType = FuncType{
	N:    "int8",
	Args: []Type{BasicType{"uint"}},
	Ret:  []Type{BasicType{"int8"}},
}
var IntInt64Conv FuncType = FuncType{
	N:    "int",
	Args: []Type{BasicType{"int64"}},
	Ret:  []Type{BasicType{"int"}},
}

var MathSqrt FuncType = FuncType{
	N:    "math.Sqrt",
	Args: []Type{BasicType{"float64"}},
	Ret:  []Type{BasicType{"float64"}},
}
var MathMax FuncType = FuncType{
	N:    "math.Max",
	Args: []Type{BasicType{"float64"}, BasicType{"float64"}},
	Ret:  []Type{BasicType{"float64"}},
}
var MathNaN FuncType = FuncType{
	N:    "math.NaN",
	Args: []Type{},
	Ret:  []Type{BasicType{"float64"}},
}
var MathLdexp FuncType = FuncType{
	N:    "math.Ldexp",
	Args: []Type{BasicType{"float64"}, BasicType{"int"}},
	Ret:  []Type{BasicType{"float64"}},
}

var PredeclaredFuncs = []FuncType{
	LenFun,
	Float32Float64Conv,
	Float64Float32Conv,
	IntFloat64Conv,
	IntUintConv,
	UintIntConv,
	Int16IntConv,
	IntInt16Conv,
	Int8Int32Conv,
	Int32Int8Conv,
	Int8UintConv,
	IntInt64Conv,
	MathSqrt,
	MathMax,
	MathNaN,
	MathLdexp,
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

func (mp MapType) TypeAst() *ast.MapType {
	return &ast.MapType{
		Key:   &ast.Ident{Name: mp.KeyT.Name()},
		Value: &ast.Ident{Name: mp.ValueT.Name()},
	}
}

// ------------------------------------ //
//   preallocated                       //
// ------------------------------------ //

var Idents = map[string]*ast.Ident{
	"bool":       &ast.Ident{Name: "bool"},
	"byte":       &ast.Ident{Name: "byte"},
	"int":        &ast.Ident{Name: "int"},
	"int8":       &ast.Ident{Name: "int8"},
	"int16":      &ast.Ident{Name: "int16"},
	"int32":      &ast.Ident{Name: "int32"},
	"int64":      &ast.Ident{Name: "int64"},
	"uint":       &ast.Ident{Name: "uint"},
	"float32":    &ast.Ident{Name: "float32"},
	"float64":    &ast.Ident{Name: "float64"},
	"complex128": &ast.Ident{Name: "complex128"},
	"rune":       &ast.Ident{Name: "rune"},
	"string":     &ast.Ident{Name: "string"},
}

func TypeIdent(t string) *ast.Ident {
	if i, ok := Idents[t]; ok {
		return i
	} else {
		return &ast.Ident{Name: t}
	}
}

var LenIdent = &ast.Ident{Name: "len"}
var TrueIdent = &ast.Ident{Name: "true"}
var FalseIdent = &ast.Ident{Name: "false"}
