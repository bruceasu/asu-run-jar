package runner

import (
	"archive/zip"
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Run dispatches a JAR, Java source file, or JShell script to its runtime.
func Run(input string, args []string, dryRun bool, jdkSelector string) error {
	abs, err := filepath.Abs(input)
	if err != nil {
		return fmt.Errorf("resolve input path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("input not found: %s", input)
	}
	if info.IsDir() {
		return fmt.Errorf("run expects a .jar, .java, or .jsh file; got directory %s", input)
	}
	switch strings.ToLower(filepath.Ext(abs)) {
	case ".jar":
		return RunJar(abs, args, dryRun, jdkSelector)
	case ".java":
		return RunJava(abs, args, dryRun, jdkSelector)
	case ".jsh":
		return RunJShell(abs, args, dryRun, jdkSelector)
	default:
		return fmt.Errorf("unsupported input type %q; use .jar, .java, or .jsh", filepath.Ext(abs))
	}
}

// RunJar runs ordinary, dependency-aware, and fat JARs produced by ajava.
func RunJar(jarPath string, args []string, dryRun bool, jdkSelector string) error {
	requiredJava, err := jarRequiredJavaVersion(jarPath)
	if err != nil {
		return fmt.Errorf("inspect JAR Java version: %w", err)
	}
	java, err := toolForJDK(jdkSelector, "java")
	if err != nil {
		if jdkSelector == "" {
			java = adjacentJava(filepath.Dir(jarPath), requiredJava)
			if java != "" {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	} else if jdkSelector == "" && requiredJava > 0 {
		major, versionErr := javaMajorVersion(java)
		if versionErr == nil && major < requiredJava {
			if localJava := adjacentJava(filepath.Dir(jarPath), requiredJava); localJava != "" {
				java = localJava
			} else {
				return fmt.Errorf("JAR requires Java %d or newer, but %s is Java %d; place a suitable runtime in a sibling jre/ directory", requiredJava, java, major)
			}
		}
	}
	jarAbs, err := filepath.Abs(jarPath)
	if err != nil {
		return fmt.Errorf("resolve JAR path: %w", err)
	}
	r, err := zip.OpenReader(jarAbs)
	if err != nil {
		return fmt.Errorf("open JAR %s: %w", jarPath, err)
	}
	defer r.Close()

	var manifest, depsFile, depsStatic *zip.File
	for _, file := range r.File {
		switch strings.ToLower(file.Name) {
		case "meta-inf/manifest.mf":
			manifest = file
		case "meta-inf/dependencies.lib":
			depsFile = file
		case "meta-inf/dependencies-static.lib":
			depsStatic = file
		}
	}
	if depsStatic != nil || depsFile == nil {
		return runCommand(java, append([]string{"-jar", jarAbs}, args...), filepath.Dir(jarAbs), dryRun)
	}
	if manifest == nil {
		return fmt.Errorf("JAR %s has dependencies.lib but no manifest", jarPath)
	}
	mainClass, err := readMainClass(manifest)
	if err != nil {
		return err
	}
	if mainClass == "" {
		return fmt.Errorf("JAR %s has no Main-Class in its manifest", jarPath)
	}
	dependencies, err := readDependencyList(depsFile, filepath.Dir(jarAbs))
	if err != nil {
		return err
	}
	classpath := append([]string{jarAbs}, dependencies...)
	commandArgs := []string{"-cp", strings.Join(classpath, string(filepath.ListSeparator)), mainClass}
	commandArgs = append(commandArgs, args...)
	return runCommand(java, commandArgs, filepath.Dir(jarAbs), dryRun)
}

func adjacentJava(directory string, minimumMajor int) string {
	for _, home := range adjacentJDKHomes(directory) {
		candidate := filepath.Join(home, "bin", executableName("java"))
		if !fileExists(candidate) {
			continue
		}
		if minimumMajor > 0 {
			major, err := javaMajorVersion(candidate)
			if err != nil || major < minimumMajor {
				continue
			}
		}
		return candidate
	}
	return ""
}

func jarRequiredJavaVersion(jarPath string) (int, error) {
	archive, err := zip.OpenReader(jarPath)
	if err != nil {
		return 0, err
	}
	defer archive.Close()
	var manifest *zip.File
	for _, file := range archive.File {
		if strings.EqualFold(file.Name, "META-INF/MANIFEST.MF") {
			manifest = file
			break
		}
	}
	if manifest == nil {
		return 0, nil
	}
	mainClass, err := readMainClass(manifest)
	if err != nil || mainClass == "" {
		return 0, err
	}
	classPath := strings.ReplaceAll(mainClass, ".", "/") + ".class"
	for _, file := range archive.File {
		if file.Name != classPath {
			continue
		}
		r, err := file.Open()
		if err != nil {
			return 0, err
		}
		var header [8]byte
		_, readErr := io.ReadFull(r, header[:])
		closeErr := r.Close()
		if readErr != nil {
			return 0, readErr
		}
		if closeErr != nil {
			return 0, closeErr
		}
		major := int(header[6])<<8 | int(header[7])
		if major >= 45 {
			return major - 44, nil
		}
		return 0, nil
	}
	return 0, nil
}

func readMainClass(file *zip.File) (string, error) {
	rc, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("read JAR manifest: %w", err)
	}
	defer rc.Close()
	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.ToLower(line), "main-class:") {
			return strings.TrimSpace(strings.SplitN(line, ":", 2)[1]), nil
		}
	}
	return "", scanner.Err()
}

func readDependencyList(file *zip.File, jarDir string) ([]string, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("read JAR dependency list: %w", err)
	}
	defer rc.Close()
	var dependencies []string
	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		path := line
		if !filepath.IsAbs(path) {
			path = filepath.Join(jarDir, filepath.FromSlash(path))
		}
		dependencies = append(dependencies, path)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read JAR dependency list: %w", err)
	}
	return dependencies, nil
}

