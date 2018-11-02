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

// DeleteIdent deletes the id-th Ident of type kind from the scope.
// If id < 0, it deletes the last one that was declared.
func (s *Scope) DeleteIdent(t Type, id int) {
	var lastI int = -1
	for i := range *s {
		if v := (*s)[i]; v.Type == t {
			lastI = i
		}
	}

	if lastI != -1 {
		*s = append((*s)[:lastI], (*s)[lastI+1:]...)
	}
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
		if v.Type == t {
			return true
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

// RandomIdent returns a random in-scope identifier of type t. It
// panics if no variable of Type t is in scope.
func (ls Scope) RandomIdent(t Type, rs *rand.Rand) *ast.Ident {
	ts := make([]Variable, 0)
	for _, v := range ls {
		if v.Type == t {
			ts = append(ts, v)
		}
	}

	if len(ts) == 0 {
		panic("Empty scope")
	}

	return ts[rs.Intn(len(ts))].Name
}

// return an expression made of an ident of the given type
func (ls Scope) RandomIdentExpr(t Type, rs *rand.Rand) ast.Expr {

	// we'll collect expressions of two types:
	//   1. simple ast.Expr wrapping an ast.Ident
	//   2. ast.SelectorExpr wrapping a struct field access
	exprs := make([]ast.Expr, 0)
	for _, v := range ls {
		switch v.Type.(type) {
		case StructType:
			// loop over the struct's fields, collect selector exprs
			// for the one of type t
			for i, ft := range v.Type.(StructType).Ftypes {
				if ft == t {
					exprs = append(
						exprs,
						&ast.SelectorExpr{
							X:   v.Name,
							Sel: &ast.Ident{Name: v.Type.(StructType).Fnames[i]},
						},
					)
				}
			}
		default:
			// simple variable, wrap it in an ast.Expr
			if v.Type == t {
				exprs = append(exprs, v.Name)
			}
		}
	}

	if len(exprs) == 0 {
		panic("Empty scope")
	}

	return exprs[rs.Intn(len(exprs))]
}
