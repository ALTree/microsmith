package microsmith

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Program holds a Go program (both it source and the reference to the
// file it was possibly written to).
//
// TODO: split to source/seed and filesystem stuff(?)
type Program struct {
	id       uint64
	source   []byte
	fileName string
	file     *os.File
	workdir  string

	Stats ProgramStats
}

type ProgramStats struct {
	Stmt StmtStats
	// TODO: expr stats
}

var Nfuncs int = 8
var CheckSeed int

func init() {
	rs := rand.New(rand.NewSource(time.Now().UnixNano()))
	CheckSeed = rs.Int() % 1e5
}

// NewProgram uses a DeclBuilder to generate a new random Go program
// with the given seed.
func NewProgram(rs *rand.Rand, conf ProgramConf) (*Program, error) {
	// Check wheter conf is a valid one, but without silently fixing
	// it. We want to return an error upstream.
	if err := conf.Check(false); err != nil {
		return nil, err
	}

	db := NewDeclBuilder(rs, conf)
	var buf bytes.Buffer
	printer.Fprint(&buf, token.NewFileSet(), db.File("main", Nfuncs))

	gp := &Program{
		id:     rs.Uint64(),
		source: buf.Bytes(),
	}
	gp.Stats.Stmt = db.sb.stats

	// Put a newline between each function to make the generate a
	// source file that is easier to navigate.
	gp.source = bytes.ReplaceAll(
		gp.source,
		[]byte("func "),
		[]byte("\nfunc "),
	)

	return gp, nil
}

// WriteToFile writes gp in a file having name 'prog<gp.seed>' in the
// folder passsed in the path parameter.
func (gp *Program) WriteToFile(path string) error {
	fileName := fmt.Sprintf("prog%v.go", gp.id)
	fh, err := os.Create(path + "/" + fileName)
	defer fh.Close()
	if err != nil {
		return err
	}

	fh.Write(gp.source)
	gp.fileName = fileName
	gp.file = fh
	gp.workdir = path
	return nil
}

// Check uses go/parser and go/types to parse and typecheck gp
// in-memory. If the parsing fails, it returns the parse error. If the
// typechecking fails, it returns the typechecking error. Otherwise,
// returns nil.
func (gp *Program) Check() error {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, gp.fileName, gp.source, 0)
	if err != nil {
		return err // parse error
	}

	conf := types.Config{Importer: importer.Default()}
	_, err = conf.Check(gp.fileName, fset, []*ast.File{f}, nil)
	if err != nil {
		return err // typecheck error
	}

	return nil
}

// Compile uses gc to compile the file containing the source code of
// gp. It assumes that gp was already written to disk using
// gp.WriteToFile. If the compilation process fails, it returns gc's
// error message and the cmd error code.
func (gp *Program) Compile(toolchain, goarch string, noopt, race, ssacheck bool) (string, error) {
	if gp.file == nil {
		return "", errors.New("cannot compile program with no *File")
	}

	var cmd *exec.Cmd
	switch {
	case strings.Contains(toolchain, "gccgo"):
		binName := strings.TrimSuffix(gp.fileName, ".go")
		oFlag := "-O2"
		if noopt {
			oFlag = "-Og"
		}
		cmd = exec.Command(toolchain, oFlag, "-o", binName+".o", gp.fileName)
	case strings.Contains(toolchain, "tinygo"):
		binName := strings.TrimSuffix(gp.fileName, ".go")
		if noopt {
			cmd = exec.Command(toolchain, "build", "-opt", "z", "-o", binName+".o", gp.fileName)
		} else {
			cmd = exec.Command(toolchain, "build", "-o", binName+".o", gp.fileName)
		}

	default:
		buildArgs := []string{"tool", "compile"}
		if race {
			buildArgs = append(buildArgs, "-race")
		}
		if noopt {
			buildArgs = append(buildArgs, "-N")
		}
		if ssacheck {
			cs := fmt.Sprintf("-d=ssa/check/seed=%v", CheckSeed)
			buildArgs = append(buildArgs, cs)
		}
		buildArgs = append(buildArgs, gp.fileName)
		cmd = exec.Command(toolchain, buildArgs...)
		cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+goarch)
		if goarch == "wasm" {
			cmd.Env = append(cmd.Env, "GOOS=js")
		}
	}
	cmd.Dir = gp.workdir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}

	gp.DeleteBinary()
	return "", nil
}

// DeleteBinary deletes the object file generated by gp.Compile in the
// workdir.
func (gp Program) DeleteBinary() {
	binPath := strings.TrimSuffix(gp.file.Name(), ".go")
	err := os.Remove(binPath + ".o")
	if err != nil {
		log.Printf("could not remove file %s: %s", binPath, err)
	}
}

// DeleteBinary deletes the file containing the source code of gp, as
// written to disk from gp.WriteToFile.
func (gp Program) DeleteFile() {
	fn := gp.file.Name()
	err := os.Remove(fn)
	if err != nil {
		log.Printf("could not remove file %s: %s", fn, err)
	}
}

// Move gp in a workdir subfolder named "crash".
func (gp Program) MoveCrasher() {
	fld := gp.workdir + "/crash"
	if _, err := os.Stat(fld); os.IsNotExist(err) {
		err := os.Mkdir(fld, os.ModePerm)
		if err != nil {
			fmt.Printf("Create crash subfolder failed: %v", err)
			os.Exit(2)
		}
	}

	err := os.Rename(gp.file.Name(), fld+"/"+gp.fileName)
	if err != nil {
		fmt.Printf("Move crasher to folder failed: %v", err)
		os.Exit(2)
	}
}

func (gp Program) String() string {
	line := strings.Repeat("-", 80) + "\n"
	return fmt.Sprintf(line+"%s"+line, gp.source)
}
