package build

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
)

// MavenDependency is the subset of JBang's Maven coordinate syntax needed by
// source directives and dep.lib.
type MavenDependency struct {
	GroupID    string
	ArtifactID string
	Version    string
	Classifier string
	Type       string
}

type JavaDependencyDeclarations struct {
	Maven []MavenDependency
	Jars  []string
}

func (d MavenDependency) IsBOM() bool { return d.Type == "pom" }

// ParseMavenDependency accepts group:artifact[:version[:classifier]][@type].
// A missing version is resolved through an imported BOM.
func ParseMavenDependency(value string) (MavenDependency, bool) {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		return MavenDependency{}, false
	}
	if len(value) >= 3 && value[1] == ':' && (value[2] == '/' || value[2] == '\\') {
		return MavenDependency{}, false
	}

	parts := strings.Split(value, "@")
	if len(parts) > 2 {
		return MavenDependency{}, false
	}
	typeName := "jar"
	if len(parts) == 2 {
		typeName = strings.TrimSpace(parts[1])
		if typeName == "" {
			return MavenDependency{}, false
		}
	}
	fields := strings.Split(parts[0], ":")
	if len(fields) < 2 || len(fields) > 4 {
		return MavenDependency{}, false
	}
	for i := range fields {
		fields[i] = strings.TrimSpace(fields[i])
	}
	if fields[0] == "" || fields[1] == "" {
		return MavenDependency{}, false
	}
	d := MavenDependency{GroupID: fields[0], ArtifactID: fields[1], Type: typeName}
	if len(fields) >= 3 {
		d.Version = fields[2]
	}
	if len(fields) == 4 {
		d.Classifier = fields[3]
	}
	if d.IsBOM() && (d.Version == "" || d.Classifier != "") {
		return MavenDependency{}, false
	}
	return d, true
}

// ReadJavaDependencyDirectives reads //DEPS declarations from one Java source
// file, resolving local paths from the current working directory.
func ReadJavaDependencyDirectives(source string) (JavaDependencyDeclarations, error) {
	return ReadJavaDependencyDirectivesAt(source, ".")
}

// ReadJavaDependencyDirectivesAt resolves local dependency paths relative to
// baseDir while reading Maven coordinates from a source file.
func ReadJavaDependencyDirectivesAt(source, baseDir string) (JavaDependencyDeclarations, error) {
	file, err := os.Open(source)
	if err != nil {
		return JavaDependencyDeclarations{}, fmt.Errorf("open Java source %s: %w", source, err)
	}
	defer file.Close()

	var declarations JavaDependencyDeclarations
	scanner := bufio.NewScanner(file)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line != "//DEPS" && !strings.HasPrefix(line, "//DEPS ") && !strings.HasPrefix(line, "//DEPS\t") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "//DEPS"))
		if value == "" {
			return JavaDependencyDeclarations{}, fmt.Errorf("%s:%d: //DEPS requires a dependency", source, lineNo)
		}
		tokens, err := splitDependencyTokens(value)
		if err != nil {
			return JavaDependencyDeclarations{}, fmt.Errorf("%s:%d: %w", source, lineNo, err)
		}
		for _, token := range tokens {
			if spec, ok := ParseMavenDependency(token); ok {
				declarations.Maven = append(declarations.Maven, spec)
				continue
			}
			jars, recognized, err := ExpandLocalJarReferenceAt(baseDir, token)
			if err != nil {
				return JavaDependencyDeclarations{}, fmt.Errorf("%s:%d: %w", source, lineNo, err)
			}
			if !recognized {
				return JavaDependencyDeclarations{}, fmt.Errorf("%s:%d: unsupported dependency %q; use a Maven coordinate, JAR path, or directory", source, lineNo, token)
			}
			declarations.Jars = append(declarations.Jars, jars...)
		}
	}
	if err := scanner.Err(); err != nil {
		return JavaDependencyDeclarations{}, fmt.Errorf("read Java source %s: %w", source, err)
	}
	return declarations, nil
}

