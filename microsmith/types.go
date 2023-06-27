package microsmith

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"strings"
)

type Type interface {
	Comparable() bool     // in the go specification sense
	Ast() ast.Expr        // suitable for type declaration in the ast
	Equal(t Type) bool    // is t of the same type
	Name() string         // human-readable type name
	Sliceable() bool      // can []
	Contains(t Type) bool // does the type contain type t
}

// Name to use for variables of type t
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
		case "uintptr":
			return "up"
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
		case "any":
			return "an"
		default:
			panic("unhandled type " + t.N)
		}
	case ArrayType:
		return "a" + Ident(t.Etype)
	case FuncType:
		if t.N != "FU" {
			// buildin and stdlib functions don't need identifiers
			return ""
		} else {
			return "fnc"
		}
	case StructType:
		return "st"
	case ChanType:
		return "ch"
	case MapType:
		return "m"
	case PointerType:
		return "p" + Ident(t.Btype)
	case TypeParam:
		return strings.ToLower(t.N.Name) + "_"
	default:
		panic("Ident: unknown type " + t.Name())
	}
}

// --------------------------------
//   basic
// --------------------------------

type BasicType struct {
	N string
}

func (t BasicType) Comparable() bool {
	return t.N != "any"
}

func (t BasicType) Ast() ast.Expr {
	return TypeIdent(t.N)
}

func (t BasicType) Contains(t2 Type) bool {
	return t.Equal(t2)
}

func (t BasicType) Equal(t2 Type) bool {
	if t2, ok := t2.(BasicType); !ok {
		return false
	} else {
		return t.N == t2.N
	}
}

func (bt BasicType) Name() string {
	return bt.N
}

func (bt BasicType) Sliceable() bool {
	return bt.N == "string"
}

func IsNumeric(t Type) bool {
	if bt, ok := t.(BasicType); !ok {
		return false
	} else {
		switch bt.N {
		case "int", "int8", "int16", "int32", "int64":
			return true
		case "byte", "uint", "uint8", "uint16", "uint32", "uint64":
			return true
		case "float32", "float64":
			return true
		default:
			return false
		}
	}
}

func IsOrdered(t Type) bool {
	if bt, ok := t.(BasicType); !ok {
		return false
	} else {
		switch bt.Name() {
		case "bool", "complex128":
			return false
		default:
			return true
		}
	}
}

func (t BasicType) NeedsCast() bool {
	switch t.N {
	case "byte", "int8", "int16", "int32", "int64", "uint", "uintptr", "float32", "any":
		return true
	default:
		return false
	}
}

// --------------------------------
//   pointer
// --------------------------------

type PointerType struct {
	Btype Type
}

