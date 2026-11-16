package build

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// BuildRuntimeAppImage creates a jlink runtime and packages app-image with jpackage.
func BuildRuntimeAppImage(
	jarPath, outDir, appName, mainClass, runtimeImageDir, distDir, javaOpts, addModules string,
	winConsole bool,
) error {
	jarAbs, err := filepath.Abs(jarPath)
	if err != nil {
		return fmt.Errorf("resolve jar path: %w", err)
	}
	if _, err := os.Stat(jarAbs); err != nil {
		return fmt.Errorf("jar not found: %s", jarAbs)
	}

	jarDir := filepath.Dir(jarAbs)
	jarName := filepath.Base(jarAbs)

	if strings.TrimSpace(appName) == "" {
		appName = strings.TrimSuffix(jarName, filepath.Ext(jarName))
	}
	if strings.TrimSpace(runtimeImageDir) == "" {
		runtimeImageDir = filepath.Join(outDir, "runtime")
	}
	if strings.TrimSpace(distDir) == "" {
		distDir = filepath.Join(outDir, "dist")
	}
	if strings.TrimSpace(mainClass) == "" {
		mainClass = detectMainClassFromManifest(jarAbs)
	}

	if err := mustHave("jdeps", "jlink", "jpackage"); err != nil {
		return err
	}

	modules, err := detectModules(jarAbs, jarDir)
	if err != nil {
		return err
	}
	modules = mergeModuleList(modules, addModules)
	Info("[RUNTIME] Modules for jlink: %s", modules)

	if err := os.RemoveAll(runtimeImageDir); err != nil {
		return fmt.Errorf("clean runtime dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(runtimeImageDir), 0o755); err != nil {
		return fmt.Errorf("create runtime parent dir: %w", err)
	}
	if err := os.MkdirAll(distDir, 0o755); err != nil {
		return fmt.Errorf("create dist dir: %w", err)
	}

	jlinkArgs := []string{
		"--add-modules", modules,
		"--strip-debug",
		"--compress=2",
		"--no-header-files",
		"--no-man-pages",
		"--output", runtimeImageDir,
	}
	Info("[RUNTIME] Running jlink...")
	if out, errb, err := Run(exec.Command("jlink", jlinkArgs...)); err != nil {
		return fmt.Errorf("jlink failed: %v\n%s\n%s", err, out, errb)
	}

	jpackageArgs := []string{
		"--type", "app-image",
		"--name", appName,
		"--input", jarDir,
		"--main-jar", jarName,
		"--runtime-image", runtimeImageDir,
		"--dest", distDir,
	}
	if strings.TrimSpace(mainClass) != "" {
		jpackageArgs = append(jpackageArgs, "--main-class", mainClass)
	}
	for _, opt := range splitJavaOptions(javaOpts) {
		jpackageArgs = append(jpackageArgs, "--java-options", opt)
	}
	if runtime.GOOS == "windows" && winConsole {
		jpackageArgs = append(jpackageArgs, "--win-console")
	}

	Info("[RUNTIME] Running jpackage...")
	if out, errb, err := Run(exec.Command("jpackage", jpackageArgs...)); err != nil {
		return fmt.Errorf("jpackage failed: %v\n%s\n%s", err, out, errb)
	}

	Ok("[RUNTIME] App image created: %s", filepath.Join(distDir, appName))
	return nil
}

func mustHave(cmds ...string) error {
	for _, c := range cmds {
		if _, err := exec.LookPath(c); err != nil {
			return fmt.Errorf("%s command not found in PATH", c)
		}
	}
	return nil
}

func detectModules(jarPath, jarDir string) (string, error) {
	args := []string{
		"--ignore-missing-deps",
		"--multi-release", "25",
		"--recursive",
		"--print-module-deps",
	}

	libs, analysisJar, cleanup, err := collectRuntimeDependencyJars(jarPath, jarDir)
	if err != nil {
		return "", err
	}
	if cleanup != nil {
		defer cleanup()
	}
	if len(libs) > 0 {
		args = append(args, "--class-path", strings.Join(libs, string(os.PathListSeparator)))
	}
	args = append(args, analysisJar)

	out, errb, err := Run(exec.Command("jdeps", args...))
	if err != nil {
		return "", fmt.Errorf("jdeps failed: %v\n%s\n%s", err, out, errb)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line, nil
		}
	}
	return "", fmt.Errorf("jdeps returned empty module list")
}