// CollectJavaDependencyDeclarations reads //DEPS from each source in order.
// Local paths are resolved relative to the source file that declares them.
func CollectJavaDependencyDeclarations(sources []string) (JavaDependencyDeclarations, error) {
	var result JavaDependencyDeclarations
	seenSources := make(map[string]bool, len(sources))
	seenJars := make(map[string]bool)
	for _, source := range sources {
		absSource, err := filepath.Abs(source)
		if err != nil {
			return JavaDependencyDeclarations{}, fmt.Errorf("resolve Java source %s: %w", source, err)
		}
		key := filepath.Clean(absSource)
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if seenSources[key] {
			continue
		}
		seenSources[key] = true

		declarations, err := ReadJavaDependencyDirectivesAt(absSource, filepath.Dir(absSource))
		if err != nil {
			return JavaDependencyDeclarations{}, err
		}
		result.Maven = append(result.Maven, declarations.Maven...)
		for _, jar := range declarations.Jars {
			key := filepath.Clean(jar)
			if runtime.GOOS == "windows" {
				key = strings.ToLower(key)
			}
			if !seenJars[key] {
				seenJars[key] = true
				result.Jars = append(result.Jars, jar)
			}
		}
	}
	return result, nil
}

func splitDependencyTokens(value string) ([]string, error) {
	var tokens []string
	var token strings.Builder
	var quote rune
	flush := func() {
		if token.Len() > 0 {
			tokens = append(tokens, token.String())
			token.Reset()
		}
	}
	for _, char := range value {
		if quote != 0 {
			if char == quote {
				quote = 0
			} else {
				token.WriteRune(char)
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}
		if unicode.IsSpace(char) {
			flush()
			continue
		}
		token.WriteRune(char)
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in //DEPS")
	}
	flush()
	return tokens, nil
}

// ExpandLocalJarReference expands a local JAR file, a directory containing
// JARs, or a filesystem glob. Relative paths are based on the project working
// directory, matching dep.lib and the lib/ directory convention.
func ExpandLocalJarReference(reference string) ([]string, bool, error) {
	return ExpandLocalJarReferenceAt(".", reference)
}

// ExpandLocalJarReferenceAt expands a local reference relative to baseDir.
func ExpandLocalJarReferenceAt(baseDir, reference string) ([]string, bool, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return nil, false, nil
	}
	if strings.HasPrefix(reference, "~/") || strings.HasPrefix(reference, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, true, fmt.Errorf("resolve home directory: %w", err)
		}
		reference = filepath.Join(home, reference[2:])
	}
	pattern := reference
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(baseDir, pattern)
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, true, fmt.Errorf("invalid JAR path pattern %q: %w", reference, err)
	}
	pathLike := filepath.IsAbs(reference) || strings.ContainsAny(reference, `/\\`) || strings.HasPrefix(reference, ".") || strings.HasSuffix(strings.ToLower(reference), ".jar")
	if len(matches) == 0 {
		if pathLike {
			return nil, true, fmt.Errorf("local dependency path not found: %s", reference)
		}
		return nil, false, nil
	}

	var jars []string
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			return nil, true, fmt.Errorf("inspect local dependency %s: %w", match, err)
		}
		if !info.IsDir() {
			if !strings.EqualFold(filepath.Ext(match), ".jar") {
				return nil, true, fmt.Errorf("local dependency is not a JAR or directory: %s", match)
			}
			absolute, err := filepath.Abs(match)
			if err != nil {
				return nil, true, fmt.Errorf("resolve local dependency %s: %w", match, err)
			}
			jars = append(jars, absolute)
			continue
		}
		err = filepath.WalkDir(match, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".jar") {
				absolute, absErr := filepath.Abs(path)
				if absErr != nil {
					return absErr
				}
				jars = append(jars, absolute)
			}
			return nil
		})
		if err != nil {
			return nil, true, fmt.Errorf("scan dependency directory %s: %w", match, err)
		}
	}
	if len(jars) == 0 {
		return nil, true, fmt.Errorf("dependency path contains no JAR files: %s", reference)
	}
	return jars, true, nil
}

