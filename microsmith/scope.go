package microsmith

import (
	"fmt"
	"go/ast"
	"reflect"
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
	str = str[:len(s.vars)-1] + "\n}"
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

// NewIdent adds a new variable of Type t to the scope, automatically
// assigning it a name that is not already taken. It returns a pointer
// to the new variable's ast.Ident.
func (s *Scope) NewIdent(t Type) *ast.Ident {
	tc := 0
	for _, v := range s.vars {
		if _, basic := t.(BasicType); basic && v.Type.Equal(t) {
			tc++
		} else if reflect.TypeOf(t) == reflect.TypeOf(v.Type) {
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

// Returns a random Addressable variable in scope, that can be used in
// the LHS of an AssignStmt. If nofunc is TRUE, ignore FuncType
// variables.
func (s Scope) RandomVar(nofunc bool) Variable {
	vs := make([]Variable, 0, 256)
	for _, v := range s.vars {
		// Maps are NOT addressable, but it doesn't matter here
		// because the only RandomVar caller (AssignStmt), always
		// assigns to maps as m[...] = , and that is allowed. What is
		// not allowed is m[...].i =.
		if _, ok := v.Type.(MapType); ok {
			vs = append(vs, v)
			continue
		}
		if v.Type.Addressable() {
			if nofunc {
				if _, ok := v.Type.(FuncType); !ok {
					vs = append(vs, v)
				}
			} else {
				vs = append(vs, v)
			}
		}
	}

	if len(vs) == 0 {
		fmt.Println(s)
		panic("RandomVar: no addressable variable in scope")
	}

	return RandItem(s.pb.rs, vs)
}

// Returns a function with return type t
func (s Scope) GetRandomFunc(t Type) (Variable, bool) {
	funcs := make([]Variable, 0, 32)
	for _, v := range s.vars {
		if ft, ok := v.Type.(FuncType); ok && ft.Ret[0].Equal(t) {
			funcs = append(funcs, v)
		}
	}
	if len(funcs) == 0 {
		return Variable{}, false
	}
	return RandItem(s.pb.rs, funcs), true
}

// Returns a random function in scope; but not a predefined one.
func (s Scope) GetRandomFuncAnyType() (Variable, bool) {
	funcs := make([]Variable, 0, 32)
	for _, v := range s.vars {
		if t, ok := v.Type.(FuncType); ok && t.Local {
			funcs = append(funcs, v)
		}
	}
	if len(funcs) == 0 {
		return Variable{}, false
	}
	return RandItem(s.pb.rs, funcs), true
}

// Return a random Variable of type t (exact match)
func (s Scope) GetRandomVarOfType(t Type) (Variable, bool) {
	cnt := 0
	for _, v := range s.vars {
		if t.Equal(v.Type) {
			cnt++
		}
	}

	if cnt == 0 {
		return Variable{}, false
	}

	rand := 1 + s.pb.rs.Intn(cnt)
	cnt = 0
	for _, v := range s.vars {
		if t.Equal(v.Type) {
			cnt++
		}
		if cnt == rand {
			return v, true
		}
	}

	panic("unreachable")
}

func (s Scope) GetRandomRangeable() (Variable, bool) {
	cnt := 0
	for _, v := range s.vars {
		if _, ok := v.Type.(ArrayType); ok {
			cnt++
		} else if t, ok := v.Type.(BasicType); ok && t.N == "string" {
			cnt++
		}
	}

	if cnt == 0 {
		return Variable{}, false
	}

	rand := 1 + s.pb.rs.Intn(cnt)
	cnt = 0
	for _, v := range s.vars {
		if _, ok := v.Type.(ArrayType); ok {
			cnt++
		} else if t, ok := v.Type.(BasicType); ok && t.N == "string" {
			cnt++
		}
		if cnt == rand {
			return v, true
		}
	}

	panic("unreachable")
}

func (s Scope) RandVarSubType(t Type) (Variable, bool) {
	vars := make([]Variable, 0, 32)
	for _, v := range s.vars {
		if v.Type.Contains(t) {
			vars = append(vars, v)
		}
	}
	if len(vars) == 0 {
		return Variable{}, false
	}
	return RandItem(s.pb.rs, vars), true
}

// return a chan (of any subtype). Useful as a replacement of
// GetRandomVarOfType when we want a channel to receive on and the
// underlying type doesn't matter.
func (s Scope) GetRandomVarChan() (Variable, bool) {
	cnt := 0
	for _, v := range s.vars {
		if _, isChan := v.Type.(ChanType); isChan {
			cnt++
		}
	}

	if cnt == 0 {
		return Variable{}, false
	}

	rand := 1 + s.pb.rs.Intn(cnt)
	cnt = 0
	for _, v := range s.vars {
		if _, isChan := v.Type.(ChanType); isChan {
			cnt++
		}
		if cnt == rand {
			return v, true
		}
	}

	panic("unreachable")
}

func FindByName(tp []Constraint, name string) Constraint {
	for i := 0; i < len(tp); i++ {
		if tp[i].N.Name == name {
			return tp[i]
		}
	}
	panic("unreachable")
}
