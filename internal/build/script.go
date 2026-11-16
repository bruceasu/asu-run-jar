package build

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ScriptFile maps a source file to its path in the script's runtime directory
// or generated JAR.
type ScriptFile struct {
	Target string
	Source string
}

// ScriptDirectives contains the local source and resource declarations used
// by an ajava script. Maven dependencies are parsed separately so existing
// dep.lib behavior remains unchanged for project builds.
type ScriptDirectives struct {
	JDK            string
	Sources        []string
	Files          []ScriptFile
	CompileOptions []string
	RuntimeOptions []string
}

// ReadScriptDirectives reads //SOURCES, //FILES, //COMPILE_OPTIONS, and
// //RUNTIME_OPTIONS from a source file. Local paths are relative to the file.
func ReadScriptDirectives(source string) (ScriptDirectives, error) {
	absSource, err := filepath.Abs(source)
	if err != nil {
		return ScriptDirectives{}, fmt.Errorf("resolve script path: %w", err)
	}
	file, err := os.Open(absSource)
	if err != nil {
		return ScriptDirectives{}, fmt.Errorf("open Java source %s: %w", source, err)
	}
	defer file.Close()

	var result ScriptDirectives
	scanner := bufio.NewScanner(file)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		name, value, ok := directiveLine(line)
		if !ok {
			continue
		}
		tokens, err := splitDependencyTokens(value)
		if err != nil {
			return ScriptDirectives{}, fmt.Errorf("%s:%d: %w", source, lineNo, err)
		}
		if (name == "SOURCES" || name == "FILES") && len(tokens) == 0 {
			return ScriptDirectives{}, fmt.Errorf("%s:%d: //%s requires at least one path", source, lineNo, name)
		}
		switch name {
		case "SOURCES":
			for _, token := range tokens {
				paths, err := expandSourceReference(filepath.Dir(absSource), token)
				if err != nil {
					return ScriptDirectives{}, fmt.Errorf("%s:%d: %w", source, lineNo, err)
				}
				result.Sources = append(result.Sources, paths...)
			}
		case "FILES":
			for _, token := range tokens {
				files, err := expandScriptFile(filepath.Dir(absSource), token)
				if err != nil {
					return ScriptDirectives{}, fmt.Errorf("%s:%d: %w", source, lineNo, err)
				}
				result.Files = append(result.Files, files...)
			}
		case "COMPILE_OPTIONS":
			result.CompileOptions = append(result.CompileOptions, tokens...)
		case "RUNTIME_OPTIONS":
			result.RuntimeOptions = append(result.RuntimeOptions, tokens...)
		case "JDK":
			if len(tokens) != 1 {
				return ScriptDirectives{}, fmt.Errorf("%s:%d: //JDK requires one version or JDK home path", source, lineNo)
			}
			if result.JDK != "" && result.JDK != tokens[0] {
				return ScriptDirectives{}, fmt.Errorf("%s:%d: conflicting //JDK directives", source, lineNo)
			}
			result.JDK = tokens[0]
		}
	}
	if err := scanner.Err(); err != nil {
		return ScriptDirectives{}, fmt.Errorf("read Java source %s: %w", source, err)
	}
	result.Sources = uniquePaths(result.Sources)
	return result, nil
}

func directiveLine(line string) (name, value string, ok bool) {
	if !strings.HasPrefix(line, "//") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "//")
	name, value, _ = strings.Cut(line, " ")
	if value == "" {
		name, value, _ = strings.Cut(line, "\t")
	}
	if name == line {
		value = ""
	}
	return strings.ToUpper(strings.TrimSpace(name)), strings.TrimSpace(value), true
}

func expandSourceReference(baseDir, reference string) ([]string, error) {
	pattern := reference
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(baseDir, pattern)
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid source pattern %q: %w", reference, err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("source not found: %s", reference)
	}
	var sources []string
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			return nil, fmt.Errorf("inspect source %s: %w", match, err)
		}
		if info.IsDir() {
			err = filepath.WalkDir(match, func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".java") {
					sources = append(sources, path)
				}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("scan source directory %s: %w", match, err)
			}
			continue
		}
		if !strings.EqualFold(filepath.Ext(match), ".java") {
			return nil, fmt.Errorf("source is not a .java file or directory: %s", match)
		}
		sources = append(sources, match)
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("source reference contains no .java files: %s", reference)
	}
	return uniquePaths(sources), nil
}

