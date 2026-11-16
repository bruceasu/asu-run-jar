package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var compactJDKProfiles = map[string][]string{
	"cli":  {"java.base", "java.logging", "jdk.compiler"},
	"http": {"java.base", "java.logging", "java.net.http", "jdk.compiler", "jdk.crypto.ec"},
	"db": {
		"java.base", "java.logging", "java.management", "java.naming", "java.security.jgss",
		"java.security.sasl", "java.sql", "java.transaction.xa", "java.xml", "jdk.compiler",
		"jdk.crypto.ec", "jdk.unsupported",
	},
	"full": {
		"java.base", "java.logging", "java.management", "java.naming", "java.net.http",
		"java.security.jgss", "java.security.sasl", "java.sql", "java.transaction.xa", "java.xml",
		"jdk.compiler", "jdk.crypto.ec", "jdk.unsupported",
	},
}

// BuildCompactJDK creates a small jlink image with java and javac launchers.
func BuildCompactJDK(profile, outputDir, jdkHome string) error {
	profile = strings.ToLower(strings.TrimSpace(profile))
	modules, ok := compactJDKProfiles[profile]
	if !ok {
		return fmt.Errorf("unknown JDK profile %q; choose cli, http, db, or full", profile)
	}
	jlink, err := findJlink(jdkHome)
	if err != nil {
		return err
	}
	absOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("resolve output directory: %w", err)
	}
	if _, err := os.Stat(absOutput); err == nil {
		return fmt.Errorf("output directory already exists; remove it before rebuilding: %s", absOutput)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect output directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absOutput), 0o755); err != nil {
		return fmt.Errorf("create output parent directory: %w", err)
	}
	args := []string{
		"--add-modules", strings.Join(modules, ","),
		"--launcher", "javac=jdk.compiler/com.sun.tools.javac.Main",
		"--strip-debug", "--compress=2", "--no-header-files", "--no-man-pages",
		"--output", absOutput,
	}
	Info("[JDK] Building %s profile with modules: %s", profile, strings.Join(modules, ","))
	out, errb, err := Run(exec.Command(jlink, args...))
	if err != nil {
		return fmt.Errorf("jlink failed: %v\n%s\n%s", err, out, errb)
	}
	if err := verifyCompactJDK(absOutput); err != nil {
		return err
	}
	Ok("[JDK] Compact JDK created: %s", absOutput)
	return nil
}

func findJlink(jdkHome string) (string, error) {
	name := "jlink"
	if strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") || os.PathSeparator == '\\' {
		name += ".exe"
	}
	if strings.TrimSpace(jdkHome) != "" {
		home := jdkHome
		if strings.EqualFold(filepath.Base(filepath.Clean(home)), "bin") {
			home = filepath.Dir(home)
		}
		path := filepath.Join(home, "bin", name)
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("JDK %q does not provide jlink at %s", jdkHome, path)
		}
		return path, nil
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	if home := os.Getenv("JAVA_HOME"); home != "" {
		path := filepath.Join(home, "bin", name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("jlink not found; use a JDK 9+ and put its bin directory on PATH or pass --jdk")
}

func verifyCompactJDK(home string) error {
	for _, name := range []string{"java", "javac"} {
		if os.PathSeparator == '\\' {
			name += ".exe"
		}
		path := filepath.Join(home, "bin", name)
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("jlink output is missing the %s launcher: %s", name, path)
		}
	}
	return nil
}