// ResolveMavenDependencies resolves direct and transitive runtime dependencies
// with Maven. The returned cleanup function removes the temporary resolution
// directory after compilation and packaging have finished.
func ResolveMavenDependencies(specs []MavenDependency) ([]string, func(), error) {
	return ResolveMavenDependenciesInDir(specs, ".")
}

// ResolveMavenDependenciesInDir resolves dependencies using Maven from the
// supplied project directory, which allows a script to be run from elsewhere.
func ResolveMavenDependenciesInDir(specs []MavenDependency, projectDir string) ([]string, func(), error) {
	if err := validateMavenDependencyVersions(specs); err != nil {
		return nil, nil, err
	}
	specs = uniqueMavenDependencies(specs)
	if len(specs) == 0 {
		return nil, nil, nil
	}

	projectDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve Maven project directory: %w", err)
	}
	mavenCmd, available := findAvailableMavenCommandIn(projectDir)
	if !available {
		return resolveDirectMavenDependencies(specs)
	}

	root, err := os.MkdirTemp("", "ajava-deps-")
	if err != nil {
		return nil, nil, fmt.Errorf("create Maven dependency workspace: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	outputDir := filepath.Join(root, "lib")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("create Maven dependency output: %w", err)
	}
	pomPath := filepath.Join(root, "pom.xml")
	if err := os.WriteFile(pomPath, []byte(mavenDependencyPOM(specs)), 0o644); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("write temporary Maven POM: %w", err)
	}

	cmd := exec.Command(mavenCmd,
		"-B", "-q", "-f", pomPath,
		"dependency:copy-dependencies",
		"-DincludeScope=runtime",
		"-DoutputDirectory="+outputDir,
	)
	cmd.Dir = projectDir
	out, errb, err := Run(cmd)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("Maven dependency resolution failed: %v\n%s\n%s", err, out, errb)
	}

	var jars []string
	err = filepath.WalkDir(outputDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".jar") {
			jars = append(jars, path)
		}
		return nil
	})
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("collect resolved Maven dependencies: %w", err)
	}
	if len(jars) == 0 {
		cleanup()
		return nil, nil, fmt.Errorf("Maven resolved no JAR dependencies; check the declared coordinates")
	}
	Info("[MAVEN] Resolved %d runtime JAR(s), including transitive dependencies", len(jars))
	return jars, cleanup, nil
}

func validateMavenDependencyVersions(specs []MavenDependency) error {
	type artifactVersion struct {
		version string
	}
	versions := make(map[string]artifactVersion, len(specs))
	for _, spec := range specs {
		if spec.Version == "" || spec.IsBOM() {
			continue
		}
		key := spec.GroupID + ":" + spec.ArtifactID
		if previous, ok := versions[key]; ok && previous.version != spec.Version {
			return fmt.Errorf("conflicting //DEPS versions for %s: %s and %s", key, previous.version, spec.Version)
		}
		versions[key] = artifactVersion{version: spec.Version}
	}
	return nil
}

func findAvailableMavenCommand() (string, bool) {
	command := FindMavenCommand()
	if _, err := exec.LookPath(command); err == nil {
		return command, true
	}
	if info, err := os.Stat(command); err == nil && !info.IsDir() {
		absolute, absErr := filepath.Abs(command)
		if absErr == nil {
			return absolute, true
		}
		return command, true
	}
	return command, false
}

func findAvailableMavenCommandIn(projectDir string) (string, bool) {
	name := "mvnw"
	if runtime.GOOS == "windows" {
		name = "mvnw.cmd"
	}
	wrapper := filepath.Join(projectDir, name)
	if _, err := os.Stat(wrapper); err == nil {
		return wrapper, true
	}
	command := "mvn"
	if _, err := exec.LookPath(command); err == nil {
		return command, true
	}
	return command, false
}

