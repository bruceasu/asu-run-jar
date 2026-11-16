package build

import (
	"archive/zip"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// CollectJavaFiles 收集指定目录下的所有 Java 文件
func CollectJavaFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".java") {
			ap, err := filepath.Abs(p)
			if err != nil {
				return err
			}
			files = append(files, ap)
		}
		return nil
	})
	return files, err
}

var pkgRe = regexp.MustCompile(`(?m)^\s*package\s+([a-zA-Z0-9_\.]+)\s*;`)

// InferMainFromSingle 从单个 Java 文件推断主类名
func InferMainFromSingle(srcRoot, onlyFile string) (string, error) {
	// 尝试解析 package
	if b, err := os.ReadFile(onlyFile); err == nil {
		if m := pkgRe.FindSubmatch(b); len(m) == 2 {
			pkg := string(m[1])
			clz := strings.TrimSuffix(filepath.Base(onlyFile), ".java")
			return pkg + "." + clz, nil
		}
	}
	// 按路径推断
	srcAbs, err := filepath.Abs(srcRoot)
	if err != nil {
		return "", err
	}
	rel := MustRel(srcAbs, onlyFile)
	noExt := strings.TrimSuffix(rel, ".java")
	fqcn := ToSlash(noExt)
	fqcn = strings.ReplaceAll(fqcn, "/", ".")
	return fqcn, nil
}

func findJavaSourceForClass(javaFiles []string, className string) string {
	className = strings.TrimSpace(className)
	if className == "" {
		return ""
	}
	if i := strings.IndexByte(className, '$'); i >= 0 {
		className = className[:i]
	}
	for _, source := range javaFiles {
		baseName := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
		qualifiedName := baseName
		if data, err := os.ReadFile(source); err == nil {
			if match := pkgRe.FindSubmatch(data); len(match) == 2 {
				qualifiedName = string(match[1]) + "." + baseName
			}
		}
		if qualifiedName == className {
			return source
		}
	}
	return ""
}

// JavacCompile 编译 Java 源文件
func JavacCompile(javaFiles []string, out string, deps []string) error {
	return JavacCompileWithOptions(javaFiles, out, deps, nil)
}

// JavacCompileWithOptions compiles source files with additional javac flags.
func JavacCompileWithOptions(javaFiles []string, out string, deps, options []string) error {
	return JavacCompileWithCompiler("javac", javaFiles, out, deps, options)
}

// JavacCompileWithCompiler compiles sources using a specified javac executable.
func JavacCompileWithCompiler(compiler string, javaFiles []string, out string, deps, options []string) error {
	argsFile, err := os.CreateTemp("", "javac-sources-*.txt")
	if err != nil {
		return fmt.Errorf("create temp args file: %w", err)
	}
	defer os.Remove(argsFile.Name())

	for _, f := range javaFiles {
		fmt.Fprintln(argsFile, f)
	}
	if err := argsFile.Close(); err != nil {
		return err
	}

	cp := []string{out}
	cp = append(cp, deps...)

	// 打印 javac 命令
	fmt.Println("[INFO] javac command:")
	fmt.Print("javac  -parameters -encoding utf-8 -d ", out, " -cp ", strings.Join(cp, string(os.PathListSeparator)), " @"+argsFile.Name())
	fmt.Println()

	args := []string{"-parameters", "-encoding", "utf-8", "-d", out, "-cp", strings.Join(cp, string(os.PathListSeparator))}
	args = append(args, options...)
	args = append(args, "@"+argsFile.Name())
	cmd := exec.Command(compiler, args...)
	outb, errb, err := Run(cmd)
	if err != nil {
		return fmt.Errorf("javac failed: %v\n%s\n%s", err, outb, errb)
	}
	return nil
}

