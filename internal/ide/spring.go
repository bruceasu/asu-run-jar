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

func GenerateSpring(folder string) error {
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

	defaultGroup := "com.example"
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

	fmt.Print("Select Java version (8/17/21, default: 17): ")
	javaVersion := "17"
	if scanner.Scan() {
		if v := strings.TrimSpace(scanner.Text()); v != "" {
			switch v {
			case "8", "17", "21":
				javaVersion = v
			default:
				build.Info("[SPRING] Unsupported Java version %s, fallback to 17", v)
			}
		}
	} else if err := scanner.Err(); err != nil {
		return fmt.Errorf("read Java version: %w", err)
	}

	fmt.Printf("Enter package name (default: %s): ", groupId)
	packageName := groupId
	if scanner.Scan() {
		if v := strings.TrimSpace(scanner.Text()); v != "" {
			packageName = v
		}
	} else if err := scanner.Err(); err != nil {
		return fmt.Errorf("read package name: %w", err)
	}

	defaultDeps := "web"
	fmt.Printf("Enter dependencies (comma-separated, default: %s):\n", defaultDeps)
	fmt.Println("Common dependencies: web,lombok,actuator,validation,websocket,cache,data-jpa,security")
	dependencies := defaultDeps
	if scanner.Scan() {
		if v := strings.TrimSpace(scanner.Text()); v != "" {
			dependencies = v
		}
	} else if err := scanner.Err(); err != nil {
		return fmt.Errorf("read dependencies: %w", err)
	}

	generatedPath := filepath.Join(targetParent, artifactId)
	if _, err := os.Stat(generatedPath); err == nil {
		return fmt.Errorf("target directory %s already exists", generatedPath)
	}

	// 检查 spring 命令是否可用
	springCmd := findSpringCommand()
	if springCmd == "" {
		return errors.New("spring CLI not found. Please install Spring Boot CLI first.\n" +
			"Download from: https://docs.spring.io/spring-boot/installing.html#getting-started.installing.cli")
	}

	build.Info("[SPRING] Generating project using %s", springCmd)
	args := []string{
		"init",
		"--build=maven",
		"--type=maven-project",
		"-g", groupId,
		"-a", artifactId,
		"-v", version,
		"-name", artifactId,
		"--package-name", packageName,
		"--packaging=jar",
		"--java-version=" + javaVersion,
		"--description=" + artifactId + " with Spring Boot",
		"--dependencies=" + dependencies,
		artifactId,
	}

	outb, errb, err := build.RunCommandDir(targetParent, springCmd, args...)
	if err != nil {
		return fmt.Errorf("spring init failed: %v\n%s\n%s", err, outb, errb)
	}
	build.Info("[SPRING] Spring Boot project generation completed")

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

	// 检查并更新 pom.xml（如果需要添加额外配置）
	pomPath := filepath.Join(baseDir, "pom.xml")
	if _, err := os.Stat(pomPath); err == nil {
		pomBytes, err := os.ReadFile(pomPath)
		if err != nil {
			return fmt.Errorf("failed to read pom.xml: %w", err)
		}

		pomStr := string(pomBytes)

		// 确保有 maven-compiler-plugin 的 parameters 配置
		if strings.Contains(pomStr, "maven-compiler-plugin") && !strings.Contains(pomStr, "<parameters>true</parameters>") {
			// 在第一个 <configuration> 标签后添加 parameters（如果有 configuration 标签）
			if strings.Contains(pomStr, "maven-compiler-plugin</artifactId>") {
				// 在 maven-compiler-plugin 的第一个 <configuration> 内添加 parameters
				idx := strings.Index(pomStr, "maven-compiler-plugin</artifactId>")
				if idx > 0 {
					// 查找这个插件后的第一个 <configuration>
					searchStart := pomStr[idx:]
					confIdx := strings.Index(searchStart, "<configuration>")
					if confIdx > 0 {
						// 在 <configuration> 后插入
						insertPos := idx + confIdx + len("<configuration>")
						pomStr = pomStr[:insertPos] + "\n\t\t\t\t\t<parameters>true</parameters>" + pomStr[insertPos:]
					}
				}
			}
		}

		if err := os.WriteFile(pomPath, []byte(pomStr), 0644); err != nil {
			return fmt.Errorf("failed to update pom.xml: %w", err)
		}
	}

	build.Info("Generated Spring Boot project in %s", baseDir)

	if err := build.EnsureGitSetup(baseDir); err != nil {
		build.Info("[GIT] %v", err)
	}

	return nil
}

func findSpringCommand() string {
	// Windows: spring.bat or spring.cmd
	// Unix: spring
	candidates := []string{"spring", "spring.bat", "spring.cmd"}
	for _, cmd := range candidates {
		if _, _, err := build.RunCommandDir(".", cmd, "--version"); err == nil {
			return cmd
		}
	}
	return ""
}
