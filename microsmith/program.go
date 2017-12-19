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

// GoProgram holds a Go program (both it source and the reference to
// the file it was possibly written to).
// TODO: split to source/seed and filesystem stuff(?)
type GoProgram struct {
	seed     int64
	source   []byte
	fileName string
	file     *os.File
	workDir  string
}

// NewGoProgram uses a DeclBuilder to generate a new random Go program
// with the passed seed.
func NewGoProgram(seed int64) *GoProgram {
	gp := new(GoProgram)

	db := NewDeclBuilder(seed)
	var buf bytes.Buffer
	printer.Fprint(&buf, token.NewFileSet(), db.File("main", 1))

	gp.seed = seed
	gp.source = buf.Bytes()

	return gp
}

// WriteToFile writes gp in a file having name 'prog<gp.seed>' in the
// folder passsed in the path parameter.
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

// Check uses go/parser and go/types to parse and typecheck gp
// in-memory. If the parsing fails, it returns the parse error. If the
// typechecking fails, it returns the typechecking error. Otherwise,
// returns nil.
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

// Compile uses gc to compile the file containing the source code of
// gp. It assumes that gp was already written to disk using
// gp.WriteToFile. If the compilatio process fails, it logs gc's error
// message and returns the cmd error code.
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

// DeleteBinary deletes the binary generated by gp.Compile in the
// workDir.
func (gp GoProgram) DeleteBinary() {
	binPath := strings.TrimSuffix(gp.file.Name(), ".go")
	err := os.Remove(binPath)
	if err != nil {
		log.Printf("could not remove file %s: %s", binPath, err)
	}
}

// DeleteBinary deletes the file containing the source code of gp, as
// written to disk from gp.WriteToFile.
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