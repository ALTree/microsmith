package microsmith

import (
	"fmt"
	"go/ast"
	"go/token"
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

func (s Scope) RandomVar(addressable bool) Variable {

	vs := make([]Variable, 0, 16)
	for _, v := range s {
		if addressable {
			if Addressable(v.Type) {
				vs = append(vs, v)
			}
		} else {
			vs = append(vs, v)
		}
	}

	if len(vs) == 0 {
		if addressable {
			panic("RandomVar: no addressable variable in scope")
		} else {
			panic("RandomVar: no variable in scope")
		}
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
		panic("NewIdent: not for building functions")
	case StructType:
		// we increment at every struct var, even if technically they
		// are not the same type
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
	default:
		for _, v := range *s {
			if v.Type == t {
				tc++
			}
		}
	}

	name := fmt.Sprintf("%s%v", Ident(t), tc)
	id := &ast.Ident{Name: name}

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
		case ChanType:
			if v.Type.(ChanType).Base() == t {
				return true
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
		case ChanType:
			tMap[v.Type.(ChanType).Base()] = struct{}{}
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

// returns a list of function that are in scope and have return type t.
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

// Used by ExprStmt() (which is currently disabled).
func (ls Scope) InScopeFuncsReal(t Type) []Variable {
	funcs := make([]Variable, 0)
	for _, v := range ls {
		switch v.Type.(type) {
		case FuncType:
			if v.Type.(FuncType).Ret[0] == t &&
				(v.Name.Name != "len" && v.Name.Name != "int" && v.Name.Name != "float64") {
				funcs = append(funcs, v)
			}
		default:
			continue
		}
	}

	return funcs
}

// return a list of in-scope channel variables
func (ls Scope) InScopeChans() []Variable {
	chans := make([]Variable, 0)
	for _, v := range ls {
		switch v.Type.(type) {
		case ChanType:
			chans = append(chans, v)
		}
	}

	return chans
}

// Returns an expression made of an ident of the given type.
//
// If addr is true, it's required that the returned Expr will be
// addressable, i.e. it'll be allowed to &() it. In practise, this is
// used to exclude chan expressions, since they're not addressable.
func (ls Scope) RandomIdentHelper(t Type, rs *rand.Rand, addr bool) ast.Expr {

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
		case PointerType:
			if v.Type == t {
				cnt++
			} else if v.Type.(PointerType).Base() == t {
				cnt++
			}
		case ChanType:
			if v.Type == t {
				cnt++
			} else if v.Type.(ChanType).Base() == t {
				if !addr {
					// not required to return an addressable Expr, so ok to
					// return a <- expression
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
		case PointerType:
			// pointers in scope are useful in two cases:
			//   1. when the caller requested a pointer ident
			//   2. when the caller requested a type that is the base
			//   type of the pointer
			// In the first case, we count the pointer ident itself, in
			// the second case we'll return *p
			if v.Type == t {
				cnt++
				if cnt == rand {
					return v.Name
				}
			} else if v.Type.(PointerType).Base() == t {
				cnt++
				if cnt == rand {
					return &ast.UnaryExpr{
						Op: token.MUL,
						X:  &ast.Ident{Name: v.Name.Name},
					}
				}
			}
		case ChanType:
			if v.Type == t {
				cnt++
				if cnt == rand {
					return v.Name
				}
			} else if !addr && v.Type.(ChanType).Base() == t {
				cnt++
				if cnt == rand {
					return &ast.UnaryExpr{
						Op: token.ARROW,
						X:  &ast.Ident{Name: v.Name.Name},
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

func (ls Scope) RandomIdentExpr(t Type, rs *rand.Rand) ast.Expr {
	return ls.RandomIdentHelper(t, rs, false)
}

// We need this e.g. when calling for &(Expr()); but we also use it
// for AssignStmt, since you can't assign to <-CH
func (ls Scope) RandomIdentExprAddressable(t Type, rs *rand.Rand) ast.Expr {
	return ls.RandomIdentHelper(t, rs, true)
}
