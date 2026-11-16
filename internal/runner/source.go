package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/bruceasu/asu-run-jar/internal/build"
)

// RunJava compiles and runs a Java source file together with its local
// //SOURCES and Maven/local //DEPS declarations.
func RunJava(source string, args []string, dryRun bool, jdkSelector string) error {
	absSource, err := filepath.Abs(source)
	if err != nil {
		return fmt.Errorf("resolve Java source: %w", err)
	}
	directives, err := build.ReadScriptDirectives(absSource)
	if err != nil {
		return err
	}
	selectedJDK := jdkSelector
	if selectedJDK == "" {
		selectedJDK = directives.JDK
	}
	sources, err := build.ResolveScriptSources(absSource)
	if err != nil {
		return err
	}
	dependencies, cleanupDependencies, err := build.CollectScriptDependencies(absSource)
	if err != nil {
		return err
	}
	if cleanupDependencies != nil {
		defer cleanupDependencies()
	}

	tempRoot, err := os.MkdirTemp("", "ajava-run-")
	if err != nil {
		return fmt.Errorf("create temporary run directory: %w", err)
	}
	defer os.RemoveAll(tempRoot)
	classesDir := filepath.Join(tempRoot, "classes")
	if err := os.MkdirAll(classesDir, 0o755); err != nil {
		return err
	}
	if err := build.CopyScriptFiles(classesDir, directives.Files); err != nil {
		return err
	}
	workingDir := filepath.Dir(absSource)
	if err := build.StageScriptFiles(workingDir, directives.Files); err != nil {
		return err
	}
	java, effectiveJDK, err := sourceJDK(absSource, selectedJDK)
	if err != nil {
		return err
	}
	javaMajor, err := javaMajorVersion(java)
	if err != nil {
		return err
	}
	classpath := append([]string{classesDir}, dependencies...)
	if javaMajor >= 11 && len(sources) == 1 && len(directives.CompileOptions) == 0 {
		commandArgs := append([]string(nil), directives.RuntimeOptions...)
		commandArgs = append(commandArgs, "--class-path", strings.Join(classpath, string(filepath.ListSeparator)), absSource)
		commandArgs = append(commandArgs, args...)
		return runCommand(java, commandArgs, workingDir, dryRun)
	}

	javac, err := toolForJDK(effectiveJDK, "javac")
	if err != nil {
		return err
	}
	if err := build.JavacCompileWithCompiler(javac, sources, classesDir, dependencies, directives.CompileOptions); err != nil {
		return err
	}
	mainClass, err := build.InferMainFromSingle(filepath.Dir(absSource), absSource)
	if err != nil {
		return fmt.Errorf("infer main class from %s: %w", source, err)
	}
	commandArgs := append([]string(nil), directives.RuntimeOptions...)
	commandArgs = append(commandArgs, "-cp", strings.Join(classpath, string(filepath.ListSeparator)), mainClass)
	commandArgs = append(commandArgs, args...)
	return runCommand(java, commandArgs, workingDir, dryRun)
}

var javaVersionPattern = regexp.MustCompile(`(?i)(?:version\s+)?"?(\d+)(?:\.(\d+))?`)

func javaMajorVersion(java string) (int, error) {
	output, err := exec.Command(java, "-version").CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("query Java version from %s: %w", java, err)
	}
	matches := javaVersionPattern.FindStringSubmatch(string(output))
	if len(matches) < 2 {
		return 0, fmt.Errorf("could not determine Java version from %s output: %s", java, strings.TrimSpace(string(output)))
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("parse Java version from %s output: %w", java, err)
	}
	if major == 1 && len(matches) > 2 && matches[2] != "" {
		major, err = strconv.Atoi(matches[2])
		if err != nil {
			return 0, fmt.Errorf("parse Java version from %s output: %w", java, err)
		}
	}
	return major, nil
}

