package microsmith_test

import (
	"go/ast"
	"io/ioutil"
	"math/rand"
	"os"
	"runtime"
	"testing"

	"github.com/ALTree/microsmith/microsmith"
)

const WorkDir = "work"

var allTypes = []microsmith.Type{
	microsmith.BasicType{"bool"},
	microsmith.BasicType{"byte"},
	microsmith.BasicType{"int"},
	microsmith.BasicType{"int8"},
	microsmith.BasicType{"int16"},
	microsmith.BasicType{"int32"},
	microsmith.BasicType{"int64"},
	microsmith.BasicType{"uint"},
	microsmith.BasicType{"float32"},
	microsmith.BasicType{"float64"},
	microsmith.BasicType{"complex128"},
	microsmith.BasicType{"rune"},
	microsmith.BasicType{"string"},
}

var TestConfigurations = map[string]microsmith.ProgramConf{
	"small": {
		microsmith.StmtConf{
			MaxStmtDepth: 1,
		},
		allTypes,
	},

	"medium": {
		microsmith.StmtConf{
			MaxStmtDepth: 2,
		},
		allTypes,
	},

	"big": {
		microsmith.StmtConf{
			MaxStmtDepth: 3,
		},
		allTypes,
	},
	"huge": {
		microsmith.StmtConf{
			MaxStmtDepth: 5,
		},
		allTypes,
	},
}

// check n generated programs with go/types (in-memory)
func testProgramGoTypes(t *testing.T, n int, conf microsmith.ProgramConf) {
	for i := 0; i < n; i++ {
		gp := microsmith.NewProgram(rand.New(rand.NewSource(42)), conf)
		err := gp.Check()
		if err != nil {
			tmpfile, _ := ioutil.TempFile("", "fail*.go")
			if _, err := tmpfile.Write([]byte(gp.String())); err != nil {
				t.Fatal(err)
			}
			t.Fatalf("Program failed typechecking with error:\n%s", err)
		}

		checkStats(t, gp)
	}
}

func TestRandConf(t *testing.T) {
	lim := 50
	if testing.Short() {
		lim = 5
	}
	for i := 0; i < lim; i++ {
		conf := microsmith.RandConf(rand.New(rand.NewSource(42)))
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
	lim := 10
	if testing.Short() {
		lim = 2
	}
	testProgramGoTypes(t, lim, TestConfigurations["big"])
}

func TestHuge(t *testing.T) {
	lim := 5
	if testing.Short() {
		lim = 1
	}
	testProgramGoTypes(t, lim, TestConfigurations["huge"])
}

func TestSingleType(t *testing.T) {
	tc := TestConfigurations["medium"]
	for _, typ := range microsmith.AllTypes {
		t.Run(typ.Name(), func(t *testing.T) {
			tc.SupportedTypes = []microsmith.Type{typ}
			testProgramGoTypes(t, 10, tc)
		})
	}
}

func TestStmtStats(t *testing.T) {
	for i := 0; i < 50; i++ {
		rs := rand.New(rand.NewSource(444))
		gp := microsmith.NewProgram(rs, microsmith.RandConf(rs))
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
		t.Logf("At least one Stmt has low count\n%+v\n", ss)
	}

}

// Check generated programs with go tool compile (from file). This is
// much slower than using go/types.
func TestProgramGc(t *testing.T) {
	if testing.Short() || runtime.GOOS == "windows" {
		t.Skip()
	}

	if _, err := os.Stat(WorkDir); os.IsNotExist(err) {
		err := os.MkdirAll(WorkDir, os.ModePerm)
		if err != nil {
			t.Fatalf("%v", err)
		}
	}
	rand := rand.New(rand.NewSource(42))
	keepdir := false
	for i := 0; i < 50; i++ {
		gp := microsmith.NewProgram(rand, microsmith.RandConf(rand))
		err := gp.WriteToFile(WorkDir)
		if err != nil {
			t.Fatalf("Could not write to file: %s", err)
		}
		fz := microsmith.FuzzOptions{
			"go",
			false, false, false,
		}
		out, err := gp.Compile("amd64", fz)
		if err != nil {
			t.Fatalf("Program did not compile: %s", out)
			keepdir = false
		}
	}

	if !keepdir {
		os.RemoveAll(WorkDir)
	}
}

var BenchConf = microsmith.ProgramConf{
	microsmith.StmtConf{
		MaxStmtDepth: 2,
	},
	[]microsmith.Type{
		microsmith.BasicType{"bool"},
		microsmith.BasicType{"int"},
		microsmith.BasicType{"int16"},
		microsmith.BasicType{"uint"},
		microsmith.BasicType{"float64"},
		microsmith.BasicType{"complex128"},
		microsmith.BasicType{"rune"},
		microsmith.BasicType{"string"},
	},
}

var gp *ast.File

func BenchmarkProgram(b *testing.B) {
	b.ReportAllocs()
	rand := rand.New(rand.NewSource(19))
	for i := 0; i < b.N; i++ {
		db := microsmith.NewDeclBuilder(rand, BenchConf)
		gp = db.File(1)
	}
}
