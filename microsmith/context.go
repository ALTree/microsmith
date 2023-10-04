package microsmith

import (
	"go/ast"
	"math/rand"
	"strconv"
	"strings"
)

// Context holds all the contextual information needed while
// generating a random package.
type Context struct { // TODO(alb): rename to PackageContext

	// program-wide settings for the fuzzer
	programConf ProgramConf

	// all the Costraints available declared in the package
	constraints []Constraint

	// package-wide scope of vars and func available to the code in a
	// given moment
	scope *Scope

	// Function-level scope of the type parameters available to the
	// code in the body. Reset and re-filled in when declaring a new
	// function.
	typeparams *Scope

	// Wheter we are building a loop body or the argument of a defer
	// statement.
	inLoop, inDefer bool
}

func NewContext(pc ProgramConf) *Context {
	return &Context{
		programConf: pc,
	}
}

// ProgramConf holds program-wide configuration settings that change
// the kind of programs that are generated.
type ProgramConf struct {
	MultiPkg   bool // for -multipkg
	TypeParams bool // for -tp
	ExpRange   bool // for new range types
}

// --------------------------------
//   Randomizers
// --------------------------------

func RandItem[T any](r *rand.Rand, a []T) T {
	if r == nil {
		panic("RandItem: nil rand.Rand")
	}
	return a[r.Intn(len(a))]
}

// --------------------------------
//   Types Randomizers
// --------------------------------

// Returns a slice of n random types, including composite types
// (structs, array, maps, chans).
func (pb PackageBuilder) RandTypes(n int) []Type {
	types := make([]Type, n)
	for i := 0; i < n; i++ {
		types[i] = pb.RandType()
	}
	return types
}

// Returns a single random type (including structs, array, maps,
// chans).
func (pb PackageBuilder) RandType() Type {
	pb.typedepth++
	defer func() { pb.typedepth-- }()

	if pb.typedepth >= 5 {
		return pb.RandBaseType()
	}

	switch pb.rs.Intn(15) {
	case 0, 1:
		return ArrayOf(pb.RandType())
	case 2:
		return ChanOf(pb.RandType())
	case 3, 4:
		return MapOf(
			pb.RandComparableType(),
			pb.RandType(),
		)
	case 5, 6:
		return PointerOf(pb.RandType())
	case 7, 8:
		return pb.RandStructType()
	case 9:
		return pb.RandFuncType()
	case 10:
		return pb.RandInterfaceType()
	default:
		return pb.RandBaseType()
	}
}

func (pb PackageBuilder) RandComparableType() Type {
	types := make([]Type, 0, 32)

	// from Base Types
	for _, t := range pb.baseTypes {
		if t.Comparable() {
			types = append(types, t)
		}
	}

	// from type parameters
	if tp := pb.ctx.typeparams; tp != nil {
		for _, v := range tp.vars {
			if v.Type.Comparable() {
				types = append(types, MakeTypeParam(v))
			}
		}
	}

	return RandItem(pb.rs, types)
}

// Returns a single BaseType (primitives, or a type parameter).
func (pb PackageBuilder) RandBaseType() Type {
	if tp := pb.ctx.typeparams; tp != nil {
		i := pb.rs.Intn(len(pb.baseTypes) + len(tp.vars))
		if i < len(pb.baseTypes) {
			return pb.baseTypes[i]
		} else {
			return MakeTypeParam((tp.vars)[i-len(pb.baseTypes)])
		}
	} else {
		return RandItem(pb.rs, pb.baseTypes)
	}
}

func (pb PackageBuilder) RandNumericType() BasicType {
	t := RandItem(pb.rs, pb.baseTypes)
	for !IsNumeric(t) {
		t = RandItem(pb.rs, pb.baseTypes)
	}
	return t.(BasicType)
}

func (pb PackageBuilder) RandStructType() StructType {
	st := StructType{[]Type{}, []string{}}
	for i := 0; i < pb.rs.Intn(6); i++ {
		t := pb.RandType()
		st.Ftypes = append(st.Ftypes, t)
		// we want structs fields to be exported, capitalize the names
		st.Fnames = append(st.Fnames, strings.Title(Ident(t))+strconv.Itoa(i))
	}
	return st
}

func (pb PackageBuilder) RandFuncType() FuncType {
	args := make([]Type, 0, pb.rs.Intn(8))

	// arguments
	for i := 0; i < cap(args); i++ {
		args = append(args, pb.RandType())
	}

	// optionally make the last parameter variadic
	if len(args) > 0 && pb.rs.Intn(4) == 0 {
		args[len(args)-1] = EllipsisType{Base: args[len(args)-1]}
	}

	// return type
	ret := []Type{pb.RandType()}

	return FuncType{"FU", args, ret, true}
}

func (pb PackageBuilder) RandInterfaceType() InterfaceType {
	var in InterfaceType
	for i := 0; i < pb.rs.Intn(4); i++ {
		t := pb.RandFuncType()
		in.Methods = append(in.Methods, Method{&ast.Ident{Name: "M" + strconv.Itoa(i)}, t})
	}
	return in
}