// RunJShell runs a .jsh script with the local JDK's JShell. Java files listed
// through //SOURCES are compiled first and made available on the classpath.
func RunJShell(source string, args []string, dryRun bool, jdkSelector string) error {
	absSource, err := filepath.Abs(source)
	if err != nil {
		return fmt.Errorf("resolve JShell script: %w", err)
	}
	directives, err := build.ReadScriptDirectives(absSource)
	if err != nil {
		return err
	}
	selectedJDK := jdkSelector
	if selectedJDK == "" {
		selectedJDK = directives.JDK
	}
	sources, err := build.ResolveScriptSources(absSource)
	if err != nil {
		return err
	}
	var javaSources []string
	for _, path := range sources {
		if strings.EqualFold(filepath.Ext(path), ".java") {
			javaSources = append(javaSources, path)
		}
	}
	dependencies, cleanupDependencies, err := build.CollectScriptDependencies(absSource)
	if err != nil {
		return err
	}
	if cleanupDependencies != nil {
		defer cleanupDependencies()
	}
	jshell, err := toolForJDK(selectedJDK, "jshell")
	if err != nil {
		return err
	}
	effectiveJDK := selectedJDK
	if effectiveJDK == "" {
		if home, ok := jdkHomeForExecutable(jshell); ok {
			effectiveJDK = home
		}
	}

	tempRoot, err := os.MkdirTemp("", "ajava-jsh-")
	if err != nil {
		return fmt.Errorf("create temporary JShell directory: %w", err)
	}
	defer os.RemoveAll(tempRoot)
	classesDir := filepath.Join(tempRoot, "classes")
	if err := os.MkdirAll(classesDir, 0o755); err != nil {
		return err
	}
	if len(javaSources) > 0 {
		javac, err := toolForJDK(effectiveJDK, "javac")
		if err != nil {
			return err
		}
		if err := build.JavacCompileWithCompiler(javac, javaSources, classesDir, dependencies, directives.CompileOptions); err != nil {
			return err
		}
	}
	if err := build.CopyScriptFiles(classesDir, directives.Files); err != nil {
		return err
	}
	workingDir := filepath.Dir(absSource)
	if err := build.StageScriptFiles(workingDir, directives.Files); err != nil {
		return err
	}

	bootstrap := filepath.Join(tempRoot, "bootstrap.jsh")
	if err := makeJShellBootstrap(bootstrap, args); err != nil {
		return fmt.Errorf("prepare JShell arguments: %w", err)
	}
	classpath := append([]string(nil), dependencies...)
	classpath = append([]string{classesDir}, classpath...)
	commandArgs := []string{"-s"}
	for _, option := range directives.RuntimeOptions {
		commandArgs = append(commandArgs, "-R"+option)
	}
	for _, option := range directives.CompileOptions {
		commandArgs = append(commandArgs, "-C"+option)
	}
	if len(classpath) > 0 {
		commandArgs = append(commandArgs, "--class-path", strings.Join(classpath, string(filepath.ListSeparator)))
	}
	commandArgs = append(commandArgs, "--startup", bootstrap, absSource)
	return runCommand(jshell, commandArgs, workingDir, dryRun)
}

func jdkHomeForExecutable(executable string) (string, bool) {
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		resolved = executable
	}
	binDir := filepath.Dir(filepath.Clean(resolved))
	if !strings.EqualFold(filepath.Base(binDir), "bin") {
		return "", false
	}
	home, err := filepath.Abs(filepath.Dir(binDir))
	return home, err == nil
}

func sourceJDK(source, selector string) (java, home string, err error) {
	if selector != "" {
		version := normalizeJDKVersion(selector)
		java, toolErr := toolForJDK(selector, "java")
		if toolErr == nil {
			home, err = resolveJDKHome(selector)
			if err != nil {
				return "", "", err
			}
			if fileExists(filepath.Join(home, "bin", executableName("javac"))) {
				return java, home, nil
			}
			toolErr = fmt.Errorf("JDK %q does not provide javac", selector)
		}
		if version != "" {
			for _, candidate := range adjacentJDKHomes(filepath.Dir(source)) {
				candidateJava := filepath.Join(candidate, "bin", executableName("java"))
				candidateJavac := filepath.Join(candidate, "bin", executableName("javac"))
				if !fileExists(candidateJava) || !fileExists(candidateJavac) {
					continue
				}
				major, versionErr := javaMajorVersion(candidateJava)
				if versionErr == nil && strconv.Itoa(major) == version {
					absHome, absErr := filepath.Abs(candidate)
					if absErr == nil {
						return candidateJava, absHome, nil
					}
				}
			}
		}
		return "", "", toolErr
	}

	if javac, lookupErr := exec.LookPath(executableName("javac")); lookupErr == nil {
		if home, ok := jdkHomeForExecutable(javac); ok {
			java = filepath.Join(home, "bin", executableName("java"))
			if fileExists(java) {
				return java, home, nil
			}
		}
	}
	if home := os.Getenv("JAVA_HOME"); home != "" {
		javac := filepath.Join(home, "bin", executableName("javac"))
		java := filepath.Join(home, "bin", executableName("java"))
		if fileExists(javac) && fileExists(java) {
			absHome, absErr := filepath.Abs(home)
			if absErr == nil {
				return java, absHome, nil
			}
		}
	}
	for _, candidate := range adjacentJDKHomes(filepath.Dir(source)) {
		java := filepath.Join(candidate, "bin", executableName("java"))
		javac := filepath.Join(candidate, "bin", executableName("javac"))
		if fileExists(java) && fileExists(javac) {
			return java, candidate, nil
		}
	}
	return "", "", fmt.Errorf("no suitable JDK found in PATH, JAVA_HOME, or beside %s; install a JDK or place a compact JDK in a sibling jre/ directory", filepath.Base(source))
}

func adjacentJDKHomes(sourceDir string) []string {
	return []string{
		filepath.Join(sourceDir, "jre"),
		filepath.Join(sourceDir, "runtime"),
	}
}

func executableName(name string) string {
	if os.PathSeparator == '\\' {
		return name + ".exe"
	}
	return name
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