// BuildJavaProject 处理普通 Java 项目的完整构建流程
func BuildJavaProject(
	srcDir, outDir, jarName, mainFQCN string,
	skipJar, staticMode, expLib bool,
) (jarPath, mainClass string, err error) {
	// 收集源码
	javaFiles, err := CollectJavaFiles(srcDir)
	if err != nil {
		return "", "", err
	}
	if len(javaFiles) == 0 {
		return "", "", fmt.Errorf("no .java files under %q", srcDir)
	}
	rootJavaFiles := append([]string(nil), javaFiles...)
	var scriptFiles []ScriptFile
	var compileOptions []string
	seenSources := map[string]bool{}
	for index := 0; index < len(javaFiles); index++ {
		source := javaFiles[index]
		key := filepath.Clean(source)
		if seenSources[key] {
			continue
		}
		seenSources[key] = true
		directives, err := ReadScriptDirectives(source)
		if err != nil {
			return "", "", err
		}
		javaFiles = append(javaFiles, directives.Sources...)
		scriptFiles = append(scriptFiles, directives.Files...)
		compileOptions = append(compileOptions, directives.CompileOptions...)
	}
	javaFiles = uniquePaths(javaFiles)
	// 推断/获取主类
	mainClass = strings.TrimSpace(mainFQCN)
	if mainClass == "" {
		if len(rootJavaFiles) == 1 {
			mc, err := InferMainFromSingle(srcDir, rootJavaFiles[0])
			if err != nil {
				return "", "", fmt.Errorf("failed to infer main class: %w", err)
			}
			mainClass = mc
			Info("[MAIN] Single source; inferred main: %s", mainClass)
		} else {
			Info("[HINT] Multiple source files detected. Set -main com.example.Main if you want a runnable JAR.")
		}
	}
	deps, cleanupDeps, err := CollectDeps(javaFiles)
	if err != nil {
		return "", "", err
	}
	if cleanupDeps != nil {
		defer cleanupDeps()
	}

	// 创建输出目录
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create out dir: %w", err)
	}

	classesDir := filepath.Join(filepath.Dir(srcDir), "classes")
	os.MkdirAll(classesDir, 0755)

	// 编译
	if err := JavacCompileWithOptions(javaFiles, classesDir, deps, compileOptions); err != nil {
		return "", "", err
	}
	if err := CopyScriptFiles(classesDir, scriptFiles); err != nil {
		return "", "", err
	}
	Ok("Compiled %d source file(s) to %s", len(javaFiles), classesDir)

	// 打 JAR（可选）
	if !skipJar && mainClass != "" {
		jarPath = filepath.Join(outDir, jarName)
		if err := BuildJar(jarPath, classesDir, mainClass, staticMode, deps); err != nil {
			return "", "", err
		}
		Ok("Built runnable JAR: %s", jarPath)
		Info(`[RUN] java -jar "%s"`, jarPath)
	} else {
		mainClassHint := mainClass
		if mainClassHint == "" {
			mainClassHint = "YourMainClass"
		}
		Info(`[RUN] java -cp "%s" %s`, classesDir, mainClassHint)
	}

	// 导出依赖
	if expLib {
		CopyExplib(outDir, deps)
	}

	return jarPath, mainClass, nil
}

// CollectDeps 收集依赖：lib/**/*.jar + dep.lib
func CollectDeps(javaFiles []string) ([]string, func(), error) {
	var deps []string
	var mavenSpecs []MavenDependency

	// 1. lib/**/*.jar
	filepath.WalkDir("lib", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".jar") {
			abs, _ := filepath.Abs(p)
			deps = append(deps, abs)
		}
		return nil
	})

	// 2. dep.lib
	depFile := "dep.lib"
	if b, err := os.ReadFile(depFile); err == nil {
		scanner := bufio.NewScanner(bytes.NewReader(b))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			if spec, ok := ParseMavenDependency(line); ok {
				mavenSpecs = append(mavenSpecs, spec)
				continue
			}

			// 绝对路径
			if filepath.IsAbs(line) {
				if _, err := os.Stat(line); err == nil {
					deps = append(deps, line)
				} else {
					fmt.Printf("[WARN] dependency not found: %s\n", line)
				}
				continue
			}

			// 相对路径
			if _, err := os.Stat(line); err == nil {
				abs, _ := filepath.Abs(line)
				deps = append(deps, abs)
			} else {
				home, _ := os.UserHomeDir()
				alt := filepath.Join(home, ".m2", "repository", line)
				if _, err := os.Stat(alt); err == nil {
					deps = append(deps, alt)
				} else {
					fmt.Printf("[WARN] dependency not found: %s\n", line)
				}
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, nil, fmt.Errorf("read dep.lib: %w", err)
		}
	}

	declarations, err := CollectJavaDependencyDeclarations(javaFiles)
	if err != nil {
		return nil, nil, err
	}
	mavenSpecs = append(mavenSpecs, declarations.Maven...)
	deps = append(deps, declarations.Jars...)

	resolved, cleanup, err := ResolveMavenDependencies(mavenSpecs)
	if err != nil {
		return nil, nil, err
	}
	deps = append(deps, resolved...)

	fmt.Println("[INFO] Collected dependencies:")
	for _, d := range deps {
		fmt.Println("   ", d)
	}
	return deps, cleanup, nil
}

