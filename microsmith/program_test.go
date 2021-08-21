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
		StmtConf: microsmith.StmtConf{
			MaxStmtDepth: 1,
		},
		Types:    allTypes,
		MultiPkg: false,
		FuncNum:  2,
	},

	"medium": {
		StmtConf: microsmith.StmtConf{
			MaxStmtDepth: 2,
		},
		Types:    allTypes,
		MultiPkg: false,
		FuncNum:  4,
	},

	"big": {
		StmtConf: microsmith.StmtConf{
			MaxStmtDepth: 3,
		},
		Types:    allTypes,
		MultiPkg: false,
		FuncNum:  8,
	},
	"huge": {
		StmtConf: microsmith.StmtConf{
			MaxStmtDepth: 5,
		},
		Types:    allTypes,
		MultiPkg: false,
		FuncNum:  4,
	},
}

// check n generated programs with go/types (in-memory)
func testProgramGoTypes(t *testing.T, n int, conf microsmith.ProgramConf) {
	rs := rand.New(rand.NewSource(7411))
	for i := 0; i < n; i++ {
		gp := microsmith.NewProgram(rs, conf)
		err := gp.Check()
		if err != nil {
			tmpfile, _ := ioutil.TempFile("", "fail*.go")
			if _, err := tmpfile.Write([]byte(gp.String())); err != nil {
				t.Fatal(err)
			}
			t.Fatalf("Program failed typechecking:\n%s\n%v", err, gp)
		}
	}
}

func TestRandConf(t *testing.T) {
	n := 50
	if testing.Short() {
		n = 20
	}
	rs := rand.New(rand.NewSource(42))
	for i := 0; i < 10; i++ {
		conf := microsmith.RandConf(rs)
		testProgramGoTypes(t, n, conf)
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

// Check generated programs with gc (from file).
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
		err := gp.WriteToDisk(WorkDir)
		if err != nil {
			t.Fatalf("Could not write to file: %s", err)
		}
		fz := microsmith.FuzzOptions{
			"go",
			false, false, false, false,
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
	StmtConf: microsmith.StmtConf{
		MaxStmtDepth: 2,
	},
	Types: []microsmith.Type{
		microsmith.BasicType{"bool"},
		microsmith.BasicType{"int"},
		microsmith.BasicType{"int16"},
		microsmith.BasicType{"uint"},
		microsmith.BasicType{"float64"},
		microsmith.BasicType{"complex128"},
		microsmith.BasicType{"rune"},
		microsmith.BasicType{"string"},
	},
	MultiPkg: false,
	FuncNum:  4,
}

var gp *ast.File

func BenchmarkProgram(b *testing.B) {
	b.ReportAllocs()
	rand := rand.New(rand.NewSource(19))
	for i := 0; i < b.N; i++ {
		db := microsmith.NewDeclBuilder(rand, BenchConf)
		gp = db.File("a", 0)
	}
}

var sink string

func BenchmarkRandString(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sink = microsmith.RandString()
	}
}
