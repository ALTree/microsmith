package microsmith_test

import (
	"go/ast"
	"math/rand"
	"testing"

	"github.com/ALTree/microsmith/microsmith"
)

const WorkDir = "../work/"

var TestConfigurations = map[string]microsmith.ProgramConf{
	"small": {
		microsmith.StmtConf{
			MaxStmtDepth:  1,
			MaxBlockVars:  2,
			MaxBlockStmts: 1,
		},
		microsmith.ExprConf{
			ExprKindChance: []float64{
				1, 1, 1,
			},
			LiteralChance: 0.2,
			IndexChance:   0.1,
		},
		[]microsmith.Type{
			microsmith.BasicType{"int"},
			microsmith.BasicType{"float64"},
			microsmith.BasicType{"complex128"},
			microsmith.BasicType{"bool"},
			microsmith.BasicType{"string"},
		},
	},

	"medium": {
		microsmith.StmtConf{
			MaxStmtDepth:  2,
			MaxBlockVars:  6,
			MaxBlockStmts: 4,
		},
		microsmith.ExprConf{
			ExprKindChance: []float64{
				1, 1, 1,
			},
			LiteralChance: 0.2,
			IndexChance:   0.1,
		},
		[]microsmith.Type{
			microsmith.BasicType{"int"},
			microsmith.BasicType{"float64"},
			microsmith.BasicType{"complex128"},
			microsmith.BasicType{"bool"},
			microsmith.BasicType{"string"},
		},
	},

	"big": {
		microsmith.StmtConf{
			MaxStmtDepth:  3,
			MaxBlockVars:  12,
			MaxBlockStmts: 8,
		},
		microsmith.ExprConf{
			ExprKindChance: []float64{
				1, 1, 1,
			},
			LiteralChance: 0.2,
			IndexChance:   0.1,
		},
		[]microsmith.Type{
			microsmith.BasicType{"int"},
			microsmith.BasicType{"float64"},
			microsmith.BasicType{"complex128"},
			microsmith.BasicType{"bool"},
			microsmith.BasicType{"string"},
		},
	},
}

// check n generated programs with go/types (in-memory)
func testProgramGoTypes(t *testing.T, n int, conf microsmith.ProgramConf) {
	rand := rand.New(rand.NewSource(42))
	for i := 0; i < n; i++ {
		gp, err := microsmith.NewProgram(rand.Int63(), conf)
		if err != nil {
			t.Fatalf("BadConf error: %s\n", err)
		}
		err = gp.Check()
		if err != nil {
			t.Fatalf("Program failed typechecking: %s\n%s", err, gp)
		}

		checkStats(t, gp)
	}
}

func TestDefault(t *testing.T) {
	testProgramGoTypes(t, 50, microsmith.DefaultConf)
}

func TestRandConf(t *testing.T) {
	lim := 50
	if testing.Short() {
		lim = 10
	}
	for i := 0; i < lim; i++ {
		conf := microsmith.RandConf()
		// leave this (useful for debugging)
		//fmt.Printf("%+v\n\n", conf)
		testProgramGoTypes(t, 10, conf)
	}
}

func TestSmall(t *testing.T) {
	lim := 100
	if testing.Short() {
		lim = 10
	}
	testProgramGoTypes(t, lim, TestConfigurations["small"])
}

func TestMedium(t *testing.T) {
	lim := 100
	if testing.Short() {
		lim = 10
	}
	testProgramGoTypes(t, lim, TestConfigurations["medium"])
}

func TestBig(t *testing.T) {
	testProgramGoTypes(t, 10, TestConfigurations["big"])
}

func TestAllLiterals(t *testing.T) {
	tc := TestConfigurations["medium"]
	tc.LiteralChance = 1.0
	testProgramGoTypes(t, 100, tc)
}