func (t PointerType) Comparable() bool {
	return true
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

// --------------------------------
//   array
// --------------------------------

type ArrayType struct {
	Etype Type
}

func (t ArrayType) Comparable() bool {
	return false
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

// --------------------------------
//   struct
// --------------------------------

type StructType struct {
	N      string
	Ftypes []Type   // fields types
	Fnames []string // field names
}

func (t StructType) Comparable() bool {
	for _, t := range t.Ftypes {
		if !t.Comparable() {
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
	var buf bytes.Buffer
	format.Node(&buf, token.NewFileSet(), st.Ast())
	return buf.String()
}

func (st StructType) Sliceable() bool {
	return false
}

// --------------------------------
//   func
// --------------------------------

type FuncType struct {
	N     string
	Args  []Type
	Ret   []Type
	Local bool
}

func (t FuncType) Comparable() bool {
	return false
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

		if len(ft.Ret) != len(t.Ret) {
			return false
		}
		for i := range ft.Ret {
			if !ft.Ret[i].Equal(t.Ret[i]) {
				return false
			}
		}

		if len(ft.Args) != len(t.Args) {
			return false
		}
		for i := range ft.Args {
			if !ft.Args[i].Equal(t.Args[i]) {
				return false
			}
		}
		return true
	}
}

func (f FuncType) Contains(t Type) bool {
	if f.Equal(t) {
		return true
	}
	return len(f.Ret) > 0 && t.Equal(f.Ret[0])
}

func (ft FuncType) Name() string {
	var buf bytes.Buffer
	format.Node(&buf, token.NewFileSet(), ft.Ast())
	return buf.String()
}

func (ft FuncType) Sliceable() bool {
	return false
}

// Internal implementation detail for variadic parameters in
// functions. Should never be generated or be part of a scope.
type EllipsisType struct {
	Base Type
}

func (e EllipsisType) Ast() ast.Expr {
	return &ast.Ellipsis{Elt: e.Base.Ast()}
}

func (e EllipsisType) Equal(t Type) bool {
	if t, ok := t.(EllipsisType); !ok {
		return false
	} else {
		return e.Base.Equal(t.Base)
	}
}

func (e EllipsisType) Comparable() bool     { panic("don't call") }
func (e EllipsisType) Name() string         { panic("don't call") }
func (e EllipsisType) Sliceable() bool      { panic("don't call") }
func (e EllipsisType) Contains(t Type) bool { panic("don't call") }

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

type BT = BasicType

// Casts, builtins, and some standard library function that can be
// assumed to exists, and can be called. Any null Args or Ret is
// custom-handled when generating the Expressions.
var BuiltinsFuncs = []FuncType{

	// builtins ----------------
	{
		N:    "append",
		Args: nil,
		Ret:  nil,
	},
	{
		N:    "copy",
		Args: nil,
		Ret:  []Type{BT{"int"}},
	},
	{
		N:    "len",
		Args: nil,
		Ret:  []Type{BT{"int"}},
	},
	{
		N:    "min",
		Args: nil,
		Ret:  nil,
	},
	{
		N:    "max",
		Args: nil,
		Ret:  nil,
	},
}

var StdlibFuncs = []FuncType{

	// math ----------------
	{
		N:    "math.Max",
		Args: []Type{BT{"float64"}, BT{"float64"}},
		Ret:  []Type{BT{"float64"}},
	},
	{
		N:    "math.NaN",
		Args: []Type{},
		Ret:  []Type{BT{"float64"}},
	},
	{
		N:    "math.Ldexp",
		Args: []Type{BT{"float64"}, BT{"int"}},
		Ret:  []Type{BT{"float64"}},
	},
	{
		N:    "math.Sqrt",
		Args: []Type{BT{"float64"}},
		Ret:  []Type{BT{"float64"}},
	},

	// strings ----------------
	{
		N:    "strings.Contains",
		Args: []Type{BT{"string"}, BT{"string"}},
		Ret:  []Type{BT{"bool"}},
	},
	{
		N:    "strings.Join",
		Args: []Type{ArrayType{BT{"string"}}, BT{"string"}},
		Ret:  []Type{BT{"string"}},
	},
	{
		N: "strings.TrimFunc",
		Args: []Type{
			BT{"string"},
			FuncType{
				Args:  []Type{BT{"rune"}},
				Ret:   []Type{BT{"bool"}},
				Local: true,
			},
		},
		Ret: []Type{BT{"string"}},
	},

	// reflect ----------------
	{
		N:    "reflect.DeepEqual",
		Args: nil,
		Ret:  []Type{BT{"bool"}},
	},

	// unsafe ----------------
	{
		N:    "unsafe.Sizeof",
		Args: nil,
		Ret:  []Type{BT{"uintptr"}},
	},
	{
		N:    "unsafe.Alignof",
		Args: nil,
		Ret:  []Type{BT{"uintptr"}},
	},
	{
		N:    "unsafe.Offsetof",
		Args: nil,
		Ret:  []Type{BT{"uintptr"}},
	},
	{
		N:    "unsafe.SliceData",
		Args: nil,
		Ret:  nil,
	},
	{
		N:    "unsafe.String",
		Args: []Type{PointerType{BT{"byte"}}, BT{"int"}},
		Ret:  []Type{BT{"string"}},
	},
	{
		N:    "unsafe.StringData",
		Args: []Type{BT{"string"}},
		Ret:  []Type{PointerType{BT{"byte"}}},
	},
}

// --------------------------------
//   Chan
// --------------------------------

type ChanType struct {
	T Type
}

func (t ChanType) Comparable() bool {
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

// --------------------------------
//   map
// --------------------------------

type MapType struct {
	KeyT, ValueT Type
}

func (t MapType) Comparable() bool {
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

// --------------------------------
//   Contraint
// --------------------------------

// type I0 {        <---- N
//   int | string   <-- Types
// }
type Constraint struct {
	N     *ast.Ident
	Types []Type
}

func (c Constraint) Comparable() bool {
	for _, t := range c.Types {
		if !t.Comparable() {
			return false
		}
	}
	return true
}

func (c Constraint) Ast() ast.Expr {
	return c.N // TODO(alb): right?
}

func (c Constraint) Equal(t Type) bool {
	if t2, ok := t.(Constraint); !ok {
		return false
	} else {
		if len(c.Types) != len(t2.Types) {
			return false
		}
		for i := range c.Types {
			if !c.Types[i].Equal(t2.Types[i]) { // TODO(alb): fix, needs sorting
				return false
			}
		}
		return true
	}
}

func (c Constraint) Name() string {
	return c.N.Name
}

func (c Constraint) Sliceable() bool {
	return false
}

func (c Constraint) Contains(t Type) bool {
	return c.Equal(t)
}

func (c Constraint) String() string {
	str := "{" + c.N.Name + " "
	for _, t := range c.Types {
		str += t.Name() + "|"
	}
	str = str[:len(str)-1] + "}"
	return str
}

// --------------------------------
//   TypeParam
// --------------------------------

type TypeParam struct {
	N          *ast.Ident
	Constraint Constraint
}

func (tp TypeParam) Comparable() bool {
	return tp.Constraint.Comparable()

}

func (tp TypeParam) Ast() ast.Expr {
	return tp.N
}

func (tp TypeParam) Equal(t Type) bool {
	if t2, ok := t.(TypeParam); !ok {
		return false
	} else {
		return tp.N.Name == t2.N.Name
	}
}

func (tp TypeParam) Name() string {
	return tp.N.Name
}

func (tp TypeParam) Sliceable() bool {
	return false
}

func (tp TypeParam) Contains(t Type) bool {
	return tp.Equal(t)
}

func (tp TypeParam) HasLiterals() bool {
	for _, t := range tp.Constraint.Types {
		if !IsNumeric(t) {
			return false
		}
	}
	return true
}

func MakeTypeParam(v Variable) TypeParam {
	return TypeParam{N: v.Name, Constraint: v.Type.(Constraint)}
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
	"uintptr":    &ast.Ident{Name: "uintptr"},
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

var AppendIdent = &ast.Ident{Name: "append"}
var CopyIdent = &ast.Ident{Name: "copy"}
var LenIdent = &ast.Ident{Name: "len"}
var MakeIdent = &ast.Ident{Name: "make"}
var CloseIdent = &ast.Ident{Name: "close"}
var SizeofIdent = &ast.Ident{Name: "Sizeof"}
var TrueIdent = &ast.Ident{Name: "true"}
var FalseIdent = &ast.Ident{Name: "false"}

// ------------------------------------ //
//   Ops                                //
// ------------------------------------ //

func UnaryOps(t Type) []token.Token {
	switch t2 := t.(type) {
	case BasicType:
		switch t.Name() {
		case "byte", "uint", "uintptr":
			return []token.Token{token.ADD}
		case "int", "rune", "int8", "int16", "int32", "int64":
			return []token.Token{token.ADD, token.SUB, token.XOR}
		case "float32", "float64", "complex128":
			return []token.Token{token.ADD, token.SUB}
		case "bool":
			return []token.Token{token.NOT}
		case "string", "any":
			return []token.Token{}
		default:
			panic("Unhandled BasicType " + t.Name())
		}
	case TypeParam:
		return t2.CommonOps(UnaryOps)
	default:
		return []token.Token{}
	}
}

func BinOps(t Type) []token.Token {
	switch t2 := t.(type) {

	case BasicType:
		switch t.Name() {
		case "byte", "uint", "int8", "int16", "int32", "int64":
			return []token.Token{
				token.ADD, token.AND, token.AND_NOT, token.MUL,
				token.OR, token.QUO, token.REM, token.SHL, token.SHR,
				token.SUB, token.XOR,
			}
		case "uintptr":
			return []token.Token{
				token.ADD, token.AND, token.AND_NOT, token.OR, token.XOR,
			}
		case "int":
			// We can't generate shifts for ints, because int expressions
			// are used as args for float64() conversions, and in this:
			//
			//   var i int = 2
			// 	 float64(8 >> i)
			//
			// 8 is actually of type float64; because, from the spec:
			//
			//   If the left operand of a non-constant shift expression is
			//   an untyped constant, it is first implicitly converted to
			//   the type it would assume if the shift expression were
			//   replaced by its left operand alone.
			//
			// ans apparently in float64(8), 8 is a float64. So
			//
			//   float64(8 >> i)
			//
			// fails to compile with error:
			//
			//   invalid operation: 8 >> i (shift of type float64)
			return []token.Token{
				token.ADD, token.AND, token.AND_NOT, token.MUL,
				token.OR, token.QUO, token.REM, /*token.SHL, token.SHR,*/
				token.SUB, token.XOR,
			}
		case "rune":
			return []token.Token{
				token.ADD, token.AND, token.AND_NOT,
				token.OR, token.SHR, token.SUB, token.XOR,
			}
		case "float32", "float64":
			return []token.Token{token.ADD, token.SUB, token.MUL, token.QUO}
		case "complex128":
			return []token.Token{token.ADD, token.SUB, token.MUL}
		case "bool":
			return []token.Token{token.LAND, token.LOR}
		case "string":
			return []token.Token{token.ADD}
		case "any":
			return []token.Token{}
		default:
			panic("Unhandled BasicType " + t.Name())
		}

	case TypeParam:
		return t2.CommonOps(BinOps)

	default:
		return []token.Token{}
	}
}

func (t TypeParam) CommonOps(fn func(t Type) []token.Token) []token.Token {
	// TODO(alb): cache this
	m := make(map[token.Token]int)
	for _, st := range t.Constraint.Types {
		for _, t2 := range fn(st) {
			m[t2]++
		}
	}
	res := make([]token.Token, 0, 8)
	for i, k := range m {
		if k == len(t.Constraint.Types) {
			res = append(res, i)
		}
	}
	return res
}
