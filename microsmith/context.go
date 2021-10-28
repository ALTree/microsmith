package microsmith

import (
	"math/rand"
	"strconv"
)

// Context holds all the contextual information needed while
// generating a random program.
type Context struct {

	// program-wide settings for the fuzzer
	programConf ProgramConf

	// all the Costraints available declared in the program
	constraints []Constraint

	// package-wide scope of vars and func available to the code in a
	// given moment
	scope *Scope

	// Function-level scope of the type parameters available to the
	// code in the body. Reset and re-filled in when declaring a new
	// function.
	typeparams *Scope
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
}

// --------------------------------
//   Types Randomizers
// --------------------------------

// Returns a slice of n random types, including composite types
// (structs, array, maps, chans).
func (pb ProgramBuilder) RandTypes(n int) []Type {
	types := make([]Type, n)
	for i := 0; i < n; i++ {
		types[i] = pb.RandType(false)
	}
	return types
}

// Returns a single random type (including structs, array, maps,
// chans).
func (pb ProgramBuilder) RandType(comp bool) Type {
	switch pb.rs.Intn(15) {
	case 0, 1:
		if comp {
			return pb.RandBaseType()
		} else {
			return ArrayOf(pb.RandType(true))
		}
	case 2:
		return ChanOf(pb.RandType(true))
	case 3, 4:
		return MapOf(
			pb.RandBaseType(), // map keys need to be comparable
			pb.RandType(true),
		)
	case 5, 6:
		return PointerOf(pb.RandType(true))
	case 7, 8:
		return pb.RandStructType(comp)
	case 9:
		return pb.RandFuncType()
	default:
		return pb.RandBaseType()
	}
}

// Returns a single BaseType (primitives, or a type parameter).
func (pb ProgramBuilder) RandBaseType() Type {
	if tp := pb.ctx.typeparams; tp != nil {
		i := pb.rs.Intn(len(AllTypes) + len(*tp))
		if i < len(AllTypes) {
			return AllTypes[i]
		} else {
			return MakeTypeParam((*tp)[i-len(AllTypes)])
		}
	} else {
		return AllTypes[rand.Intn(len(AllTypes))]
	}
}

func (pb ProgramBuilder) RandStructType(comparable bool) StructType {
	st := StructType{"ST", []Type{}, []string{}}
	for i := 0; i < pb.rs.Intn(6); i++ {
		t := pb.RandType(true)
		st.Ftypes = append(st.Ftypes, t)
		st.Fnames = append(st.Fnames, Ident(t)+strconv.Itoa(i))
	}
	return st
}

func (pb ProgramBuilder) RandFuncType() FuncType {
	args := make([]Type, 0, pb.rs.Intn(5))

	// arguments
	for i := 0; i < cap(args); i++ {
		args = append(args, pb.RandType(true))
	}

	// return type
	ret := []Type{pb.RandType(true)}

	return FuncType{"FU", args, ret, true}
}