// CollectScriptDependencies resolves //DEPS declarations from the script and
// its recursively included sources. Local paths are relative to each declaring
// source; Maven resolution uses the main script directory.
func CollectScriptDependencies(source string) ([]string, func(), error) {
	absSource, err := filepath.Abs(source)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve script path: %w", err)
	}
	sources, err := ResolveScriptSources(absSource)
	if err != nil {
		return nil, nil, err
	}
	declarations, err := CollectJavaDependencyDeclarations(sources)
	if err != nil {
		return nil, nil, err
	}
	deps := append([]string(nil), declarations.Jars...)
	resolved, cleanup, err := ResolveMavenDependenciesInDir(declarations.Maven, filepath.Dir(absSource))
	if err != nil {
		return nil, nil, err
	}
	return append(deps, resolved...), cleanup, nil
}

// BuildJar 构建 JAR 文件
func BuildJar(jarPath, classesDir, mainClass string, staticMode bool, deps []string) error {
	manifest, err := os.CreateTemp("", "MANIFEST-*.mf")
	if err != nil {
		return fmt.Errorf("create manifest: %w", err)
	}
	defer os.Remove(manifest.Name())

	if err := WriteManifest(manifest.Name(), mainClass, deps, staticMode); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	if staticMode {
		return BuildFatJar(jarPath, classesDir, manifest.Name(), deps)
	}

	// 普通 JAR
	cmd := exec.Command("jar", "cfm", jarPath, manifest.Name(), "-C", classesDir, ".")
	outb, errb, err := Run(cmd)
	if err != nil {
		hint := ""
		if runtime.GOOS == "windows" {
			hint = "\n(Hint: ensure a JDK is in PATH so 'jar' is available)"
		}
		return fmt.Errorf("jar failed: %v%s\n%s\n%s", err, hint, outb, errb)
	}
	// 写 META-INF/dependencies.lib
	if len(deps) > 0 {
		tmpDir := filepath.Join(os.TempDir(), "deps-"+filepath.Base(jarPath))
		metaInf := filepath.Join(tmpDir, "META-INF")
		os.MkdirAll(metaInf, 0755)
		depFile := filepath.Join(metaInf, "dependencies.lib")

		f, _ := os.Create(depFile)
		for _, d := range deps {
			fmt.Fprintln(f, "lib/"+filepath.Base(d))
		}
		f.Close()

		cmd = exec.Command("jar", "uf", jarPath, "-C", tmpDir, "META-INF/dependencies.lib")
		Run(cmd)

		os.RemoveAll(tmpDir)
	}
	return nil
}

// WriteManifest 写入 MANIFEST.MF 文件
func WriteManifest(path, mainClass string, deps []string, staticMode bool) error {
	//fmt.Println("Deps: ")
	//fmt.Println(deps)
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Manifest-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Main-Class: %s\r\n", mainClass)
	if !staticMode && len(deps) > 0 {
		cpLine := "Class-Path:"
		for _, d := range deps {
			part := "lib/" + filepath.Base(d)
			// 检查加上空格和part后是否超长
			if len(cpLine)+len(" ")+len(part) > 72 {
				buf.WriteString(cpLine + "\r\n")
				cpLine = " " + part // 换行后空格开头
			} else {
				cpLine = cpLine + " " + part
			}
		}
		buf.WriteString(cpLine + "\r\n")
	}

	buf.WriteString("\r\n") // manifest 结束必须有空行
	//fmt.Printf("[INFO] Generated MANIFEST.MF(%s):\n%s\n", path, buf.String())
	return os.WriteFile(path, buf.Bytes(), 0644)
}

