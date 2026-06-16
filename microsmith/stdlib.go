package microsmith

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strings"
)

// ---- { Types } ----------------------------------------

var BaseTypes = []Type{
	BT{"int"}, BT{"int8"}, BT{"int16"}, BT{"int32"}, BT{"int64"},
	BT{"uint"}, BT{"uint8"}, BT{"uint16"}, BT{"uint32"}, BT{"uint64"}, BT{"uintptr"},
	BT{"float32"}, BT{"float64"}, BT{"complex128"},
	BT{"bool"}, BT{"rune"}, BT{"string"}, BT{"any"},
}

var BigInt = ExternalType{
	Pkg: "big",
	N:   "Int",
	Builder: func() ast.Expr {
		return &ast.UnaryExpr{
			Op: token.MUL,
			X: &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   &ast.Ident{Name: "big"},
					Sel: &ast.Ident{Name: "NewInt"},
				},
				Args: []ast.Expr{
					&ast.BasicLit{Kind: token.INT, Value: "0"},
				},
			},
		}
	},
}

func MakeMethod(name string, args, ret []Type) Method {
	return Method{
		Name: &ast.Ident{Name: name},
		Func: FuncType{Args: args, Ret: ret},
	}
}

func MakeMethods[T any]() []Method {
	var v T
	t := reflect.TypeOf(v)
	var methods []Method

outer:
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)

		var ins []Type
		for i := 1; i < m.Type.NumIn(); i++ { // first one is the receiver
			p := m.Type.In(i)
			in, ok := NameToType(p.Name())
			if !ok {
				//fmt.Printf("Ignoring method %v for ins is %v\n", m, p.Kind())
				continue outer
			}
			ins = append(ins, in)
		}

		p := m.Type.Out(0)
		out, ok := NameToType(p.Name())
		if !ok {
			//			fmt.Println("Ignoring method for out is " + p.Name())
			continue outer
		}

		methods = append(methods, MakeMethod(m.Name, ins, []Type{out}))
	}
	fmt.Println(methods)
	return methods
}

func init() {
	// --- Big.Int -----------------------------------------
	BigInt.Methods = []Method{
		MakeMethod("Add",
			[]Type{PointerType{BigInt}, PointerType{BigInt}},
			[]Type{PointerType{BigInt}}),
		MakeMethod("Abs",
			[]Type{PointerType{BigInt}},
			[]Type{PointerType{BigInt}}),
		MakeMethod("Bytes",
			[]Type{},
			[]Type{SliceOf(BT{"byte"})}),
		MakeMethod("Cmp",
			[]Type{PointerType{BigInt}},
			[]Type{BT{"int"}}),
		MakeMethod("SetBit",
			[]Type{PointerType{BigInt}, BT{"int"}, BT{"uint"}},
			[]Type{PointerType{BigInt}}),
	}
	StdTypes = append(StdTypes, BigInt)
}

var StdTypes = []Type{}

// ---- { Functions } ----------------------------------------

var Builtins = []FuncType{
	{N: "append", Args: nil, Ret: nil},
	{N: "copy", Args: nil, Ret: []Type{BT{"int"}}},
	{N: "len", Args: nil, Ret: []Type{BT{"int"}}},
	{N: "min", Args: nil, Ret: nil},
	{N: "max", Args: nil, Ret: nil},
}

var StdFunctions = []FuncType{
	// fmt
	{
		Pkg:  "fmt",
		N:    "Print",
		Args: nil,
		Ret:  nil,
	},

	// math
	{
		Pkg:  "math",
		N:    "Max",
		Args: []Type{BT{"float64"}, BT{"float64"}},
		Ret:  []Type{BT{"float64"}},
	},
	{
		Pkg:  "math",
		N:    "NaN",
		Args: []Type{},
		Ret:  []Type{BT{"float64"}},
	},
	{
		Pkg:  "math",
		N:    "Ldexp",
		Args: []Type{BT{"float64"}, BT{"int"}},
		Ret:  []Type{BT{"float64"}},
	},
	{
		Pkg:  "math",
		N:    "Sqrt",
		Args: []Type{BT{"float64"}},
		Ret:  []Type{BT{"float64"}},
	},

	// strings
	{
		Pkg:  "strings",
		N:    "Contains",
		Args: []Type{BT{"string"}, BT{"string"}},
		Ret:  []Type{BT{"bool"}},
	},
	{
		Pkg:  "strings",
		N:    "Join",
		Args: []Type{SliceType{BT{"string"}}, BT{"string"}},
		Ret:  []Type{BT{"string"}},
	},
	{
		Pkg: "strings",
		N:   "TrimFunc",
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

	// reflect
	{
		Pkg:  "reflect",
		N:    "DeepEqual",
		Args: nil,
		Ret:  []Type{BT{"bool"}},
	},

	// unsafe
	{
		Pkg:  "unsafe",
		N:    "Sizeof",
		Args: nil,
		Ret:  []Type{BT{"uintptr"}},
	},
	{
		Pkg:  "unsafe",
		N:    "Alignof",
		Args: nil,
		Ret:  []Type{BT{"uintptr"}},
	},
	{
		Pkg:  "unsafe",
		N:    "Offsetof",
		Args: nil,
		Ret:  []Type{BT{"uintptr"}},
	},
	{
		Pkg:  "unsafe",
		N:    "SliceData",
		Args: nil,
		Ret:  nil,
	},
	{
		Pkg:  "unsafe",
		N:    "String",
		Args: []Type{PointerType{BT{"byte"}}, BT{"int"}},
		Ret:  []Type{BT{"string"}},
	},
	{
		Pkg:  "unsafe",
		N:    "StringData",
		Args: []Type{BT{"string"}},
		Ret:  []Type{PointerType{BT{"byte"}}},
	},
}

func MakeAtomicFuncs() []FuncType {
	types := []string{"uint32", "uint64", "uintptr"}
	var vs []FuncType
	for _, t := range types {
		for _, fun := range []string{"Add", "Swap"} {
			f := FuncType{
				Pkg:  "atomic",
				N:    fun + strings.Title(t),
				Args: []Type{PointerOf(BT{t}), BT{t}},
				Ret:  []Type{BT{t}},
			}
			vs = append(vs, f)
		}
	}
	for _, t := range types {
		f := FuncType{
			Pkg:  "atomic",
			N:    "Load" + strings.Title(t),
			Args: []Type{PointerOf(BT{t})},
			Ret:  []Type{BT{t}},
		}
		vs = append(vs, f)
	}

	return vs
}
