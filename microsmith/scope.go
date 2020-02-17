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
		fmt.Println(s)
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
		// StructTypes, ChanTypes and MapType identifiers do not depend on
		// the type contents (they are always named ST, CH, and M), so we
		// increment the counter at each Struct or Chan Type.
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

// HasType returns true if the current Scope ls has at least one
// variable which type matches exactly t.
func (ls Scope) HasType(t Type) bool {
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

// Return a random Ident of type t (exact match)
func (ls Scope) GetRandomVarOfType(t Type, rs *rand.Rand) (Variable, bool) {
	cnt := 0
	for _, v := range ls {
		if v.Type == t {
			cnt++
		}
	}

	if cnt == 0 {
		return Variable{}, false
	}

	rand := 1 + rs.Intn(cnt)
	cnt = 0
	for _, v := range ls {
		if v.Type == t {
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

	cnt := 0
	for _, v := range ls {
		switch v.Type.(type) {

		// for structs in scope, we look for fields of type t
		case StructType:
			for _, ft := range v.Type.(StructType).Ftypes {
				if ft == t {
					cnt++
				}
			}

		// for pointers, we look for the ones having base type t, since we
		// can dereference them to get a t Expr
		case PointerType:
			if v.Type.(PointerType).Base() == t {
				cnt++
			}

		// for channels, we can receive
		case ChanType:
			if v.Type.(ChanType).Base() == t {
				cnt++
			}

		// for arrays and maps, we can index
		case ArrayType:
			if v.Type.(ArrayType).Base() == t {
				cnt++
			}
		case MapType:
			if v.Type.(MapType).ValueT == t {
				cnt++
			}
		case BasicType:
			// Can't be used to derive, nothing to do
		}
	}

	if cnt == 0 {
		return Variable{}, false
	}

	rand := 1 + rs.Intn(cnt)
	cnt = 0

	for _, v := range ls {
		switch v.Type.(type) {
		case StructType:
			for _, ft := range v.Type.(StructType).Ftypes {
				if ft == t {
					cnt++
					if rand == cnt {
						return v, true
					}
				}
			}
		case PointerType:
			if v.Type.(PointerType).Base() == t {
				cnt++
				if cnt == rand {
					return v, true
				}
			}
		case ChanType:
			if v.Type.(ChanType).Base() == t {
				cnt++
				if cnt == rand {
					return v, true
				}
			}
		case ArrayType:
			if v.Type.(ArrayType).Base() == t {
				cnt++
				if cnt == rand {
					return v, true
				}
			}
		case MapType:
			if v.Type.(MapType).ValueT == t {
				cnt++
				if cnt == rand {
					return v, true
				}
			}
		case BasicType:
			// Can't be used to derive, nothing to do
		}
	}

	panic("unreachable")
}

// return a chan (of any subtype). Useful as a replacement of
// GetRandomVarOfType when we want a channel to receive on and the
// underluing type doesn't matter.
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
