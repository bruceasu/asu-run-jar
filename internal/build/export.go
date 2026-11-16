package build

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ExportDependencies resolves project dependencies and copies them to outputDir.
func ExportDependencies(sourceDir, outputDir string) error {
	absOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("resolve dependency output directory: %w", err)
	}
	if DetectMavenProject() {
		return MavenCopyDependenciesTo(absOutput)
	}

	sources, err := collectProjectSources(sourceDir)
	if err != nil {
		return err
	}
	dependencies, cleanup, err := CollectDeps(sources)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}
	if len(dependencies) == 0 {
		return fmt.Errorf("no dependencies found to export")
	}
	if err := os.MkdirAll(absOutput, 0o755); err != nil {
		return fmt.Errorf("create dependency output directory: %w", err)
	}
	written := make(map[string][]byte, len(dependencies))
	for _, dependency := range dependencies {
		data, err := os.ReadFile(dependency)
		if err != nil {
			return fmt.Errorf("read dependency %s: %w", dependency, err)
		}
		name := filepath.Base(dependency)
		if prior, ok := written[name]; ok {
			if !bytes.Equal(prior, data) {
				return fmt.Errorf("dependencies with different contents share output name %q", name)
			}
			continue
		}
		written[name] = data
		if err := os.WriteFile(filepath.Join(absOutput, name), data, 0o644); err != nil {
			return fmt.Errorf("write dependency %s: %w", name, err)
		}
	}
	Ok("Exported %d dependencies to %s", len(written), absOutput)
	return nil
}

func collectProjectSources(sourceDir string) ([]string, error) {
	sources, err := CollectJavaFiles(sourceDir)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	for index := 0; index < len(sources); index++ {
		key := filepath.Clean(sources[index])
		if seen[key] {
			continue
		}
		seen[key] = true
		directives, err := ReadScriptDirectives(sources[index])
		if err != nil {
			return nil, err
		}
		sources = append(sources, directives.Sources...)
	}
	return uniquePaths(sources), nil
}

// MavenCopyDependenciesTo resolves Maven's runtime dependency graph into dir.
func MavenCopyDependenciesTo(dir string) error {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve Maven dependency output directory: %w", err)
	}
	if err := os.MkdirAll(absolute, 0o755); err != nil {
		return fmt.Errorf("create Maven dependency output directory: %w", err)
	}
	mvnCmd := FindMavenCommand()
	Info("[MAVEN] Copying dependencies to %s", absolute)
	cmd := exec.Command(mvnCmd,
		"dependency:copy-dependencies",
		"-DoutputDirectory="+absolute,
		"-DincludeScope=runtime")
	out, errb, err := Run(cmd)
	if err != nil {
		return fmt.Errorf("Maven copy dependencies failed: %v\n%s\n%s", err, out, errb)
	}
	var count int
	if err := filepath.WalkDir(absolute, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".jar") {
			count++
		}
		return nil
	}); err != nil {
		return fmt.Errorf("count exported Maven dependencies: %w", err)
	}
	Ok("Exported %d Maven dependencies to %s", count, absolute)
	return nil
}