func TestAllUnary(t *testing.T) {
	tc := TestConfigurations["medium"]
	tc.ExprKindChance = []float64{1.0, 0, 0}
	testProgramGoTypes(t, 100, tc)
}

func TestSingleType(t *testing.T) {
	tc := TestConfigurations["medium"]
	for _, typ := range []string{"bool", "int", "string", "float64", "complex128"} {
		t.Run(typ, func(t *testing.T) {
			tc.SupportedTypes = []microsmith.Type{microsmith.BasicType{typ}}
			testProgramGoTypes(t, 100, tc)
		})
	}
}

func testBadConf(t *testing.T, conf microsmith.ProgramConf) {
	_, err := microsmith.NewProgram(rand.Int63(), conf)
	if err == nil {
		t.Errorf("Expected bad conf error for\n%+v\n", conf)
	}
}

func TestBadConfs(t *testing.T) {
	// IndexChance = 1 and LiteralChance = 0
	tc := TestConfigurations["medium"]
	tc.IndexChance = 1
	tc.LiteralChance = 0
	testBadConf(t, tc)

	// all zero ExprKindChance
	tc = TestConfigurations["medium"]
	for i := range tc.ExprKindChance {
		tc.ExprKindChance[i] = 0.0
	}
	testBadConf(t, tc)
}

func TestStmtStats(t *testing.T) {
	rand := rand.New(rand.NewSource(444))
	for i := 0; i < 100; i++ {
		gp, _ := microsmith.NewProgram(rand.Int63(), microsmith.DefaultConf)
		checkStats(t, gp)
	}
}

func checkStats(t *testing.T, p *microsmith.Program) {
	ss := p.Stats.Stmt
	sum := float64(ss.Branch +
		ss.Block +
		ss.For +
		ss.If +
		ss.Switch +
		ss.Send +
		ss.Select)

	// not enough statements to do a statistical check
	if sum < 100 {
		return
	}

	c := 0.01
	if (float64(ss.Assign)/sum < c) ||
		(float64(ss.Block)/sum < c) ||
		(float64(ss.For)/sum < c) ||
		(float64(ss.If)/sum < c) ||
		(float64(ss.Switch)/sum < c) ||
		(float64(ss.Send)/sum < c) ||
		(float64(ss.Select)/sum < c) {
		t.Errorf("At least one Stmt has low count\n%+v\n", ss)
	}

}

// Check generated programs with go tool compile (from file). This is
// much slower than using go/types.
func TestProgramGc(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	rand := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		gp, _ := microsmith.NewProgram(rand.Int63(), microsmith.DefaultConf)
		err := gp.WriteToFile(WorkDir)
		if err != nil {
			t.Fatalf("Could not write to file: %s", err)
		}
		out, err := gp.Compile("go", "amd64", false, false, false)
		if err != nil {
			t.Fatalf("Program did not compile: %s\n%s\n%s", out, err, gp)
		}
		gp.DeleteFile()
	}
}

var BenchConf = microsmith.ProgramConf{
	microsmith.StmtConf{
		MaxStmtDepth:  2,
		MaxBlockVars:  8,
		MaxBlockStmts: 4,
	},
	microsmith.ExprConf{
		ExprKindChance: []float64{
			1, 1, 1,
		},
		LiteralChance: 0.4,
		IndexChance:   0.25,
	},
	[]microsmith.Type{
		microsmith.BasicType{"int"},
		microsmith.BasicType{"float64"},
		microsmith.BasicType{"complex128"},
		microsmith.BasicType{"bool"},
		microsmith.BasicType{"string"},
		microsmith.BasicType{"rune"},
	},
}

var gp *ast.File

func BenchmarkProgramGeneration(b *testing.B) {
	b.ReportAllocs()
	rand := rand.New(rand.NewSource(19))
	for i := 0; i < b.N; i++ {
		db := microsmith.NewDeclBuilder(rand.Int63(), BenchConf)
		gp = db.File("main", 1)
	}
}