// BuildFatJar 构建 Fat JAR（包含所有依赖）
func BuildFatJar(jarPath, classesDir, manifest string, deps []string) error {
	tmpJar := jarPath + ".tmp"
	exec.Command("jar", "cfm", tmpJar, manifest, "-C", classesDir, ".").Run()

	out, _ := os.Create(jarPath)
	zw := zip.NewWriter(out)

	// 先写主 JAR（保留所有内容）
	AddJarKeepManifest(tmpJar, zw, nil)

	// 收集所有 services 文件的内容
	services := make(map[string][]string)

	// 再写依赖 JAR
	for _, d := range deps {
		AddJarSkipManifest(d, zw, services)
	}

	// 合并写 META-INF/services/*
	for name, lines := range services {
		w, err := zw.Create(name)
		if err != nil {
			continue
		}
		for _, l := range lines {
			fmt.Fprintln(w, l)
		}
	}

	zw.Close()
	out.Close()
	os.Remove(tmpJar)

	// 写 META-INF/dependencies-static.lib
	buf := new(bytes.Buffer)
	for _, d := range deps {
		fmt.Fprintln(buf, "lib/"+filepath.Base(d))
	}
	AddFileToJar(jarPath, "META-INF/dependencies-static.lib", buf.Bytes())

	return nil
}

// AddJarKeepManifest 主 JAR：保留所有文件（包括 MANIFEST.MF）
func AddJarKeepManifest(path string, zw *zip.Writer, services map[string][]string) {
	r, _ := zip.OpenReader(path)
	defer r.Close()
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := f.Name
		// 如果是 services 文件，收集内容
		if services != nil && strings.HasPrefix(strings.ToLower(name), "meta-inf/services/") {
			fmt.Printf("[INFO] merging service file: %s from %s\n", name, filepath.Base(path))
			rc, _ := f.Open()
			scanner := bufio.NewScanner(rc)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" && !strings.HasPrefix(line, "#") {
					services[name] = append(services[name], line)
				}
			}
			rc.Close()
			continue
		}

		// 直接写入
		w, err := zw.Create(name)
		if err != nil {
			continue
		}
		rc, _ := f.Open()
		io.Copy(w, rc)
		rc.Close()
	}
}

// AddJarSkipManifest 依赖 JAR：跳过 MANIFEST.MF，合并 services 文件
func AddJarSkipManifest(path string, zw *zip.Writer, services map[string][]string) {
	r, _ := zip.OpenReader(path)
	defer r.Close()
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := f.Name
		lname := strings.ToLower(name)

		// 跳过依赖的 MANIFEST.MF
		if lname == "meta-inf/manifest.mf" {
			continue
		}

		// 合并 META-INF/services/*
		if strings.HasPrefix(lname, "meta-inf/services/") {
			fmt.Printf("[INFO] merging service file: %s from %s\n", name, filepath.Base(path))
			rc, _ := f.Open()
			scanner := bufio.NewScanner(rc)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" && !strings.HasPrefix(line, "#") {
					services[name] = append(services[name], line)
				}
			}
			rc.Close()
			continue
		}

		// 其他文件 → 覆盖写
		w, err := zw.Create(name)
		if err != nil {
			continue
		}
		rc, _ := f.Open()
		io.Copy(w, rc)
		rc.Close()
	}
}

// AddFileToJar 向 JAR 文件添加文件
func AddFileToJar(jarPath, name string, content []byte) {
	tmp := jarPath + ".tmp2"
	os.Rename(jarPath, tmp)
	out, _ := os.Create(jarPath)
	zw := zip.NewWriter(out)

	r, _ := zip.OpenReader(tmp)
	for _, f := range r.File {
		rc, _ := f.Open()
		w, _ := zw.Create(f.Name)
		io.Copy(w, rc)
		rc.Close()
	}
	r.Close()

	w, _ := zw.Create(name)
	w.Write(content)
	zw.Close()
	out.Close()
	os.Remove(tmp)
}

// CopyExplib 会把依赖复制到输出 JAR 同目录下的 lib/ 目录
func CopyExplib(outDir string, deps []string) {
	libDir := filepath.Join(outDir, "lib")
	os.MkdirAll(libDir, 0755)

	for _, d := range deps {
		target := filepath.Join(libDir, filepath.Base(d))
		data, err := os.ReadFile(d)
		if err != nil {
			fmt.Printf("[WARN] failed to read dependency %s: %v\n", d, err)
			continue
		}
		err = os.WriteFile(target, data, 0644)
		if err != nil {
			fmt.Printf("[WARN] failed to write dependency %s: %v\n", target, err)
		}
	}
	Ok("Exported %d dependencies to %s", len(deps), libDir)
}
