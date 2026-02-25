package microsmith_test

import (
	"go/ast"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"strings"
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
	n := 100
	if testing.Short() {
		n = 20
	}
	testProgramGoTypes(t, n, microsmith.ProgramConf{})
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
	lim := 20
	if testing.Short() {
		lim = 5
	}

	if _, err := os.Stat(WorkDir); os.IsNotExist(err) {
		err := os.MkdirAll(WorkDir, os.ModePerm)
		if err != nil {
			t.Fatalf("%v", err)
		}
	}

	// build toolchain
	cmd := exec.Command(GetToolchain(), "install", "std")
	env := append(os.Environ(), "GODEBUG=installgoroot=all")
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Installing dependencies failed with error:\n----\n%s\n%s\n----\n", out, err)
	}

	keepdir := false
	for i := 0; i < lim; i++ {
		gp := microsmith.NewProgram(conf)
		err := gp.WriteToDisk(WorkDir)
		if err != nil {
			t.Fatalf("Could not write to file: %s", err)
		}
		bo := microsmith.BuildOptions{
			Toolchain:  GetToolchain(),
			Noopt:      false,
			Race:       false,
			Ssacheck:   false,
			Experiment: "",
		}
		out, err := gp.Compile("amd64", bo)
		if err != nil && !strings.Contains(out, "internal compiler error") {
			t.Fatalf("Generated program failed compilation:\n%s\n%s", out, err)
			keepdir = true
		}
	}

	if !keepdir {
		os.RemoveAll(WorkDir)
	}
}

func TestCompile(t *testing.T) {
	compile(t, microsmith.ProgramConf{MultiPkg: false})
}

func TestCompileMultiPkg(t *testing.T) {
	compile(t, microsmith.ProgramConf{MultiPkg: true})
}

var sink *ast.File

func benchHelper(b *testing.B, conf microsmith.ProgramConf) {
	b.ReportAllocs()
	rand.Seed(1)
	prog := microsmith.NewProgramBuilder(conf)
	pb := microsmith.NewPackageBuilder(conf, "main", prog)
	b.ResetTimer()
	for b.Loop() {
		sink = pb.File()
	}

}

func BenchmarkSinglePkg(b *testing.B) {
	benchHelper(b, microsmith.ProgramConf{MultiPkg: false})
}
