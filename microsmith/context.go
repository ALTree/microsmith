package microsmith

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
