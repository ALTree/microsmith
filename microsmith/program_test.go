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
			UseArrays:     false,
		},
		microsmith.ExprConf{
			ExprKindChance: []float64{
				1, 1, 1,
			},
			LiteralChance:    0.2,
			ComparisonChance: 0.1,
			IndexChance:      0.1,
		},
	},

	"medium": {
		microsmith.StmtConf{
			MaxStmtDepth: 2,
			StmtKindChance: []float64{
				1, 1, 1, 1, 1,
			},
			MaxBlockVars:  len(microsmith.SupportedTypes),
			MaxBlockStmts: 4,
			UseArrays:     false,
		},
		microsmith.ExprConf{
			ExprKindChance: []float64{
				1, 1, 1,
			},
			LiteralChance:    0.2,
			ComparisonChance: 0.1,
			IndexChance:      0.1,
		},
	},

	"big": {
		microsmith.StmtConf{
			MaxStmtDepth: 3,
			StmtKindChance: []float64{
				1, 1, 1, 1, 1,
			},
			MaxBlockVars:  4 * len(microsmith.SupportedTypes),
			MaxBlockStmts: 8,
			UseArrays:     false,
		},
		microsmith.ExprConf{
			ExprKindChance: []float64{
				1, 1, 1,
			},
			LiteralChance:    0.2,
			ComparisonChance: 0.1,
			IndexChance:      0.1,
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

func TestGoTypesDefault(t *testing.T) {
	testProgramGoTypes(t, 100, microsmith.DefaultConf)
}

func TestGoTypesRandConf(t *testing.T) {
	for i := 0; i < 100; i++ {
		conf := microsmith.RandConf()
		// leave this (useful for debugging)
		// fmt.Printf("%+v\n\n", conf)
		testProgramGoTypes(t, 10, conf)
	}
}

func TestGoTypesSmall(t *testing.T) {
	testProgramGoTypes(t, 1000, TestConfigurations["small"])
}

func TestGoTypesMedium(t *testing.T) {
	testProgramGoTypes(t, 1000, TestConfigurations["medium"])
}

func TestGoTypesBig(t *testing.T) {
	testProgramGoTypes(t, 50, TestConfigurations["big"])
}

func TestGoTypesArrays(t *testing.T) {
	tc := TestConfigurations["medium"]
	tc.Stmt.UseArrays = true
	testProgramGoTypes(t, 1000, tc)
}

func TestGoTypesNoLiterals(t *testing.T) {
	tc := TestConfigurations["medium"]
	tc.Expr.LiteralChance = 0.0
	testProgramGoTypes(t, 1000, tc)
}

func TestGoTypesAllLiterals(t *testing.T) {
	tc := TestConfigurations["medium"]
	tc.Expr.LiteralChance = 1.0
	testProgramGoTypes(t, 1000, tc)
}

func TestGoTypesAllUnary(t *testing.T) {
	tc := TestConfigurations["medium"]
	tc.Expr.ExprKindChance = []float64{1.0, 0, 0}
	testProgramGoTypes(t, 1000, tc)
}

func testBadConf(t *testing.T, conf microsmith.ProgramConf) {
	_, err := microsmith.NewGoProgram(rand.Int63(), conf)
	if err == nil {
		t.Errorf("Expected bad conf error for\n%+v\n", conf)
	}
}

func TestGoTypesBadConfs(t *testing.T) {
	// IndexChance = 1 and LiteralChance = 0
	tc := TestConfigurations["medium"]
	tc.Stmt.UseArrays = true
	tc.Expr.IndexChance = 1
	tc.Expr.LiteralChance = 0
	testBadConf(t, tc)

	// all zero StmtKindChance
	tc = TestConfigurations["medium"]
	for i := range tc.Stmt.StmtKindChance {
		tc.Stmt.StmtKindChance[i] = 0.0
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
		out, err := gp.Compile("go", "amd64", false)
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