func resolveDirectMavenDependencies(specs []MavenDependency) ([]string, func(), error) {
	var jars []string
	for _, spec := range specs {
		if spec.IsBOM() || spec.Version == "" || spec.Classifier != "" || spec.Type != "jar" {
			return nil, nil, fmt.Errorf("Maven is required for BOMs, versionless coordinates, classifiers, and transitive dependencies; install Maven or use a Maven project")
		}
		coordinate := strings.Join([]string{spec.GroupID, spec.ArtifactID, spec.Version}, ":")
		jar, err := ResolveMavenDependency(coordinate)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve %s: %w", coordinate, err)
		}
		jars = append(jars, jar)
	}
	fmt.Println("[WARN] Maven was not found; only explicitly listed JARs were resolved (transitive dependencies are unavailable)")
	return jars, nil, nil
}

func uniqueMavenDependencies(specs []MavenDependency) []MavenDependency {
	seen := make(map[string]bool, len(specs))
	unique := make([]MavenDependency, 0, len(specs))
	for _, spec := range specs {
		key := strings.Join([]string{spec.GroupID, spec.ArtifactID, spec.Version, spec.Classifier, spec.Type}, ":")
		if !seen[key] {
			seen[key] = true
			unique = append(unique, spec)
		}
	}
	return unique
}

func mavenDependencyPOM(specs []MavenDependency) string {
	var b strings.Builder
	b.WriteString("<project xmlns=\"http://maven.apache.org/POM/4.0.0\" xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\" xsi:schemaLocation=\"http://maven.apache.org/POM/4.0.0 https://maven.apache.org/xsd/maven-4.0.0.xsd\">\n")
	b.WriteString("<modelVersion>4.0.0</modelVersion><groupId>local.asu</groupId><artifactId>ajava-dependencies</artifactId><version>1.0</version>\n")
	var boms, dependencies []MavenDependency
	for _, spec := range specs {
		if spec.IsBOM() {
			boms = append(boms, spec)
		} else {
			dependencies = append(dependencies, spec)
		}
	}
	if len(boms) > 0 {
		b.WriteString("<dependencyManagement><dependencies>")
		for _, spec := range boms {
			writeDependencyXML(&b, spec, true)
		}
		b.WriteString("</dependencies></dependencyManagement>\n")
	}
	if len(dependencies) > 0 {
		b.WriteString("<dependencies>")
		for _, spec := range dependencies {
			writeDependencyXML(&b, spec, false)
		}
		b.WriteString("</dependencies>\n")
	}
	b.WriteString("</project>\n")
	return b.String()
}

func writeDependencyXML(b *strings.Builder, spec MavenDependency, bom bool) {
	b.WriteString("<dependency><groupId>")
	writeXMLEscaped(b, spec.GroupID)
	b.WriteString("</groupId><artifactId>")
	writeXMLEscaped(b, spec.ArtifactID)
	b.WriteString("</artifactId>")
	if spec.Version != "" {
		b.WriteString("<version>")
		writeXMLEscaped(b, spec.Version)
		b.WriteString("</version>")
	}
	if spec.Classifier != "" {
		b.WriteString("<classifier>")
		writeXMLEscaped(b, spec.Classifier)
		b.WriteString("</classifier>")
	}
	if bom {
		b.WriteString("<type>pom</type><scope>import</scope>")
	} else if spec.Type != "" && spec.Type != "jar" {
		b.WriteString("<type>")
		writeXMLEscaped(b, spec.Type)
		b.WriteString("</type>")
	}
	b.WriteString("</dependency>")
}

func writeXMLEscaped(b *strings.Builder, value string) {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(value))
	b.WriteString(escaped.String())
}