func toolForJDK(selector, name string) (string, error) {
	if selector != "" {
		home, err := resolveJDKHome(selector)
		if err != nil {
			return "", err
		}
		binary := name
		if os.PathSeparator == '\\' {
			binary += ".exe"
		}
		path := filepath.Join(home, "bin", binary)
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("JDK %q does not provide %s at %s", selector, name, path)
		}
		return path, nil
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	if home := os.Getenv("JAVA_HOME"); home != "" {
		binary := name
		if os.PathSeparator == '\\' {
			binary += ".exe"
		}
		path := filepath.Join(home, "bin", binary)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("%s not found in PATH or JAVA_HOME; install a JDK and ensure its bin directory is available", name)
}

func resolveJDKHome(selector string) (string, error) {
	if info, err := os.Stat(selector); err == nil {
		if !info.IsDir() {
			base := strings.ToLower(filepath.Base(selector))
			if base == "java" || base == "java.exe" || base == "javac" || base == "javac.exe" || base == "jshell" || base == "jshell.exe" {
				return filepath.Dir(filepath.Dir(selector)), nil
			}
			return "", fmt.Errorf("--jdk expects a JDK home or bin directory, got file %q", selector)
		}
		home := selector
		if strings.EqualFold(filepath.Base(filepath.Clean(home)), "bin") {
			home = filepath.Dir(home)
		}
		return filepath.Abs(home)
	}
	if filepath.IsAbs(selector) || strings.ContainsAny(selector, `/\\:`) {
		return "", fmt.Errorf("JDK path does not exist: %s", selector)
	}

	version := normalizeJDKVersion(selector)
	if version != "" {
		keys := []string{"JAVA_HOME_" + version, "JAVA" + version + "_HOME", "JDK" + version + "_HOME"}
		if version == "8" {
			keys = append(keys, "JAVA_HOME_1_8", "JDK_HOME_8")
		}
		for _, key := range keys {
			if home := os.Getenv(key); home != "" {
				if resolved, ok := jdkHomeAtVersion(home, version); ok {
					return resolved, nil
				}
			}
		}
		if home, ok := findJDKHomeOnPath(version); ok {
			return home, nil
		}
		return "", fmt.Errorf("JDK %s was not found in its environment variables or PATH; set JAVA_HOME_%s or pass its JDK home path to --jdk", version, version)
	}
	return "", fmt.Errorf("--jdk expects a JDK version (such as 8 or 17) or JDK home directory, got %q", selector)
}

func jdkHomeAtVersion(home, version string) (string, bool) {
	info, err := os.Stat(home)
	if err != nil || !info.IsDir() {
		return "", false
	}
	if strings.EqualFold(filepath.Base(filepath.Clean(home)), "bin") {
		home = filepath.Dir(home)
	}
	binary := "java"
	if os.PathSeparator == '\\' {
		binary += ".exe"
	}
	java := filepath.Join(home, "bin", binary)
	if _, err := os.Stat(java); err != nil {
		return "", false
	}
	major, err := javaMajorVersion(java)
	if err != nil || strconv.Itoa(major) != version {
		return "", false
	}
	abs, err := filepath.Abs(home)
	return abs, err == nil
}

func findJDKHomeOnPath(version string) (string, bool) {
	binary := "java"
	if os.PathSeparator == '\\' {
		binary += ".exe"
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		java := filepath.Join(dir, binary)
		if _, err := os.Stat(java); err != nil {
			continue
		}
		major, err := javaMajorVersion(java)
		if err != nil || strconv.Itoa(major) != version {
			continue
		}
		if home, ok := jdkHomeForExecutable(java); ok {
			return home, true
		}
	}
	return "", false
}

func normalizeJDKVersion(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(strings.ToLower(value), "jdk")
	if value == "1.8" {
		return "8"
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return ""
		}
	}
	return value
}

func runCommand(program string, args []string, dir string, dryRun bool) error {
	if dryRun {
		parts := make([]string, 0, len(args)+1)
		parts = append(parts, strconv.Quote(program))
		for _, arg := range args {
			parts = append(parts, strconv.Quote(arg))
		}
		fmt.Println(strings.Join(parts, " "))
		return nil
	}
	cmd := exec.Command(program, args...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr
		}
		return fmt.Errorf("start %s: %w", program, err)
	}
	return nil
}

func makeJShellBootstrap(path string, args []string) error {
	var source strings.Builder
	source.WriteString("String[] args = new String[]{")
	for i, arg := range args {
		if i > 0 {
			source.WriteString(", ")
		}
		source.WriteString(strconv.Quote(arg))
	}
	source.WriteString("};\n")
	return os.WriteFile(path, []byte(source.String()), 0o600)
}
