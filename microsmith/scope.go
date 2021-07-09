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

// Scope in array holding all the variables that are in scope in a
// given moment
type Scope []Variable

// Returns a random Addressable variable in scope, that can be used in
// the LHS of an AssignStmt. If nofunc is TRUE, ignore FuncType
// variables.
func (s Scope) RandomVar(nofunc bool) Variable {

	vs := make([]Variable, 0, 16)
	for _, v := range s {
		if Addressable(v.Type) {
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
		switch v.Type.(type) {
		case FuncType:
			if v.Type.(FuncType).Ret[0].Equal(t) {
				funcs = append(funcs, v)
			}
		default:
			continue
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
		switch v.Type.(type) {
		case FuncType:
			if v.Type.(FuncType).Local {
				funcs = append(funcs, v)
			}
		default:
			continue
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

// Like GetExprOfType, but it's *required* to return a variable from
// which we can derive an expression of type t (by indexing into
// arrays and maps, selecting into structs, receiving from a chan and
// dereferencing pointers).
func (ls Scope) GetRandomVarOfSubtype(t Type, rs *rand.Rand) (Variable, bool) {

	vars := make([]Variable, 0, 32)

	for _, v := range ls {
		switch v.Type.(type) {

		// for structs in scope, we look for fields of type t
		case StructType:
			for _, ft := range v.Type.(StructType).Ftypes {
				if ft.Equal(t) {
					vars = append(vars, v)
				}
			}

		// for pointers, we look for the ones having base type t, since we
		// can dereference them to get a t Expr
		case PointerType:
			if v.Type.(PointerType).Base().Equal(t) {
				vars = append(vars, v)
			}

		// for channels, we can receive
		case ChanType:
			if v.Type.(ChanType).Base().Equal(t) {
				vars = append(vars, v)
			}

		// for arrays and maps, we can index
		case ArrayType:
			if v.Type.(ArrayType).Base().Equal(t) {
				vars = append(vars, v)
			}
		case MapType:
			if v.Type.(MapType).ValueT.Equal(t) {
				vars = append(vars, v)
			}
		case BasicType:
			if t.Name() != "byte" {
				continue
			}
			if v.Type.Name() == "string" {
				vars = append(vars, v)
			}
		}
	}

	if len(vars) == 0 {
		return Variable{}, false
	}

	return vars[rs.Intn(len(vars))], true
}

func (s Scope) RandVarSubType(t Type, rs *rand.Rand) (Variable, bool) {
	vars := make([]Variable, 0, 32) // TODO(alb): bigger

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
