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

var FuncCount = 8
var CheckSeed int

func init() {
	rs := rand.New(rand.NewSource(time.Now().UnixNano()))
	CheckSeed = rs.Int() % 1e5
}

// NewProgram uses a DeclBuilder to generate a new random Go program
// with the given seed.
func NewProgram(rs *rand.Rand, conf ProgramConf) (*Program, error) {

	db := NewDeclBuilder(rs, conf)
	var buf bytes.Buffer
	printer.Fprint(&buf, token.NewFileSet(), db.File(FuncCount))

	gp := &Program{id: rs.Uint64(), source: buf.Bytes()}
	gp.Stats.Stmt = db.sb.stats

	// Insert a newline between each function to make the generated
	// program easier to navigate.
	gp.source = bytes.ReplaceAll(
		gp.source,
		[]byte("func "),
		[]byte("\nfunc "),
	)

	return gp, nil
}

// WriteToFile writes gp's source in a file named prog<gp.seed>.go, in
// the folder passed in the path parameter.
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

// Compile uses the given toolchain to build gp. It assumes that gp's
// source is already written to disk by gp.WriteToFile.
//
// If the compilation subprocess exits with an error code, Compile
// returns the error message printed by the toolchain and the
// subprocess error code.
func (gp *Program) Compile(toolchain, goarch string, noopt, race, ssacheck bool) (string, error) {
	if gp.file == nil {
		return "", errors.New("cannot compile program with no *File")
	}

	arcName := strings.TrimSuffix(gp.fileName, ".go") + ".o"
	binName := strings.TrimSuffix(gp.fileName, ".go")

	switch {

	case strings.Contains(toolchain, "gccgo"):
		oFlag := "-O2"
		if noopt {
			oFlag = "-Og"
		}
		cmd := exec.Command(toolchain, oFlag, "-o", arcName, gp.fileName)
		cmd.Dir = gp.workdir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), err
		}

	case strings.Contains(toolchain, "tinygo"):
		var cmd *exec.Cmd
		if noopt {
			cmd = exec.Command(toolchain, "build", "-opt", "z", "-o", arcName, gp.fileName)
		} else {
			cmd = exec.Command(toolchain, "build", "-o", arcName, gp.fileName)
		}
		cmd.Dir = gp.workdir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), err
		}

	default:
		buildArgs := []string{"tool", "compile"}
		if race {
			buildArgs = append(buildArgs, "-race")
		}
		if noopt {
			buildArgs = append(buildArgs, "-N", "-l")
		}
		if ssacheck {
			cs := fmt.Sprintf("-d=ssa/check/seed=%v", CheckSeed)
			buildArgs = append(buildArgs, cs)
		}
		buildArgs = append(buildArgs, gp.fileName)

		env := os.Environ()
		if goarch == "wasm" {
			env = append(env, "GOOS=js")
		} else {
			env = append(env, "GOOS=linux")
		}
		if goarch == "386sf" {
			env = append(env, "GOARCH=386", "GO386=softfloat")
		} else {
			env = append(env, "GOARCH="+goarch)
		}

		// compile
		cmd := exec.Command(toolchain, buildArgs...)
		cmd.Dir, cmd.Env = gp.workdir, env
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), err
		}

		// link
		cmd = exec.Command(toolchain, "tool", "link", "-o", binName, arcName)
		cmd.Dir, cmd.Env = gp.workdir, env
		out, err = cmd.CombinedOutput()
		if err != nil {
			return string(out), err
		}

	}

	gp.DeleteBinaries()
	return "", nil
}

// DeleteBinaries deletes any binary file written on disk.
func (gp Program) DeleteBinaries() {
	binPath := strings.TrimSuffix(gp.file.Name(), ".go")
	err := os.Remove(binPath + ".o")
	if err != nil {
		log.Printf("could not remove %s: %s", binPath+".o", err)
	}

	// ignore error since some toolchains don't write a binary
	_ = os.Remove(binPath)
}

// DeleteSource deletes the file containing the source code of gp, as
// written to disk from gp.WriteTofdFile.
func (gp Program) DeleteSource() {
	fn := gp.file.Name()
	_ = os.Remove(fn)
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
	return string(gp.source)
}
