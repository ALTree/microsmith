package microsmith

// Context holds all the contextual information needed while
// generating a random program.
type Context struct {
	programConf ProgramConf  // program-wide settings for the fuzzer
	constraints []Constraint // list of all the Costraints available to the package

	// package-wide scope of vars and func available to the code in a
	// given moment
	scope *Scope

	// function-level scope of the type parameters available to the
	// code in the body
	types *Scope
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
