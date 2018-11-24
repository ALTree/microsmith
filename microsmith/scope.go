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
		panic("NewIdent: not for building functions")
	case StructType:
		// we increment at every struct var, even if technically they
		// are not the same type
		for _, v := range *s {
			if _, ok := v.Type.(StructType); ok {
				tc++
			}
		}
	default:
		for _, v := range *s {
			if v.Type == t {
				tc++
			}
		}
	}
	name := fmt.Sprintf("%s%v", Ident(t), tc)

	// build Ident object
	id := new(ast.Ident)
	id.Obj = &ast.Object{Kind: ast.Var, Name: name}
	id.Name = name

	*s = append(*s, Variable{t, id})

	return id
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

// TypeInScope returns true if at least one variable of Type t is
// currently in scope.
func (ls Scope) TypeInScope(t Type) bool {
	for _, v := range ls {
		switch v.Type.(type) {
		case StructType:
			for _, ft := range v.Type.(StructType).Ftypes {
				if ft == t {
					return true
				}
			}
		default:
			if v.Type == t {
				return true
			}
		}
	}
	return false
}

// InScopeTypes returns a list of Types that have at least one
// variable currently in scope
func (ls Scope) InScopeTypes() []Type {
	tMap := make(map[Type]struct{})
	for _, v := range ls {
		switch v.Type.(type) {
		case FuncType:
			// functions are handled differently
			continue
		case StructType:
			for _, t := range v.Type.(StructType).Ftypes {
				tMap[t] = struct{}{}
			}
		default:
			tMap[v.Type] = struct{}{}
		}
	}

	tArr := make([]Type, 0, len(tMap))
	for t := range tMap {
		tArr = append(tArr, t)
	}

	return tArr
}

// returns a list of function that are in scope and have return type t
func (ls Scope) InScopeFuncs(t Type) []Variable {
	funcs := make([]Variable, 0)
	for _, v := range ls {
		switch v.Type.(type) {
		case FuncType:
			if v.Type.(FuncType).Ret[0] == t {
				funcs = append(funcs, v)
			}
		default:
			continue
		}
	}

	return funcs
}

// return an expression made of an ident of the given type
func (ls Scope) RandomIdentExpr(t Type, rs *rand.Rand) ast.Expr {

	// we'll collect expressions of two types:
	//   1. simple ast.Expr wrapping an ast.Ident
	//   2. ast.SelectorExpr wrapping a struct field access
	//
	// To reduce allocations, instead of looping over every suitable
	// ident collecting ast.Expr and then choosing one at random, we
	// make a first pass just counting the number of idents of the
	// requested type, then we draw a random number, and finally we
	// make a last pass over the idents, returning a new expression
	// when we reach the one at the position selected by the random
	// number.

	cnt := 0
	for _, v := range ls {
		switch v.Type.(type) {
		case StructType:
			for _, ft := range v.Type.(StructType).Ftypes {
				if ft == t {
					cnt++
				}
			}
		default:
			if v.Type == t {
				cnt++
			}
		}
	}

	// it's up the the caller to make sure the scope is not empty
	if cnt == 0 {
		panic("Empty scope")
	}

	rand := 1 + rs.Intn(cnt)
	cnt = 0
	for _, v := range ls {
		switch v.Type.(type) {
		case StructType:
			for i, ft := range v.Type.(StructType).Ftypes {
				if ft == t {
					cnt++
				}
				if cnt == rand {
					return &ast.SelectorExpr{
						X:   v.Name,
						Sel: &ast.Ident{Name: v.Type.(StructType).Fnames[i]},
					}
				}
			}
		default:
			if v.Type == t {
				cnt++
			}
			if cnt == rand {
				return v.Name
			}
		}
	}

	// should never happen
	panic("something went wrong when counting idents")

}
