package nob
// NOTE: inspired by Tsoding: https://github.com/tsoding/nob.h

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"time"
	"runtime"
	"errors"
	"path/filepath"
	fstring "github.com/Fipaan/lib.go/string"
	fpath "github.com/Fipaan/lib.go/path"
)

func ProgramPath() string {
	return os.Args[0]
}
func ProgramName() string {
	return filepath.Base(ProgramPath())
}
func ProgramDir() string {
	return filepath.Dir(ProgramPath())
}
func SourceName(skip int) string {
	_, file, _, ok := runtime.Caller(skip + 1)
	if !ok { return "" }
	return file
}

type Cmd struct {
	Args       []string
	Dir        string
	Stdout    *os.File
	Stderr    *os.File
	Stdin     *os.File
	ResetOnRun bool
	Pipe      *Cmd
}

func (cmd *Cmd) Reset(args ...string) {
	cmd.Args = args
	cmd.Pipe = nil
}

func CmdInit(args ...string) *Cmd {
	cmd := Cmd{
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		Stdin:      os.Stdin,
		ResetOnRun: true,
	}
	cmd.Reset(args...)
	return &cmd
}

func (cmd *Cmd) WalkIn(format string, args ...any) string {
	oldDir := cmd.Dir
	cmd.Dir = path.Join(cmd.Dir, fmt.Sprintf(format, args...))
	return oldDir
}

func CleanArg(arg string) string {
	if fstring.ContainsSpace(arg) {
		arg = fstring.Stringify(arg)
	}
	return arg
}

func (cmd *Cmd) printCmd() bool {
	if len(cmd.Args) == 0 {
		fmt.Printf("ERROR: program name wasn't provided\n")
		return false
	}
	name := cmd.Args[0]
	args := cmd.Args[1:]
	prefix := "CMD"
	if !fpath.ComparePaths(".", cmd.Dir) {
		localDir, localErr := filepath.Localize(cmd.Dir)
		if localErr != nil {
			localDir = cmd.Dir
		}
		prefix = fmt.Sprintf("%v(%v)", prefix, localDir)
	}
	fmt.Printf("%v: %v", prefix, CleanArg(name))
	for i := 0; i < len(args); i++ {
		fmt.Printf(" %v", CleanArg(args[i]))
	}
	return true
}

func (cmd *Cmd) Run() bool {
	if len(cmd.Args) == 0 {
		fmt.Printf("ERROR: program name wasn't provided\n")
		return false
	}
	name := cmd.Args[0]
	args := cmd.Args[1:]
	_cmd := exec.Command(name, args...)
	_cmd.Stdout = cmd.Stdout
	_cmd.Stderr = cmd.Stderr
	_cmd.Stdin  = cmd.Stdin
	_cmd.Dir    = cmd.Dir
	var otherCmd *exec.Cmd
	if cmd.Pipe != nil {
		if cmd.Pipe.Pipe != nil {
			fmt.Printf("ERROR: multiple pipes are not supported\n")
			return false;
		}
		_cmd.Stdout = nil
		pipe, err := _cmd.StdoutPipe()
		if err != nil {
			fmt.Printf("ERROR: couldn't pipe: %v\n", err)
			return false
		}
		otherCmd = exec.Command(name, args...)
		otherCmd.Stdin  = pipe
		otherCmd.Stdout = cmd.Pipe.Stdout
		otherCmd.Stderr = cmd.Pipe.Stderr
		otherCmd.Dir    = cmd.Pipe.Dir
		if !cmd.printCmd() { return false }
		fmt.Printf(" |\n")
		if !cmd.Pipe.printCmd() { return false }
		fmt.Printf("\n")

		if err := _cmd.Start(); err != nil {
			fmt.Printf("ERROR: %v\n", err)
			return false
		}
		if err := otherCmd.Start(); err != nil {
			fmt.Printf("ERROR: %v\n", err)
			return false
		}
    	
		if err := _cmd.Wait(); err != nil {
			fmt.Printf("ERROR: %v\n", err)
			return false
		}
		if err := otherCmd.Wait(); err != nil {
			fmt.Printf("ERROR: %v\n", err)
			return false
		}
	} else {
		if !cmd.printCmd() { return false }
		fmt.Printf("\n")
		if err := _cmd.Run(); err != nil {
			return false
		}
	}
	if cmd.ResetOnRun { cmd.Reset() }
	return true
}

func (cmd *Cmd) Push(args ...string) {
	cmd.Args = append(cmd.Args, args...)
}

func GetModTime(path string) (modTime time.Time, err error) {
	file, err := os.Open(path)
	defer file.Close()
	if err != nil { return }
	info, err := file.Stat()
	if err == nil {
		modTime = info.ModTime()
	}
	return
}

func Touch(path string) error {
    now := time.Now()

    if _, err := os.Stat(path); os.IsNotExist(err) {
        f, err := os.OpenFile(path, os.O_CREATE, 0644)
        if err != nil {
            return err
        }
        return f.Close()
    }

    return os.Chtimes(path, now, now)
}

func FileExist(path string) (bool, error) {
    _, err := os.Stat(path)
    if err == nil {
        return true, nil
    }
    if os.IsNotExist(err) {
        return false, nil
    }
    return false, err
}

func MkdirIfNotExists(path string) error {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return nil
		}
		return fmt.Errorf("%s exists but is not a directory", path)
	}

	if os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}

	return err
}

func NeedsRebuild(outPath string, srcPaths ...string) (needs bool, err error) {
	var outTime time.Time
	outTime, err = GetModTime(outPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			needs = false
			err   = nil
		}
		return
	}
	for i := 0; i < len(srcPaths); i++ {
		srcPath := srcPaths[i]
		var srcTime time.Time
		srcTime, err = GetModTime(srcPath)
		if err != nil { return }
		if srcTime.After(outTime) {
			needs = true
			return
		}
	}
	return
}
func GoRebuildUrself(cmdArgs ...string) {
	_, src_path, _, ok := runtime.Caller(1)
	if ok {
		needsRebuild, err := NeedsRebuild(ProgramPath(), src_path)
		if err != nil {
			fmt.Printf("ERROR: %v", err)
			os.Exit(1)
		}
		if !needsRebuild { return }
	}
	fmt.Printf("INFO: Rebuilding...\n")
	cmd := CmdInit(cmdArgs...)
	cmd.Dir = ProgramDir()
	if !cmd.Run() {
		os.Exit(1)
	}
	cmd.Push(os.Args...)
	if !cmd.Run() {
		os.Exit(1)
	}
	os.Exit(0)
}
