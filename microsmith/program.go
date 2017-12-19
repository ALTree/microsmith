package microsmith

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
)

type GoProgram struct {
	seed     int64
	source   []byte
	fileName string
	file     *os.File
	workDir  string
}

func NewGoProgram(seed int64) *GoProgram {
	gp := new(GoProgram)

	db := NewDeclBuilder(seed)
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
	gp.workDir = path
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
	// TODO: configurable GOARCH
	// cmd.Env = append(cmd.Env, "GOARCH=386")
	cmd.Dir = gp.workDir
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
	fmtstr :=
		"[seed %v]\n" +
			"----------------\n" +
			"%s" +
			"----------------\n"
	return fmt.Sprintf(fmtstr, gp.seed, gp.source)
}
