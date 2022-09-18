package microsmith_test

import (
	"go/ast"
	"io/ioutil"
	"os"
	"testing"

	"github.com/ALTree/microsmith/microsmith"
)

const WorkDir = "work"

// check n generated programs with go/types
func testProgramGoTypes(t *testing.T, n int, conf microsmith.ProgramConf) {
	for i := 0; i < n; i++ {
		gp := microsmith.NewProgram(conf)
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

func TestNewProgram(t *testing.T) {
	n := 50
	if testing.Short() {
		n = 10
	}
	testProgramGoTypes(t, n, microsmith.ProgramConf{})
}

func TestNewProgramTP(t *testing.T) {
	n := 50
	if testing.Short() {
		n = 10
	}

	testProgramGoTypes(
		t, n,
		microsmith.ProgramConf{
			MultiPkg:   false,
			TypeParams: true,
		})
}

func GetToolchain() string {
	if bin := os.Getenv("GO_TC"); bin != "" {
		return bin
	} else {
		return "go"
	}
}

// Check generated programs with gc (from file).
func compile(t *testing.T, conf microsmith.ProgramConf) {
	lim := 10
	if testing.Short() {
		lim = 2
	}

	if _, err := os.Stat(WorkDir); os.IsNotExist(err) {
		err := os.MkdirAll(WorkDir, os.ModePerm)
		if err != nil {
			t.Fatalf("%v", err)
		}
	}

	keepdir := false
	for i := 0; i < lim; i++ {
		gp := microsmith.NewProgram(conf)
		err := gp.WriteToDisk(WorkDir)
		if err != nil {
			t.Fatalf("Could not write to file: %s", err)
		}
		bo := microsmith.BuildOptions{
			GetToolchain(),
			false, false, false, false,
		}
		out, err := gp.Compile("amd64", bo)
		if err != nil {
			t.Fatalf("Program did not compile: %s\n%s", out, err)
			keepdir = true
		}
	}

	if !keepdir {
		os.RemoveAll(WorkDir)
	}
}

func TestCompile(t *testing.T) {
	compile(t,
		microsmith.ProgramConf{
			MultiPkg:   false,
			TypeParams: false,
		})
}

func TestCompileMultiPkg(t *testing.T) {
	compile(t,
		microsmith.ProgramConf{
			MultiPkg:   true,
			TypeParams: false,
		})
}

func TestCompileTypeParams(t *testing.T) {
	compile(t,
		microsmith.ProgramConf{
			MultiPkg:   false,
			TypeParams: true,
		})
}

func TestCompileMultiPkgTypeParams(t *testing.T) {
	compile(t,
		microsmith.ProgramConf{
			MultiPkg:   true,
			TypeParams: true,
		})
}

var sink *ast.File

func benchHelper(b *testing.B, conf microsmith.ProgramConf) {
	b.ReportAllocs()
	pb := microsmith.NewProgramBuilder(conf, 1)
	for i := 0; i < b.N; i++ {
		db := microsmith.NewPackageBuilder(conf, "main", pb)
		sink = db.File()
	}
}

func BenchmarkSinglePkg(b *testing.B) {
	benchHelper(b, microsmith.ProgramConf{
		MultiPkg:   false,
		TypeParams: false,
	})
}

func BenchmarkSinglePkgTP(b *testing.B) {
	benchHelper(b, microsmith.ProgramConf{
		MultiPkg:   false,
		TypeParams: true,
	})
}