func expandScriptFile(baseDir, token string) ([]ScriptFile, error) {
	target, source := token, token
	mapped := false
	if left, right, ok := strings.Cut(token, "="); ok {
		target, source = left, right
		mapped = true
		if target == "" || source == "" {
			return nil, fmt.Errorf("invalid //FILES mapping %q; use target=source", token)
		}
	}
	if !filepath.IsAbs(source) {
		source = filepath.Join(baseDir, source)
	}
	matches, err := filepath.Glob(source)
	if err != nil {
		return nil, fmt.Errorf("invalid file pattern %q: %w", token, err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("file not found: %s", token)
	}

	var result []ScriptFile
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			return nil, fmt.Errorf("inspect file %s: %w", match, err)
		}
		matchTarget := target
		if !mapped {
			matchTarget, err = filepath.Rel(baseDir, match)
			if err != nil || strings.HasPrefix(matchTarget, ".."+string(filepath.Separator)) {
				matchTarget = filepath.Base(match)
			}
			matchTarget = filepath.ToSlash(filepath.Clean(matchTarget))
		}
		if info.IsDir() {
			err = filepath.WalkDir(match, func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() {
					return nil
				}
				rel, err := filepath.Rel(match, path)
				if err != nil {
					return err
				}
				dest := filepath.ToSlash(filepath.Join(matchTarget, rel))
				if err := validateScriptTarget(dest); err != nil {
					return err
				}
				result = append(result, ScriptFile{Target: dest, Source: path})
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("scan resource directory %s: %w", match, err)
			}
			continue
		}
		if err := validateScriptTarget(matchTarget); err != nil {
			return nil, err
		}
		result = append(result, ScriptFile{Target: filepath.ToSlash(filepath.Clean(matchTarget)), Source: match})
	}
	return result, nil
}

func validateScriptTarget(target string) error {
	clean := filepath.Clean(filepath.FromSlash(target))
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("//FILES target must stay inside the output: %s", target)
	}
	return nil
}

// ResolveScriptSources follows local //SOURCES directives recursively while
// preventing cycles. The main source is always first in the returned list.
func ResolveScriptSources(mainSource string) ([]string, error) {
	mainAbs, err := filepath.Abs(mainSource)
	if err != nil {
		return nil, fmt.Errorf("resolve main source: %w", err)
	}
	queue := []string{mainAbs}
	seen := map[string]bool{}
	var result []string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		key := filepath.Clean(current)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, current)
		directives, err := ReadScriptDirectives(current)
		if err != nil {
			return nil, err
		}
		queue = append(queue, directives.Sources...)
	}
	return result, nil
}

// CopyScriptFiles stages declared resources under destination. Target paths
// are validated during parsing and checked again before writing.
func CopyScriptFiles(destination string, files []ScriptFile) error {
	seen := map[string]string{}
	for _, file := range files {
		if err := validateScriptTarget(file.Target); err != nil {
			return err
		}
		target := filepath.Join(destination, filepath.FromSlash(file.Target))
		key := strings.ToLower(filepath.Clean(target))
		if previous, ok := seen[key]; ok {
			if filepath.Clean(previous) == filepath.Clean(file.Source) {
				continue
			}
			return fmt.Errorf("duplicate //FILES target: %s", file.Target)
		}
		seen[key] = file.Source
		data, err := os.ReadFile(file.Source)
		if err != nil {
			return fmt.Errorf("read resource %s: %w", file.Source, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create resource directory: %w", err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("write resource %s: %w", target, err)
		}
	}
	return nil
}

// StageScriptFiles places declared resources in a script's execution
// directory. Existing files are preserved unless they are the same source.
func StageScriptFiles(destination string, files []ScriptFile) error {
	root, err := filepath.Abs(destination)
	if err != nil {
		return fmt.Errorf("resolve script execution directory: %w", err)
	}
	for _, file := range files {
		if err := validateScriptTarget(file.Target); err != nil {
			return err
		}
		target := filepath.Join(root, filepath.FromSlash(file.Target))
		if SamePath(target, file.Source) {
			continue
		}
		if _, err := os.Stat(target); err == nil {
			return fmt.Errorf("//FILES target already exists; refusing to overwrite: %s", target)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect //FILES target %s: %w", target, err)
		}
		data, err := os.ReadFile(file.Source)
		if err != nil {
			return fmt.Errorf("read resource %s: %w", file.Source, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create resource directory: %w", err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("stage resource %s: %w", target, err)
		}
	}
	return nil
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		abs, err := filepath.Abs(path)
		if err == nil {
			path = abs
		}
		key := filepath.Clean(path)
		if !seen[key] {
			seen[key] = true
			result = append(result, key)
		}
	}
	sort.Strings(result)
	return result
}
