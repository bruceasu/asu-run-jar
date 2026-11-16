package ide

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bruceasu/asu-run-jar/internal/build"
)

func GenerateEclipse(folder string) error {
	// If folder is "." or current directory, use current directory
	baseDir := folder
	if folder == "." {
		var err error
		baseDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	} else {
		abs, err := filepath.Abs(folder)
		if err != nil {
			return fmt.Errorf("failed to resolve path %s: %w", folder, err)
		}
		baseDir = abs
	}

	// Create directories
	dirs := []string{filepath.Join(baseDir, "src"), filepath.Join(baseDir, "lib"), filepath.Join(baseDir, "classes"), filepath.Join(baseDir, "bin")}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create .project
	projectPath := filepath.Join(baseDir, ".project")
	projectContent := `<?xml version="1.0" encoding="UTF-8"?>
<projectDescription>
	<name>JavaProject</name>
	<comment></comment>
	<projects>
	</projects>
	<buildSpec>
		<buildCommand>
			<name>org.eclipse.jdt.core.javabuilder</name>
			<arguments>
			</arguments>
		</buildCommand>
	</buildSpec>
	<natures>
		<nature>org.eclipse.jdt.core.javanature</nature>
	</natures>
</projectDescription>`
	if err := os.WriteFile(projectPath, []byte(projectContent), 0644); err != nil {
		return fmt.Errorf("failed to write .project: %w", err)
	}

	// Create .classpath
	classpathPath := filepath.Join(baseDir, ".classpath")
	var classpath strings.Builder
	classpath.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<classpath>
	<classpathentry kind="src" path="src"/>
	<classpathentry kind="con" path="org.eclipse.jdt.launching.JRE_CONTAINER"/>`)
	err := filepath.WalkDir(filepath.Join(baseDir, "lib"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".jar") {
			return nil
		}
		relative, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		fmt.Fprintf(&classpath, "\n\t<classpathentry kind=\"lib\" path=\"%s\"/>", escapeXML(filepath.ToSlash(relative)))
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to scan library directory: %w", err)
	}
	classpath.WriteString(`
	<classpathentry kind="output" path="classes"/>
</classpath>`)
	classpathContent := classpath.String()
	if err := os.WriteFile(classpathPath, []byte(classpathContent), 0644); err != nil {
		return fmt.Errorf("failed to write .classpath: %w", err)
	}

	build.Info("Generated Eclipse Java project settings in %s", baseDir)

	if err := build.EnsureGitSetup(baseDir); err != nil {
		build.Info("[GIT] %v", err)
	}
	return nil
}
