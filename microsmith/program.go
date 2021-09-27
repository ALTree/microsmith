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

type Program struct {
	id      uint64      // random id used in the names of the Program files
	workdir string      // directory where the Program files are written
	files   []*File     // the program's files
	conf    ProgramConf // settings driving what kind of program is generated
}

type File struct {
	pkg    string
	source []byte
	name   string
	path   *os.File
}

type BuildOptions struct {
	Toolchain             string
	Noopt, Race, Ssacheck bool
	Unified               bool
}

type CodeOptions struct {
	Typeparams bool
}

var CheckSeed int

func init() {
	rs := rand.New(rand.NewSource(time.Now().UnixNano()))
	CheckSeed = rs.Int() % 1e5
}

func NewProgram(rs *rand.Rand, conf ProgramConf) *Program {
	pg := &Program{
		id:    rs.Uint64(),
		conf:  conf,
		files: make([]*File, 0),
	}

	if pg.conf.MultiPkg {
		pg.files = append(pg.files, pg.NewFile(rs, "a"))
	}
	pg.files = append(pg.files, pg.NewFile(rs, "main"))

	return pg
}

func (gp *Program) NewFile(rs *rand.Rand, pkg string) *File {
	db := NewDeclBuilder(rs, gp.conf)
	var buf bytes.Buffer
	printer.Fprint(&buf, token.NewFileSet(), db.File(pkg, gp.id))
	src := bytes.ReplaceAll(buf.Bytes(), []byte("func "), []byte("\nfunc "))
	return &File{pkg: pkg, source: src}
}

func (gp *Program) WriteToDisk(path string) error {
	gp.workdir = path
	for i, f := range gp.files {
		fileName := fmt.Sprintf("prog%v_%v.go", gp.id, f.pkg)
		fh, err := os.Create(path + "/" + fileName)
		defer fh.Close()
		if err != nil {
			return err
		}

		fh.Write(f.source)
		gp.files[i].name = fileName
		gp.files[i].path = fh
	}
	return nil
}

// Check uses go/parser and go/types to parse and typecheck gp
// in-memory.
func (gp *Program) Check() error {
	if len(gp.files) > 1 {
		// multi-package program, skip typechecking
		return nil
	}

	file, fset := gp.files[0], token.NewFileSet()
	f, err := parser.ParseFile(fset, file.name, file.source, 0)
	if err != nil {
		return err // parse error
	}

	conf := types.Config{Importer: importer.Default()}
	_, err = conf.Check(file.name, fset, []*ast.File{f}, nil)
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
func (gp *Program) Compile(arch string, bo BuildOptions) (string, error) {
	if len(gp.files) == 0 {
		return "", errors.New("cannot compile program with no files")
	}

	baseName := fmt.Sprintf("prog%v", gp.id)
	arcName := baseName + "_main.o"

	switch {

	case strings.Contains(bo.Toolchain, "gccgo"):
		oFlag := "-O2"
		if bo.Noopt {
			oFlag = "-Og"
		}
		cmd := exec.Command(bo.Toolchain, oFlag, "-o", arcName, baseName+"_main.go")
		cmd.Dir = gp.workdir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), err
		}

	case strings.Contains(bo.Toolchain, "tinygo"):
		var cmd *exec.Cmd
		if bo.Noopt {
			cmd = exec.Command(bo.Toolchain, "build", "-opt", "z", "-o", arcName, baseName+"_main.go")
		} else {
			cmd = exec.Command(bo.Toolchain, "build", "-o", arcName, baseName+"_main.go")
		}
		cmd.Dir = gp.workdir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), err
		}

	default:

		// Setup env variables
		env := os.Environ()
		if arch == "wasm" {
			env = append(env, "GOOS=js")
		} else {
			env = append(env, "GOOS=linux")
		}
		if arch == "386sf" {
			env = append(env, "GOARCH=386", "GO386=softfloat")
		} else {
			env = append(env, "GOARCH="+arch)
		}
		if bo.Unified {
			env = append(env, "GOEXPERIMENT=unified")
		}

		// Setup compile args
		buildArgs := []string{"tool", "compile"}
		if bo.Race {
			buildArgs = append(buildArgs, "-race")
		}
		if bo.Noopt {
			buildArgs = append(buildArgs, "-N", "-l")
		}
		if bo.Ssacheck {
			cs := fmt.Sprintf("-d=ssa/check/seed=%v", CheckSeed)
			buildArgs = append(buildArgs, cs)
		}

		// Compile
		for _, file := range gp.files {
			var cmdArgs []string
			if file.pkg == "main" {
				cmdArgs = append(buildArgs, "-I=.")
				cmdArgs = append(cmdArgs, file.name)
			} else {
				cmdArgs = append(buildArgs, file.name)
			}

			cmd := exec.Command(bo.Toolchain, cmdArgs...)
			cmd.Dir, cmd.Env = gp.workdir, env
			out, err := cmd.CombinedOutput()
			if err != nil {
				return string(out), err
			}
		}

		// Setup link args
		linkArgs := []string{"tool", "link", "-L=."}
		if bo.Race {
			linkArgs = append(linkArgs, "-race")
		}
		linkArgs = append(linkArgs, "-o", baseName, arcName)

		// Link
		cmd := exec.Command(bo.Toolchain, linkArgs...)
		cmd.Dir, cmd.Env = gp.workdir, env
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), err
		}
	}

	gp.DeleteBinaries()
	return "", nil
}

// DeleteBinaries deletes any binary file written on disk.
func (gp Program) DeleteBinaries() {
	basePath := gp.workdir + fmt.Sprintf("/prog%v", gp.id)
	for _, file := range gp.files {
		err := os.Remove(basePath + "_" + file.pkg + ".o")
		if err != nil {
			log.Printf("could not remove %s: %s", basePath+"_"+file.pkg+".o", err)
		}

	}

	// ignore error since some toolchains don't write a binary
	_ = os.Remove(basePath)
}

// DeleteSource deletes all gp files.
func (gp Program) DeleteSource() {
	for _, file := range gp.files {
		_ = os.Remove(file.path.Name())
	}
}

// Move gp's files in a workdir subfolder named "crash".
func (gp Program) MoveCrasher() {
	fld := gp.workdir + "/crash"
	if _, err := os.Stat(fld); os.IsNotExist(err) {
		err := os.Mkdir(fld, os.ModePerm)
		if err != nil {
			fmt.Printf("Could not create crash folder: %v", err)
			os.Exit(2)
		}
	}

	for _, file := range gp.files {
		err := os.Rename(file.path.Name(), fld+"/"+file.name)
		if err != nil {
			fmt.Printf("Could not move crasher: %v", err)
			os.Exit(2)
		}
	}
}

func (p Program) String() string {
	var res string
	for _, file := range p.files {
		res += string(file.source)
		if len(p.files) > 1 {
			res += "\n--------\n"
		}
	}
	return res
}

func (p Program) Name() string {
	return fmt.Sprintf("prog%v", p.id)
}
