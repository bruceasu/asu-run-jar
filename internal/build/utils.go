package build

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Run executes a command and returns stdout, stderr, and error
func Run(cmd *exec.Cmd) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// RunCommandDir executes a command in a specific directory
func RunCommandDir(dir, name string, args ...string) (string, string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return Run(cmd)
}

// SamePath checks if two paths refer to the same location (case-insensitive on Windows)
func SamePath(a, b string) bool {
	ap, err := filepath.Abs(a)
	if err != nil {
		ap = filepath.Clean(a)
	} else {
		ap = filepath.Clean(ap)
	}
	bp, err := filepath.Abs(b)
	if err != nil {
		bp = filepath.Clean(b)
	} else {
		bp = filepath.Clean(bp)
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(ap, bp)
	}
	return ap == bp
}

// Info prints an info message
func Info(f string, a ...any) {
	fmt.Printf("[INFO] "+f+"\n", a...)
}

// Ok prints a success message
func Ok(f string, a ...any) {
	fmt.Printf("[OK]   "+f+"\n", a...)
}

// Fail prints an error message and exits with code 1
func Fail(err error) {
	fmt.Fprintf(os.Stderr, "[ERR]  %v\n", err)
	os.Exit(1)
}

// MustAbs returns the absolute path or fails
func MustAbs(p string) string {
	ap, err := filepath.Abs(p)
	if err != nil {
		Fail(err)
	}
	return ap
}

// MustGetwd returns the current working directory or fails
func MustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		Fail(err)
	}
	return wd
}

// MustRel returns the relative path or fails
func MustRel(base, p string) string {
	rel, err := filepath.Rel(base, p)
	if err != nil {
		Fail(err)
	}
	return rel
}

// ToSlash converts path to use forward slashes
func ToSlash(p string) string {
	return filepath.ToSlash(p)
}