func collectRuntimeDependencyJars(jarPath, jarDir string) ([]string, string, func(), error) {
	var jars []string
	libDir := filepath.Join(jarDir, "lib")
	err := filepath.WalkDir(libDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if os.IsNotExist(walkErr) {
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".jar") {
			jars = append(jars, path)
		}
		return nil
	})
	if err != nil {
		return nil, "", nil, fmt.Errorf("scan external runtime dependencies: %w", err)
	}

	archive, err := zip.OpenReader(jarPath)
	if err != nil {
		return nil, "", nil, fmt.Errorf("open application JAR for module analysis: %w", err)
	}
	var nested []*zip.File
	var appClasses []*zip.File
	for _, file := range archive.File {
		name := strings.ToUpper(filepath.ToSlash(file.Name))
		if strings.HasSuffix(name, ".JAR") && (strings.HasPrefix(name, "BOOT-INF/LIB/") || strings.HasPrefix(name, "WEB-INF/LIB/") || strings.HasPrefix(name, "LIB/")) {
			nested = append(nested, file)
		}
		if strings.HasSuffix(name, ".CLASS") && (strings.HasPrefix(name, "BOOT-INF/CLASSES/") || strings.HasPrefix(name, "WEB-INF/CLASSES/")) {
			appClasses = append(appClasses, file)
		}
	}
	if len(nested) == 0 && len(appClasses) == 0 {
		if err := archive.Close(); err != nil {
			return nil, "", nil, fmt.Errorf("close application JAR: %w", err)
		}
		return jars, jarPath, nil, nil
	}
	tempDir, err := os.MkdirTemp("", "asu-jdeps-deps-")
	if err != nil {
		archive.Close()
		return nil, "", nil, fmt.Errorf("create nested dependency workspace: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	for i, file := range nested {
		rc, err := file.Open()
		if err != nil {
			archive.Close()
			cleanup()
			return nil, "", nil, fmt.Errorf("open nested dependency %s: %w", file.Name, err)
		}
		name := fmt.Sprintf("%04d-%s", i, filepath.Base(file.Name))
		path := filepath.Join(tempDir, name)
		out, err := os.Create(path)
		if err == nil {
			_, err = io.Copy(out, rc)
			if closeErr := out.Close(); err == nil {
				err = closeErr
			}
		}
		rc.Close()
		if err != nil {
			archive.Close()
			cleanup()
			return nil, "", nil, fmt.Errorf("extract nested dependency %s: %w", file.Name, err)
		}
		jars = append(jars, path)
	}
	analysisJar := jarPath
	if len(appClasses) > 0 {
		appJarPath := filepath.Join(tempDir, "application-classes.jar")
		appJar, err := os.Create(appJarPath)
		if err != nil {
			archive.Close()
			cleanup()
			return nil, "", nil, fmt.Errorf("create extracted application classes JAR: %w", err)
		}
		zipWriter := zip.NewWriter(appJar)
		for _, file := range appClasses {
			entryName := filepath.ToSlash(file.Name)
			for _, prefix := range []string{"BOOT-INF/classes/", "WEB-INF/classes/"} {
				if strings.HasPrefix(strings.ToUpper(entryName), strings.ToUpper(prefix)) {
					entryName = entryName[len(prefix):]
					break
				}
			}
			rc, err := file.Open()
			if err != nil {
				zipWriter.Close()
				appJar.Close()
				archive.Close()
				cleanup()
				return nil, "", nil, fmt.Errorf("open application class %s: %w", file.Name, err)
			}
			writer, err := zipWriter.Create(entryName)
			if err == nil {
				_, err = io.Copy(writer, rc)
			}
			rc.Close()
			if err != nil {
				zipWriter.Close()
				appJar.Close()
				archive.Close()
				cleanup()
				return nil, "", nil, fmt.Errorf("copy application class %s: %w", file.Name, err)
			}
		}
		if err := zipWriter.Close(); err != nil {
			appJar.Close()
			archive.Close()
			cleanup()
			return nil, "", nil, fmt.Errorf("finish extracted application classes JAR: %w", err)
		}
		if err := appJar.Close(); err != nil {
			archive.Close()
			cleanup()
			return nil, "", nil, fmt.Errorf("close extracted application classes JAR: %w", err)
		}
		analysisJar = appJarPath
	}
	if err := archive.Close(); err != nil {
		cleanup()
		return nil, "", nil, fmt.Errorf("close application JAR: %w", err)
	}
	return jars, analysisJar, cleanup, nil
}

func splitJavaOptions(v string) []string {
	parts := strings.FieldsFunc(v, func(r rune) bool {
		return r == ';' || r == '\n'
	})
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func mergeModuleList(detected, extra string) string {
	set := map[string]struct{}{}
	add := func(list string) {
		for _, it := range strings.Split(list, ",") {
			it = strings.TrimSpace(it)
			if it != "" {
				set[it] = struct{}{}
			}
		}
	}
	add(detected)
	add(extra)

	mods := make([]string, 0, len(set))
	for m := range set {
		mods = append(mods, m)
	}
	sort.Strings(mods)
	return strings.Join(mods, ",")
}

func detectMainClassFromManifest(jarPath string) string {
	r, err := zip.OpenReader(jarPath)
	if err != nil {
		return ""
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.EqualFold(f.Name, "META-INF/MANIFEST.MF") {
			rc, err := f.Open()
			if err != nil {
				return ""
			}
			defer rc.Close()

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(rc)
			var lastKey string
			var mainClass string
			for _, raw := range strings.Split(buf.String(), "\n") {
				line := strings.TrimRight(raw, "\r")
				if strings.HasPrefix(line, " ") && lastKey == "Main-Class" {
					mainClass += strings.TrimSpace(line)
					continue
				}
				parts := strings.SplitN(line, ":", 2)
				if len(parts) != 2 {
					lastKey = ""
					continue
				}
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				lastKey = key
				if strings.EqualFold(key, "Main-Class") {
					mainClass = val
				}
			}
			return mainClass
		}
	}
	return ""
}
