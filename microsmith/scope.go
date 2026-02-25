package microsmith

import (
	"fmt"
	"go/ast"
)

type Variable struct {
	Type Type
	Name *ast.Ident
}

func (v Variable) String() string {
	return v.Name.String() + " " + v.Type.Name()
}

// A scope holds a list of all the variables that are in scope in a
// given moment
type Scope struct {
	pb   *PackageBuilder
	vars []Variable
}

func (s Scope) String() string {
	if len(s.vars) == 0 {
		return "{empty scope}"
	}
	str := "{\n"
	for i := range s.vars {
		str += s.vars[i].String() + "\n"
	}
	str = str[:len(str)-1] + "\n}"
	return str
}

// Has returns true if the scope has at least one variable of Type t.
func (s Scope) Has(t Type) bool {
	for _, v := range s.vars {
		if v.Type.Equal(t) {
			return true
		}
	}
	return false
}

func (s Scope) FindVarByName(name string) (Variable, bool) {
	for _, v := range s.vars {
		if v.Name.Name == name {
			return v, true
		}
	}
	return Variable{}, false
}

// NewIdent adds a new variable of Type t to the scope, automatically
// assigning it a name that is not already taken. It returns a pointer
// to the new variable's ast.Ident.
func (s *Scope) NewIdent(t Type) *ast.Ident {
	tc := 0
	for _, v := range s.vars {
		if Ident(t) == Ident(v.Type) {
			tc++
		}
	}
	name := fmt.Sprintf("%s%v", Ident(t), tc)
	id := &ast.Ident{Name: name}
	s.vars = append(s.vars, Variable{t, id})
	return id
}

// Adds v to the scope.
func (s *Scope) AddVariable(i *ast.Ident, t Type) {
	s.vars = append(s.vars, Variable{t, i})
}

func (s *Scope) DeleteIdentByName(name *ast.Ident) {
	del := -1
	for i := range s.vars {
		if v := s.vars[i]; v.Name.Name == name.Name {
			del = i
			break
		}
	}

	if del != -1 {
		s.vars = append(s.vars[:del], s.vars[del+1:]...)
	}
}

// Returns a random variable in scope among the ones that satisfy
// pred(v, t). If there isn't one, returns false as the second value.
func (s Scope) RandPred(pred func(v Variable, t ...Type) bool, t ...Type) (Variable, bool) {
	vs := make([]Variable, 0, 256)
	for _, v := range s.vars {
		if pred(v, t...) {
			vs = append(vs, v)
		}
	}
	if len(vs) == 0 {
		return Variable{}, false
	}
	return RandItem(s.pb.rs, vs), true
}

// Returns a random variable in scope that can be used in the LHS of
// an assignment.
func (s Scope) RandAssignable() (Variable, bool) {
	return s.RandPred(func(v Variable, _ ...Type) bool {
		if _, ok := v.Type.(ExternalType); ok {
			return false
		}
		f, fnc := v.Type.(FuncType)
		return (fnc && f.Local) || !fnc
	})
}

// Returns a random (named) function with return type t
func (s Scope) RandFuncRet(t Type) (FuncType, bool) {
	fs := make([]FuncType, 0, 256)
	for _, f := range s.pb.functions {
		switch f.N {
		case "unsafe.SliceData":
			_, isPointer := t.(PointerType)
			if isPointer {
				fs = append(fs, f)
			}
		case "min", "max":
			if IsNumeric(t) || t.Equal(BT{"string"}) {
				fs = append(fs, f)
			}
		default:
			if len(f.Ret) > 0 && f.Ret[0].Equal(t) {
				fs = append(fs, f)
			}
		}
	}

	for _, v := range s.vars {
		if f, ok := v.Type.(FuncType); ok {
			if f.N != "" && len(f.Ret) > 0 && f.Ret[0].Equal(t) {
				fs = append(fs, f)
			}
		}
	}

	if len(fs) == 0 {
		return FuncType{}, false
	}

	return RandItem(s.pb.rs, fs), true
}

// Returns a random (v, Method) pair, with v.Method() having return
// type t
func (s Scope) RandMethod(t Type) (Variable, Method, bool) {
	type S struct {
		v Variable
		m Method
	}
	fs := make([]S, 0, 256)
	for _, v := range s.vars {
		if et, ok := v.Type.(ExternalType); ok {
			for _, met := range et.Methods {
				if len(met.Func.Ret) > 0 {
					if met.Func.Ret[0].Equal(t) {
						fs = append(fs, S{v, met})
					}
				}
			}
		}
	}

	if len(fs) == 0 {
		return Variable{}, Method{}, false
	}

	m := RandItem(s.pb.rs, fs)
	return m.v, m.m, true
}

// Returns a random function in scope; but not a predefined one.
func (s Scope) RandFunc() (Variable, bool) {
	return s.RandPred(func(v Variable, _ ...Type) bool {
		f, fnc := v.Type.(FuncType)
		return (fnc && f.Local) // TODO(alb): why is f.Local needed here?
	})
}

// Return a random variable of type t (exact match)
func (s Scope) RandVar(t Type) (Variable, bool) {
	return s.RandPred(func(v Variable, t ...Type) bool {
		return v.Type.Equal(t[0])
	}, t)
}

// Returns a variable containing t
func (s Scope) RandVarSubType(t Type) (Variable, bool) {
	return s.RandPred(func(v Variable, t ...Type) bool {
		return v.Type.Contains(t[0])
	}, t)
}

// Returns a random variable that can be cleared
func (s Scope) RandClearable() (Variable, bool) {
	return s.RandPred(func(v Variable, _ ...Type) bool {
		switch v.Type.(type) {
		case SliceType, MapType:
			return true
		default:
			return false
		}
	})
}

// Returns a chan (of any subtype)
func (s Scope) RandChan() (Variable, bool) {
	return s.RandPred(func(v Variable, _ ...Type) bool {
		_, ischan := v.Type.(ChanType)
		return ischan
	})
}

// Returns a struct (of any type)
func (s Scope) RandStruct() (Variable, bool) {
	return s.RandPred(func(v Variable, _ ...Type) bool {
		_, isstruct := v.Type.(StructType)
		return isstruct
	})
}

func FindByName(tp []Constraint, name string) Constraint {
	for i := 0; i < len(tp); i++ {
		if tp[i].N.Name == name {
			return tp[i]
		}
	}
	panic("unreachable")
}
