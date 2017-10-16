package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/ALTree/microsmith/microsmith"
)

const WorkDir = "work/"

func main() {

	for i := int64(0); i < 100; i++ {

		fmt.Printf("Seed %v - ", i)
		gp := NewGoProgram(i)

		//fmt.Println("\n", gp)

		err := gp.Check()
		if err != nil {
			log.Fatalf("Program failed typechecking: %s\n%s", err, gp)
		}
		fmt.Printf("typechecking ✓  ")

		err = gp.WriteToFile(WorkDir)
		if err != nil {
			log.Fatalf("Could not write to file: %s", err)
		}
		fmt.Printf("write file ✓  ")

		err = gp.Compile()
		if err != nil {
			log.Fatalf("Program did not compile: %s\n%s", err, gp)
		}
		fmt.Printf("compile ✓  \n")

		//fmt.Printf("Program was compiled successfully.\n%s\n", gp)
		gp.DeleteFile()
	}
}

type GoProgram struct {
	seed     int64
	source   []byte
	fileName string
	file     *os.File
}

func NewGoProgram(seed int64) *GoProgram {
	gp := new(GoProgram)

	db := microsmith.NewDeclBuilder(seed)
	var buf bytes.Buffer
	printer.Fprint(&buf, token.NewFileSet(), db.File("main", 1))

	gp.seed = seed
	gp.source = buf.Bytes()

	return gp
}

func (gp *GoProgram) WriteToFile(path string) error {
	fileName := fmt.Sprintf("prog%v.go", gp.seed)
	fh, err := os.Create(path + fileName)
	defer fh.Close()
	if err != nil {
		return err
	}

	fh.Write(gp.source)

	gp.fileName = fileName
	gp.file = fh
	return nil
}

func (gp *GoProgram) Check() error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, gp.fileName, gp.source, 0)
	if err != nil {
		return err // parse error
	}

	conf := types.Config{}
	_, err = conf.Check(gp.fileName, fset, []*ast.File{f}, nil)
	if err != nil {
		return err // typecheck error
	}

	return nil
}

func (gp *GoProgram) Compile() error {
	if gp.file == nil {
		return errors.New("cannot compile program with no *File")
	}

	cmd := exec.Command("go", "build", gp.fileName)
	cmd.Dir = WorkDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Compiler error:\n%s\n", out)
		return err
	}

	gp.DeleteBinary()
	return nil
}

func (gp GoProgram) DeleteBinary() {
	binPath := strings.TrimSuffix(gp.file.Name(), ".go")
	err := os.Remove(binPath)
	if err != nil {
		log.Printf("could not remove file %s: %s", binPath, err)
	}
}

func (gp GoProgram) DeleteFile() {
	fn := gp.file.Name()
	err := os.Remove(fn)
	if err != nil {
		log.Printf("could not remove file %s: %s", fn, err)
	}
}

func (gp GoProgram) String() string {
	ds := "--------"
	return fmt.Sprintf("%s\n%s%s", ds, gp.source, ds)
}
