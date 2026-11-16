package ide

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bruceasu/asu-run-jar/internal/build"
)

func GenerateMaven(folder string, findMavenCommand func() string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	var targetParent string
	var desiredPath string
	var defaultArtifact string
	if folder == "." {
		targetParent = cwd
		defaultArtifact = filepath.Base(cwd)
		if defaultArtifact == "." || defaultArtifact == string(filepath.Separator) {
			defaultArtifact = "app"
		}
	} else {
		abs, err := filepath.Abs(folder)
		if err != nil {
			return fmt.Errorf("failed to resolve path %s: %w", folder, err)
		}
		desiredPath = abs
		targetParent = filepath.Dir(abs)
		defaultArtifact = filepath.Base(abs)
		if err := os.MkdirAll(targetParent, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory %s: %w", targetParent, err)
		}
	}

	scanner := bufio.NewScanner(os.Stdin)

	defaultGroup := "me.asu"
	fmt.Printf("Enter groupId (default: %s): ", defaultGroup)
	groupId := defaultGroup
	if scanner.Scan() {
		if v := strings.TrimSpace(scanner.Text()); v != "" {
			groupId = v
		}
	} else if err := scanner.Err(); err != nil {
		return fmt.Errorf("read groupId: %w", err)
	}

	fmt.Printf("Enter artifactId (default: %s): ", defaultArtifact)
	artifactId := defaultArtifact
	if scanner.Scan() {
		if v := strings.TrimSpace(scanner.Text()); v != "" {
			artifactId = v
		}
	} else if err := scanner.Err(); err != nil {
		return fmt.Errorf("read artifactId: %w", err)
	}
	if strings.TrimSpace(artifactId) == "" {
		return errors.New("artifactId is required")
	}

	defaultVersion := "1.0.0-SNAPSHOT"
	fmt.Printf("Enter version (default: %s): ", defaultVersion)
	version := defaultVersion
	if scanner.Scan() {
		if v := strings.TrimSpace(scanner.Text()); v != "" {
			version = v
		}
	} else if err := scanner.Err(); err != nil {
		return fmt.Errorf("read version: %w", err)
	}

	fmt.Print("Select Java version (8/17/21, default: 21): ")
	javaVersion := "21"
	if scanner.Scan() {
		if v := strings.TrimSpace(scanner.Text()); v != "" {
			switch v {
			case "8", "17", "21":
				javaVersion = v
			default:
				build.Info("[MAVEN] Unsupported Java version %s, fallback to 21", v)
			}
		}
	} else if err := scanner.Err(); err != nil {
		return fmt.Errorf("read Java version: %w", err)
	}

	generatedPath := filepath.Join(targetParent, artifactId)
	if _, err := os.Stat(generatedPath); err == nil {
		return fmt.Errorf("target directory %s already exists", generatedPath)
	}

	mvnCmd := findMavenCommand()
	build.Info("[MAVEN] Generating project using %s", mvnCmd)
	args := []string{
		"archetype:generate",
		"-DgroupId=" + groupId,
		"-DartifactId=" + artifactId,
		"-Dversion=" + version,
		"-DarchetypeGroupId=org.apache.maven.archetypes",
		"-DarchetypeArtifactId=maven-archetype-quickstart",
		"-DarchetypeVersion=1.5",
		"-DinteractiveMode=false",
	}
	outb, errb, err := build.RunCommandDir(targetParent, mvnCmd, args...)
	if err != nil {
		return fmt.Errorf("maven archetype generation failed: %v\n%s\n%s", err, outb, errb)
	}
	build.Info("[MAVEN] Archetype generation completed")

	baseDir := generatedPath
	if desiredPath != "" {
		desiredPath = filepath.Clean(desiredPath)
		if !build.SamePath(baseDir, desiredPath) {
			if _, err := os.Stat(desiredPath); err == nil {
				return fmt.Errorf("destination %s already exists", desiredPath)
			}
			if err := os.Rename(baseDir, desiredPath); err != nil {
				return fmt.Errorf("failed to move project to %s: %w", desiredPath, err)
			}
			baseDir = desiredPath
		}
	}

	// 更新 pom.xml 中的 Java 版本和构建配置，而不是完全覆盖
	pomPath := filepath.Join(baseDir, "pom.xml")
	pomBytes, err := os.ReadFile(pomPath)
	if err != nil {
		return fmt.Errorf("failed to read pom.xml: %w", err)
	}

	pomStr, err := configureMavenPom(string(pomBytes), groupId+".App", javaVersion)
	if err != nil {
		return fmt.Errorf("configure generated pom.xml: %w", err)
	}

	if err := os.WriteFile(pomPath, []byte(pomStr), 0644); err != nil {
		return fmt.Errorf("failed to update pom.xml: %w", err)
	}

	build.Info("Generated Maven project in %s", baseDir)

	if err := build.EnsureGitSetup(baseDir); err != nil {
		build.Info("[GIT] %v", err)
	}

	return nil
}
