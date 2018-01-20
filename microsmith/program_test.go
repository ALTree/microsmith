package microsmith_test

import (
	"math/rand"
	"testing"

	"github.com/ALTree/microsmith/microsmith"
)

const WorkDir = "../work/"

// check generated programs with go/types (in-memory)
func TestProgramGotypes(t *testing.T) {
	rand := rand.New(rand.NewSource(42))
	for i := 0; i < 2000; i++ {
		gp := microsmith.NewGoProgram(rand.Int63())
		err := gp.Check()
		if err != nil {
			t.Fatalf("Program failed typechecking: %s\n%s", err, gp)
		}
	}
}

// check generated programs with gc (from file)
// Speed is ~10 program/second
func TestProgramGc(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	rand := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		gp := microsmith.NewGoProgram(rand.Int63())
		err := gp.WriteToFile(WorkDir)
		if err != nil {
			t.Fatalf("Could not write to file: %s", err)
		}
		err = gp.Compile()
		if err != nil {
			t.Fatalf("Program did not compile: %s\n%s", err, gp)
		}
		gp.DeleteFile()
	}
}

var gp *microsmith.GoProgram

func BenchmarkProgramGeneration(b *testing.B) {
	b.ReportAllocs()
	rand := rand.New(rand.NewSource(19))
	for i := 0; i < b.N; i++ {
		gp = microsmith.NewGoProgram(rand.Int63())
	}
}
