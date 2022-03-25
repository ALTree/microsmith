package microsmith_test

import (
	"io/ioutil"
	"os"
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
		MultiPkg: false,
	},

	"medium": {
		MultiPkg: false,
	},

	"big": {
		MultiPkg: false,
	},
	"huge": {
		MultiPkg: false,
	},
}

// check n generated programs with go/types (in-memory)
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

func TestRandConf(t *testing.T) {
	n := 10
	if testing.Short() {
		n = 5
	}
	for i := 0; i < 10; i++ {
		// no multipkg, no typeparams
		testProgramGoTypes(t, n, microsmith.ProgramConf{})
	}
}

func TestRandConfTypeParams(t *testing.T) {
	n := 10
	if testing.Short() {
		n = 5
	}
	for i := 0; i < 10; i++ {
		testProgramGoTypes(
			t, n,
			microsmith.ProgramConf{
				MultiPkg:   false,
				TypeParams: true,
			})
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

func TestTypeParams(t *testing.T) {
	conf := TestConfigurations["medium"]
	conf.TypeParams = true
	testProgramGoTypes(t, 50, conf)
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
