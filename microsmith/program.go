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
	id      uint64  // random id used in the names of the Program files
	workdir string  // directory where the Program files are written
	files   []*File // the program's files
	Stats   ProgramStats
}

type File struct {
	pack   string // the package name
	source []byte
	name   string // TODO(alb): remove?
	path   *os.File
}

type ProgramStats struct {
	Stmt StmtStats
	// TODO: expr stats
}

type FuzzOptions struct {
	Toolchain             string
	Noopt, Race, Ssacheck bool
}

var FuncCount = 8
var CheckSeed int

func init() {
	rs := rand.New(rand.NewSource(time.Now().UnixNano()))
	CheckSeed = rs.Int() % 1e5
}

func NewProgram(rs *rand.Rand, conf ProgramConf) *Program {
	id := rs.Uint64()

	var files []*File = make([]*File, 0)
	if conf.MultiPkg {
		files = append(files, NewFile(NewDeclBuilder(rs, conf), "a", id, conf.MultiPkg))
	}
	files = append(files, NewFile(NewDeclBuilder(rs, conf), "main", id, conf.MultiPkg))

	return &Program{id: id, files: files}
}

func NewFile(db *DeclBuilder, pack string, id uint64, MultiPkg bool) *File {
	var buf bytes.Buffer
	printer.Fprint(&buf, token.NewFileSet(), db.File(FuncCount, pack, id, MultiPkg))
	src := bytes.ReplaceAll(buf.Bytes(), []byte("func "), []byte("\nfunc "))
	return &File{pack: pack, source: src}
}

func (gp *Program) WriteToDisk(path string) error {
	gp.workdir = path

	for i, f := range gp.files {
		fileName := fmt.Sprintf("prog%v_%v.go", gp.id, f.pack)
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
		return nil
	}

	file := gp.files[0]

	fset := token.NewFileSet()

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
func (gp *Program) Compile(arch string, fz FuzzOptions) (string, error) {
	if gp.files[0].path == nil {
		return "", errors.New("cannot compile program with no *File")
	}

	arcName := strings.TrimSuffix(gp.files[0].name, "_a.go") + "_main.o"
	binName := strings.TrimSuffix(gp.files[0].name, "_a.go")

	switch {

	case strings.Contains(fz.Toolchain, "gccgo"):
		oFlag := "-O2"
		if fz.Noopt {
			oFlag = "-Og"
		}
		cmd := exec.Command(fz.Toolchain, oFlag, "-o", arcName, gp.files[0].name)
		cmd.Dir = gp.workdir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), err
		}

	case strings.Contains(fz.Toolchain, "tinygo"):
		var cmd *exec.Cmd
		if fz.Noopt {
			cmd = exec.Command(fz.Toolchain, "build", "-opt", "z", "-o", arcName, gp.files[0].name)
		} else {
			cmd = exec.Command(fz.Toolchain, "build", "-o", arcName, gp.files[0].name)
		}
		cmd.Dir = gp.workdir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), err
		}

	default:

		// Setup build args
		buildArgs := []string{"tool", "compile"}
		if fz.Race {
			buildArgs = append(buildArgs, "-race")
		}
		if fz.Noopt {
			buildArgs = append(buildArgs, "-N", "-l")
		}
		if fz.Ssacheck {
			cs := fmt.Sprintf("-d=ssa/check/seed=%v", CheckSeed)
			buildArgs = append(buildArgs, cs)
		}

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

		for _, file := range gp.files {
			var cmdArgs []string
			if file.pack == "main" {
				cmdArgs = append(buildArgs, "-I=.")
				cmdArgs = append(cmdArgs, file.name)
			} else {
				cmdArgs = append(buildArgs, file.name)
			}

			cmd := exec.Command(fz.Toolchain, cmdArgs...)
			cmd.Dir, cmd.Env = gp.workdir, env
			out, err := cmd.CombinedOutput()
			if err != nil {
				return string(out), err
			}
		}

		// link
		linkArgs := []string{"tool", "link", "-L=."}
		if fz.Race {
			linkArgs = append(linkArgs, "-race")
		}
		linkArgs = append(linkArgs, "-o", binName, arcName)
		cmd := exec.Command(fz.Toolchain, linkArgs...)
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
	binPath := strings.TrimSuffix(gp.files[0].path.Name(), "_a.go")
	for _, file := range gp.files {
		err := os.Remove(binPath + "_" + file.pack + ".o")
		if err != nil {
			log.Printf("could not remove %s: %s", binPath+".o", err)
		}

		// ignore error since some toolchains don't write a binary
		_ = os.Remove(binPath)
	}
}

// DeleteSource deletes the file containing the source code of gp, as
// written to disk from gp.WriteTofdFile.
func (gp Program) DeleteSource() {
	for _, file := range gp.files {
		_ = os.Remove(file.path.Name())
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

	for _, file := range gp.files {
		err := os.Rename(file.path.Name(), fld+"/"+file.name)
		if err != nil {
			fmt.Printf("Move crasher to folder failed: %v", err)
			os.Exit(2)
		}
	}
}

func (p Program) String() string {
	// TODO(alb): fix
	return string(p.files[0].source)
}

func (p Program) Name() string {
	// TODO(alb): fix
	return p.files[0].name
}
