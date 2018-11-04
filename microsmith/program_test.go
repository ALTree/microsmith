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
			MaxStmtDepth: 1,
			StmtKindChance: []float64{
				1, 1, 1, 1, 1,
			},
			MaxBlockVars:  1,
			MaxBlockStmts: 1,
			UseArrays:     true,
			UseStructs:    true,
			UsePointers:   true,
		},
		microsmith.ExprConf{
			ExprKindChance: []float64{
				1, 1, 1,
			},
			LiteralChance:    0.2,
			ComparisonChance: 0.1,
			IndexChance:      0.1,
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
			StmtKindChance: []float64{
				1, 1, 1, 1, 1,
			},
			MaxBlockVars:  3,
			MaxBlockStmts: 4,
			UseArrays:     true,
			UseStructs:    true,
			UsePointers:   true,
		},
		microsmith.ExprConf{
			ExprKindChance: []float64{
				1, 1, 1,
			},
			LiteralChance:    0.2,
			ComparisonChance: 0.1,
			IndexChance:      0.1,
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
			StmtKindChance: []float64{
				1, 1, 1, 1, 1,
			},
			MaxBlockVars:  12,
			MaxBlockStmts: 8,
			UseArrays:     true,
			UseStructs:    true,
			UsePointers:   true,
		},
		microsmith.ExprConf{
			ExprKindChance: []float64{
				1, 1, 1,
			},
			LiteralChance:    0.2,
			ComparisonChance: 0.1,
			IndexChance:      0.1,
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
		gp, err := microsmith.NewGoProgram(rand.Int63(), conf)
		if err != nil {
			t.Fatalf("BadConf error: %s\n", err)
		}
		err = gp.Check()
		if err != nil {
			t.Fatalf("Program failed typechecking: %s\n%s", err, gp)
		}
	}
}

func TestDefault(t *testing.T) {
	testProgramGoTypes(t, 200, microsmith.DefaultConf)
}

func TestRandConf(t *testing.T) {
	for i := 0; i < 100; i++ {
		conf := microsmith.RandConf()
		// leave this (useful for debugging)
		//fmt.Printf("%+v\n\n", conf)
		testProgramGoTypes(t, 10, conf)
	}
}

func TestSmall(t *testing.T) {
	testProgramGoTypes(t, 1000, TestConfigurations["small"])
}

func TestMedium(t *testing.T) {
	testProgramGoTypes(t, 1000, TestConfigurations["medium"])
}

func TestBig(t *testing.T) {
	testProgramGoTypes(t, 50, TestConfigurations["big"])
}

func TestNoArrays(t *testing.T) {
	tc := TestConfigurations["medium"]
	tc.UseArrays = false
	testProgramGoTypes(t, 500, tc)
}

func TestNoStructs(t *testing.T) {
	tc := TestConfigurations["medium"]
	tc.UseStructs = false
	testProgramGoTypes(t, 500, tc)
}

func TestNoPointers(t *testing.T) {
	tc := TestConfigurations["medium"]
	tc.UsePointers = false
	testProgramGoTypes(t, 500, tc)
}

func TestNoLiterals(t *testing.T) {
	tc := TestConfigurations["medium"]
	tc.LiteralChance = 0.0
	tc.UseArrays = false
	testProgramGoTypes(t, 1000, tc)
}

func TestAllLiterals(t *testing.T) {
	tc := TestConfigurations["medium"]
	tc.LiteralChance = 1.0
	testProgramGoTypes(t, 1000, tc)
}

func TestAllUnary(t *testing.T) {
	tc := TestConfigurations["medium"]
	tc.ExprKindChance = []float64{1.0, 0, 0}
	testProgramGoTypes(t, 1000, tc)
}

func TestOnlyOneType(t *testing.T) {
	tc := TestConfigurations["medium"]

	for _, typ := range []string{"bool", "int", "string", "float64", "complex128"} {
		t.Run(typ, func(t *testing.T) {
			tc.SupportedTypes = []microsmith.Type{microsmith.BasicType{typ}}
			testProgramGoTypes(t, 100, tc)
		})
	}
}

func testBadConf(t *testing.T, conf microsmith.ProgramConf) {
	_, err := microsmith.NewGoProgram(rand.Int63(), conf)
	if err == nil {
		t.Errorf("Expected bad conf error for\n%+v\n", conf)
	}
}

func TestBadConfs(t *testing.T) {
	// IndexChance = 1 and LiteralChance = 0
	tc := TestConfigurations["medium"]
	tc.UseArrays = true
	tc.IndexChance = 1
	tc.LiteralChance = 0
	testBadConf(t, tc)

	// all zero StmtKindChance
	tc = TestConfigurations["medium"]
	for i := range tc.StmtKindChance {
		tc.StmtKindChance[i] = 0.0
	}
	testBadConf(t, tc)

	// all zero ExprKindChance
	tc = TestConfigurations["medium"]
	for i := range tc.ExprKindChance {
		tc.ExprKindChance[i] = 0.0
	}
	testBadConf(t, tc)
}

// check generated programs with gc (from file)
// Speed is ~10 program/second
func TestProgramGc(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	rand := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		gp, _ := microsmith.NewGoProgram(rand.Int63(), microsmith.DefaultConf)
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

var gp *ast.File

func BenchmarkProgramGeneration(b *testing.B) {
	b.ReportAllocs()
	rand := rand.New(rand.NewSource(19))
	for i := 0; i < b.N; i++ {
		db := microsmith.NewDeclBuilder(rand.Int63(), microsmith.DefaultConf)
		gp = db.File("main", 1)
	}
}
