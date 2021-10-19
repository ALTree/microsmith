package microsmith

import (
	"fmt"
	"go/ast"
	"math/rand"
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
type Scope []Variable

// Returns a random Addressable variable in scope, that can be used in
// the LHS of an AssignStmt. If nofunc is TRUE, ignore FuncType
// variables.
func (s Scope) RandomVar(nofunc bool) Variable {
	vs := make([]Variable, 0, 256)
	for _, v := range s {
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

	return vs[rand.Intn(len(vs))]
}

func (ls Scope) String() string {
	if len(ls) == 0 {
		return "{empty scope}"
	}
	s := "{\n"
	for i := range ls {
		s += ls[i].String() + "\n"
	}
	s = s[:len(s)-1] + "\n}"
	return s
}

// NewIdent adds to the scope a new variable of Type t, and return a
// pointer to it
func (s *Scope) NewIdent(t Type) *ast.Ident {
	tc := 0
	switch t.(type) {
	case FuncType:
		for _, v := range *s {
			if ft, ok := v.Type.(FuncType); ok && ft.Local {
				tc++
			}
		}

	// StructType, ChanType, MapType, and ArrayType identifiers do not
	// depend on the type contents (they are always named ST, CH, and
	// M), so we increment the counter at each Struct or Chan Type.

	case StructType:
		for _, v := range *s {
			if _, ok := v.Type.(StructType); ok {
				tc++
			}
		}
	case ChanType:
		for _, v := range *s {
			if _, ok := v.Type.(ChanType); ok {
				tc++
			}
		}
	case MapType:
		for _, v := range *s {
			if _, ok := v.Type.(MapType); ok {
				tc++
			}
		}
	case ArrayType:
		for _, v := range *s {
			if _, ok := v.Type.(ArrayType); ok {
				tc++
			}
		}

	case PointerType:
		for _, v := range *s {
			if _, ok := v.Type.(PointerType); ok {
				tc++
			}
		}

	default:
		for _, v := range *s {
			if v.Type.Equal(t) {
				tc++
			}
		}
	}

	name := fmt.Sprintf("%s%v", Ident(t), tc)
	id := &ast.Ident{Name: name}

	*s = append(*s, Variable{t, id})
	return id
}

// Adds v to the scope.
func (s *Scope) AddVariable(i *ast.Ident, t Type) {
	*s = append(*s, Variable{t, i})
}

func (s *Scope) DeleteIdentByName(name *ast.Ident) {
	del := -1
	for i := range *s {
		if v := (*s)[i]; v.Name.Name == name.Name {
			del = i
			break
		}
	}

	if del != -1 {
		*s = append((*s)[:del], (*s)[del+1:]...)
	}
}

// HasType returns true if the current Scope ls has at least one
// variable which type matches exactly t.
func (ls Scope) HasType(t Type) bool {
	for _, v := range ls {
		if v.Type.Equal(t) {
			return true
		}
	}
	return false
}

// Returns a function with return type t
func (ls Scope) GetRandomFunc(t Type) (Variable, bool) {
	funcs := make([]Variable, 0, 32)
	for _, v := range ls {
		if ft, ok := v.Type.(FuncType); ok && ft.Ret[0].Equal(t) {
			funcs = append(funcs, v)
		}
	}
	if len(funcs) == 0 {
		return Variable{}, false
	}
	return funcs[rand.Intn(len(funcs))], true
}

// Returns a random function in scope; but not a predefined one.
func (ls Scope) GetRandomFuncAnyType() (Variable, bool) {
	funcs := make([]Variable, 0, 32)
	for _, v := range ls {
		if t, ok := v.Type.(FuncType); ok && t.Local {
			funcs = append(funcs, v)
		}
	}
	if len(funcs) == 0 {
		return Variable{}, false
	}
	return funcs[rand.Intn(len(funcs))], true
}

// Return a random Variable of type t (exact match)
func (ls Scope) GetRandomVarOfType(t Type, rs *rand.Rand) (Variable, bool) {
	cnt := 0
	for _, v := range ls {
		if t.Equal(v.Type) {
			cnt++
		}
	}

	if cnt == 0 {
		return Variable{}, false
	}

	rand := 1 + rs.Intn(cnt)
	cnt = 0
	for _, v := range ls {
		if t.Equal(v.Type) {
			cnt++
		}
		if cnt == rand {
			return v, true
		}
	}

	panic("unreachable")
}

func (ls Scope) GetRandomRangeable(rs *rand.Rand) (Variable, bool) {
	cnt := 0
	for _, v := range ls {
		if _, ok := v.Type.(ArrayType); ok {
			cnt++
		} else if t, ok := v.Type.(BasicType); ok && t.N == "string" {
			cnt++
		}
	}

	if cnt == 0 {
		return Variable{}, false
	}

	rand := 1 + rs.Intn(cnt)
	cnt = 0
	for _, v := range ls {
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

func (s Scope) RandVarSubType(t Type, rs *rand.Rand) (Variable, bool) {
	vars := make([]Variable, 0, 32)
	for _, v := range s {
		if v.Type.Contains(t) {
			vars = append(vars, v)
		}
	}
	if len(vars) == 0 {
		return Variable{}, false
	}
	return vars[rs.Intn(len(vars))], true
}

// return a chan (of any subtype). Useful as a replacement of
// GetRandomVarOfType when we want a channel to receive on and the
// underlying type doesn't matter.
func (ls Scope) GetRandomVarChan(rs *rand.Rand) (Variable, bool) {
	cnt := 0
	for _, v := range ls {
		if _, isChan := v.Type.(ChanType); isChan {
			cnt++
		}
	}

	if cnt == 0 {
		return Variable{}, false
	}

	rand := 1 + rs.Intn(cnt)
	cnt = 0
	for _, v := range ls {
		if _, isChan := v.Type.(ChanType); isChan {
			cnt++
		}
		if cnt == rand {
			return v, true
		}
	}

	panic("unreachable")
}

// TypeParams holds a list of all the type parameters interfaces that
// are available to the function in the package
type TypeParams []TypeParam

func (tp TypeParams) FindByName(name string) TypeParam {
	for i := 0; i < len(tp); i++ {
		if tp[i].Name.Name == name {
			return tp[i]
		}
	}
	panic("unreachable")
}
