package microsmith_test

import (
	"go/ast"
	"math/rand"
	"os"
	"testing"

	"github.com/ALTree/microsmith/microsmith"
)

const WorkDir = "work"

var TestConfigurations = map[string]microsmith.ProgramConf{
	"small": {
		microsmith.StmtConf{
			MaxStmtDepth: 1,
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
			MaxStmtDepth: 2,
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
			MaxStmtDepth: 3,
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
	for i := 0; i < n; i++ {
		gp, err := microsmith.NewProgram(rand.New(rand.NewSource(42)), conf)
		if err != nil {
			t.Fatalf("BadConf error: %s\n", err)
		}
		err = gp.Check()
		if err != nil {
			t.Fatalf("Program failed typechecking with error:\n%s", err)
		}

		checkStats(t, gp)
	}
}

func TestRandConf(t *testing.T) {
	lim := 50
	if testing.Short() {
		lim = 20
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

func TestSingleType(t *testing.T) {
	tc := TestConfigurations["medium"]
	for _, typ := range []string{
		"bool", "int", "rune", "string",
		"float64", "complex128",
	} {
		t.Run(typ, func(t *testing.T) {
			tc.SupportedTypes = []microsmith.Type{microsmith.BasicType{typ}}
			testProgramGoTypes(t, 100, tc)
		})
	}
}

func TestStmtStats(t *testing.T) {

	for i := 0; i < 100; i++ {
		gp, _ := microsmith.NewProgram(
			rand.New(rand.NewSource(444)),
			microsmith.RandConf(),
		)
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

	if _, err := os.Stat(WorkDir); os.IsNotExist(err) {
		err := os.MkdirAll(WorkDir, os.ModePerm)
		if err != nil {
			t.Fatalf("%v", err)
		}
	}
	defer func() {
		os.RemoveAll(WorkDir)
	}()

	rand := rand.New(rand.NewSource(42))
	for i := 0; i < 50; i++ {
		gp, _ := microsmith.NewProgram(rand, microsmith.RandConf())
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
		MaxStmtDepth: 2,
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

func BenchmarkProgram(b *testing.B) {
	b.ReportAllocs()
	rand := rand.New(rand.NewSource(19))
	for i := 0; i < b.N; i++ {
		db := microsmith.NewDeclBuilder(rand, BenchConf)
		gp = db.File("main", 1)
	}
}
