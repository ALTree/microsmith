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
	Seed     int64
	source   []byte
	fileName string
	file     *os.File
	workDir  string
}

// NewGoProgram uses a DeclBuilder to generate a new random Go program
// with the passed seed.
func NewGoProgram(seed int64, conf ProgramConf) (*GoProgram, error) {
	// Check wheter conf is a valid one, but without silently fixing
	// it. We want to return an error upstream.
	if err := conf.Check(false); err != nil {
		return nil, err
	}

	gp := new(GoProgram)

	db := NewDeclBuilder(seed, conf)
	var buf bytes.Buffer
	printer.Fprint(&buf, token.NewFileSet(), db.File("main", 1))

	gp.Seed = seed
	gp.source = buf.Bytes()

	return gp, nil
}

// WriteToFile writes gp in a file having name 'prog<gp.seed>' in the
// folder passsed in the path parameter.
func (gp *GoProgram) WriteToFile(path string) error {
	fileName := fmt.Sprintf("prog%v.go", gp.Seed)
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
// gp.WriteToFile. If the compilatio process fails, it returns gc's
// error message and the cmd error code.
func (gp *GoProgram) Compile(toolchain, goarch string, noopt, race bool) (string, error) {
	if gp.file == nil {
		return "", errors.New("cannot compile program with no *File")
	}

	var cmd *exec.Cmd
	if !strings.Contains(toolchain, "gccgo") {
		buildArgs := []string{"tool", "compile"}
		if race {
			buildArgs = append(buildArgs, "-race")
		}
		if noopt {
			buildArgs = append(buildArgs, "-N")
		}
		buildArgs = append(buildArgs, gp.fileName)
		cmd = exec.Command(toolchain, buildArgs...)
		cmd.Env = append(os.Environ(), "GOARCH="+goarch)
	} else {
		binName := strings.TrimSuffix(gp.fileName, ".go")
		oFlag := "-O2"
		if noopt {
			oFlag = "-Og"
		}
		cmd = exec.Command(toolchain, oFlag, "-o", binName, gp.fileName)
		// no support for custom GOARCH when using gccgo
	}
	cmd.Dir = gp.workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}

	gp.DeleteBinary()
	return "", nil
}

// DeleteBinary deletes the object file generated by gp.Compile in the
// workDir.
func (gp GoProgram) DeleteBinary() {
	binPath := strings.TrimSuffix(gp.file.Name(), ".go")
	err := os.Remove(binPath + ".o")
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
	return fmt.Sprintf(fmtstr, gp.Seed, gp.source)
}
