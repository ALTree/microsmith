package microsmith

import (
	"fmt"
	"go/ast"
)

type Type interface {
	Addressable() bool
	Ast() ast.Expr
	Equal(Type) bool
	Name() string
	Sliceable() bool
	Contains(t Type) bool
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
	case TypeParam:
		return "x"
	default:
		panic("Ident: unknown type " + t.Name())
	}
}

// -------------------------------- //
//   basic                          //
// -------------------------------- //

type BasicType struct {
	N string
}

func (t BasicType) Addressable() bool {
	return true
}

func (t BasicType) Ast() ast.Expr {
	return TypeIdent(t.Name())
}

func (t BasicType) Contains(t2 Type) bool {
	return t.Equal(t2)
}

func (t BasicType) Equal(t2 Type) bool {
	if t2, ok := t2.(BasicType); !ok {
		return false
	} else {
		return t.Name() == t2.Name()
	}
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

func (t PointerType) Addressable() bool {
	return t.Base().Addressable()
}

func (t PointerType) Ast() ast.Expr {
	return &ast.StarExpr{X: t.Base().Ast()}
}

func (t PointerType) Base() Type {
	return t.Btype
}

func (t PointerType) Contains(t2 Type) bool {
	if t.Equal(t2) {
		return true
	} else {
		return t.Base().Contains(t2)
	}
}

func (pt PointerType) Equal(t Type) bool {
	if t2, ok := t.(PointerType); !ok {
		return false
	} else {
		return pt.Base().Equal(t2.Base())
	}
}

func (pt PointerType) Name() string {
	return "*" + pt.Btype.Name()
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

func (t ArrayType) Addressable() bool {
	return true
}

func (t ArrayType) Ast() ast.Expr {
	return &ast.ArrayType{Elt: t.Base().Ast()}
}

func (at ArrayType) Base() Type {
	return at.Etype
}

func (t ArrayType) Contains(t2 Type) bool {
	if t.Equal(t2) {
		return true
	} else {
		return t.Base().Contains(t2)
	}
}

func (at ArrayType) Equal(t Type) bool {
	if t2, ok := t.(ArrayType); !ok {
		return false
	} else {
		return at.Base().Equal(t2.Base())
	}
}

func (at ArrayType) Name() string {
	return "[]" + at.Etype.Name()
}

func (at ArrayType) Sliceable() bool {
	return true
}

func ArrayOf(t Type) ArrayType {
	return ArrayType{t}
}

// -------------------------------- //
//   struct                         //
// -------------------------------- //

type StructType struct {
	N      string
	Ftypes []Type   // fields types
	Fnames []string // field names
}

func (t StructType) Addressable() bool {
	for _, t := range t.Ftypes {
		if !t.Addressable() {
			return false
		}
	}
	return true
}

func (t StructType) Ast() ast.Expr {
	fields := make([]*ast.Field, 0, len(t.Fnames))
	for i := range t.Fnames {
		field := &ast.Field{
			Names: []*ast.Ident{&ast.Ident{Name: t.Fnames[i]}},
			Type:  t.Ftypes[i].Ast(),
		}
		fields = append(fields, field)
	}

	return &ast.StructType{
		Fields: &ast.FieldList{List: fields},
	}
}

func (t StructType) Contains(t2 Type) bool {
	if t.Equal(t2) {
		return true
	}

	for _, ft := range t.Ftypes {
		if ft.Contains(t2) {
			return true
		}
	}
	return false
}

func (st StructType) Equal(t Type) bool {
	if t2, ok := t.(StructType); !ok {
		return false
	} else {
		if len(st.Ftypes) != len(t2.Ftypes) {
			return false
		}
		for i := range st.Ftypes {
			if !st.Ftypes[i].Equal(t2.Ftypes[i]) {
				return false
			}
		}
	}
	return true
}

func (st StructType) Name() string {
	s := "struct{"
	for _, t := range st.Ftypes {
		s += " " + t.Name() + ","
	}
	s += " }"
	return s
}

func (st StructType) Sliceable() bool {
	return false
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

func (t FuncType) Addressable() bool {
	return t.Local
}

func (t FuncType) Ast() ast.Expr {
	p, r := t.MakeFieldLists(false, 0)
	return &ast.FuncType{Params: p, Results: r}
}

func (ft FuncType) Equal(t Type) bool {
	if t, ok := t.(FuncType); !ok {
		return false
	} else {
		if ft.Local != t.Local {
			return false
		}

		if !ft.Ret[0].Equal(t.Ret[0]) {
			return false
		}

		if len(ft.Args) != len(t.Args) {
			return false
		}
		for i := range ft.Args {
			if !ft.Args[i].Equal(t.Args[i]) {
				return false
			}
		}
	}
	return true
}

func (f FuncType) Contains(t Type) bool {
	return f.Equal(t)
}

func (ft FuncType) Name() string {
	return ft.N
}

func (ft FuncType) Sliceable() bool {
	return false
}

// Build two ast.FieldList object (one for params, the other for
// results) from a FuncType, to use in function declarations and
// function literals. If named is true, it gives the function
// parameters names (p<s>, p<s+1>, ...)
func (ft FuncType) MakeFieldLists(named bool, s int) (*ast.FieldList, *ast.FieldList) {
	params := &ast.FieldList{
		List: make([]*ast.Field, 0, len(ft.Args)),
	}
	for i, arg := range ft.Args {
		p := ast.Field{Type: arg.Ast()}
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
			&ast.Field{Type: arg.Ast()},
		)
	}

	return params, results
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

func (t ChanType) Addressable() bool {
	return false
}

func (t ChanType) Ast() ast.Expr {
	return &ast.ChanType{
		Dir:   3,
		Value: t.Base().Ast(),
	}
}

func (ct ChanType) Base() Type {
	return ct.T
}

func (t ChanType) Contains(t2 Type) bool {
	if t.Equal(t2) {
		return true
	} else {
		return t.Base().Contains(t2)
	}
}

func (t ChanType) Equal(t2 Type) bool {
	if t2, ok := t2.(ChanType); !ok {
		return false
	} else {
		return t.Base().Equal(t2.Base())
	}
}

func (ct ChanType) Name() string {
	return "chan " + ct.T.Name()
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

func (t MapType) Addressable() bool {
	return false
}

func (t MapType) Ast() ast.Expr {
	return &ast.MapType{
		Key:   t.KeyT.Ast(),
		Value: t.ValueT.Ast(),
	}
}

func (t MapType) Contains(t2 Type) bool {
	if t.Equal(t2) {
		return true
	}

	return t.ValueT.Contains(t2)
}

func (t MapType) Equal(t2 Type) bool {
	if t2, ok := t2.(MapType); !ok {
		return false
	} else {
		return t.KeyT.Equal(t2.KeyT) && t.ValueT.Equal(t2.ValueT)
	}
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
//   Type Parameter                     //
// ------------------------------------ //

type TypeParam struct {
	Types []Type
	N     *ast.Ident
}

func (tp TypeParam) Addressable() bool {
	return true
}

func (tp TypeParam) Ast() ast.Expr {
	return tp.N
}

func (tp TypeParam) Equal(t Type) bool {
	if t2, ok := t.(TypeParam); !ok {
		return false
	} else {
		if len(tp.Types) != len(t2.Types) {
			return false
		}
		for i := range tp.Types {
			if !tp.Types[i].Equal(t2.Types[i]) { // TODO(alb): fix, needs sorting
				return false
			}
		}
		return true
	}
}

func (tp TypeParam) Name() string {
	return tp.String()
}

func (tp TypeParam) Sliceable() bool {
	return false
}

func (tp TypeParam) Contains(t Type) bool {
	return tp.Equal(t)
}

func (tp TypeParam) String() string {
	str := "{" + tp.N.Name + " "
	for _, t := range tp.Types {
		str += t.Name() + "|"
	}
	str = str[:len(str)-1] + "}"
	return str
}

// ------------------------------------ //
//   Contraints                         //
// ------------------------------------ //

// type I0 {        <---- N
//   int | string   <-- Types
// }
type Constraint struct {
	N     *ast.Ident
	Types []Type
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
