package build

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// IsMavenCoordinate 判断是否为 Maven 坐标格式: groupId:artifactId:version
func IsMavenCoordinate(s string) bool {
	// Windows 绝对路径判断 (如 C:\path 或 D:\path)
	if len(s) >= 3 && s[1] == ':' && (s[2] == '\\' || s[2] == '/') {
		return false
	}

	// Linux/Mac 绝对路径判断 (以 / 开头)
	if strings.HasPrefix(s, "/") {
		return false
	}

	// 包含路径分隔符的不是 Maven 坐标
	if strings.Contains(s, "/") || strings.Contains(s, "\\") {
		return false
	}

	// Maven 坐标格式：groupId:artifactId:version
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return false
	}

	// 三个部分都不能为空
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return false
		}
	}

	return true
}

// ResolveMavenDependency 解析 Maven 依赖：groupId:artifactId:version -> 本地路径或下载
func ResolveMavenDependency(coordinate string) (string, error) {
	parts := strings.Split(coordinate, ":")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid Maven coordinate format: %s (expected groupId:artifactId:version)", coordinate)
	}

	groupId := parts[0]
	artifactId := parts[1]
	version := parts[2]

	// 构建本地 Maven 仓库路径
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	groupPath := strings.ReplaceAll(groupId, ".", string(filepath.Separator))
	localPath := filepath.Join(home, ".m2", "repository", groupPath, artifactId, version,
		fmt.Sprintf("%s-%s.jar", artifactId, version))

	// 检查本地文件是否存在
	if _, err := os.Stat(localPath); err == nil {
		Info("[MAVEN] Found locally: %s", coordinate)
		return localPath, nil
	}

	// 本地不存在，尝试从 Maven 中央仓库下载
	Info("[MAVEN] Downloading: %s", coordinate)
	if err := DownloadMavenArtifact(groupId, artifactId, version, localPath); err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}

	Ok("[MAVEN] Downloaded: %s -> %s", coordinate, localPath)
	return localPath, nil
}

// DownloadMavenArtifact 从 Maven 中央仓库下载 JAR 文件
func DownloadMavenArtifact(groupId, artifactId, version, destPath string) error {
	// Maven 中央仓库 URL
	groupPath := strings.ReplaceAll(groupId, ".", "/")
	jarFileName := fmt.Sprintf("%s-%s.jar", artifactId, version)
	url := fmt.Sprintf("https://repo1.maven.org/maven2/%s/%s/%s/%s",
		groupPath, artifactId, version, jarFileName)

	// 创建目标目录
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// 下载文件
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: artifact not found at %s", resp.StatusCode, url)
	}

	// 写入文件
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		os.Remove(destPath) // 清理失败的下载
		return fmt.Errorf("failed to write file: %w", err)
	}

	Info("[MAVEN] Downloaded %d bytes", written)
	return nil
}

// DetectMavenProject 检测是否为 Maven 项目
func DetectMavenProject() bool {
	// 检查 pom.xml 是否存在
	if _, err := os.Stat("pom.xml"); err == nil {
		return true
	}
	return false
}

// FindMavenCommand 查找 Maven 命令（优先 mvnw，其次 mvn）
func FindMavenCommand() string {
	// 优先使用 Maven Wrapper
	if runtime.GOOS == "windows" {
		if _, err := os.Stat("mvnw.cmd"); err == nil {
			return "mvnw.cmd"
		}
	} else {
		if _, err := os.Stat("mvnw"); err == nil {
			return "./mvnw"
		}
	}

	// 使用系统 Maven
	return "mvn"
}

// MavenCompile Maven 编译
func MavenCompile() error {
	mvnCmd := FindMavenCommand()
	Info("[MAVEN] Compiling with: %s clean compile -DskipTests", mvnCmd)

	cmd := exec.Command(mvnCmd, "clean", "compile", "-DskipTests")
	outb, errb, err := Run(cmd)
	if err != nil {
		return fmt.Errorf("Maven compile failed: %v\n%s\n%s", err, outb, errb)
	}
	return nil
}

// MavenCopyDependencies 使用 Maven 插件复制依赖
func MavenCopyDependencies() error {
	mvnCmd := FindMavenCommand()
	Info("[MAVEN] Copying dependencies to target/lib")
	if err := os.RemoveAll(filepath.Join("target", "lib")); err != nil {
		return fmt.Errorf("clean copied Maven dependencies: %w", err)
	}

	cmd := exec.Command(mvnCmd,
		"dependency:copy-dependencies",
		"-DoutputDirectory=target/lib",
		"-DincludeScope=runtime")
	outb, errb, err := Run(cmd)
	if err != nil {
		return fmt.Errorf("Maven copy dependencies failed: %v\n%s\n%s", err, outb, errb)
	}
	return nil
}

// ExtractMainClassFromPom 从 pom.xml 提取主类
func ExtractMainClassFromPom() (string, error) {
	data, err := os.ReadFile("pom.xml")
	if err != nil {
		return "", err
	}
	type pomProperty struct {
		XMLName xml.Name
		Value   string `xml:",chardata"`
	}
	type pluginConfig struct {
		MainClass string `xml:"mainClass"`
		Archive   struct {
			Manifest struct {
				MainClass string `xml:"mainClass"`
			} `xml:"manifest"`
		} `xml:"archive"`
	}
	type pomPlugin struct {
		GroupID       string       `xml:"groupId"`
		ArtifactID    string       `xml:"artifactId"`
		Configuration pluginConfig `xml:"configuration"`
	}
	var pom struct {
		GroupID  string `xml:"groupId"`
		Artifact string `xml:"artifactId"`
		Version  string `xml:"version"`
		Parent   struct {
			GroupID string `xml:"groupId"`
			Version string `xml:"version"`
		} `xml:"parent"`
		Properties struct {
			Values []pomProperty `xml:",any"`
		} `xml:"properties"`
		Build struct {
			Plugins []pomPlugin `xml:"plugins>plugin"`
		} `xml:"build"`
	}
	if err := xml.Unmarshal(data, &pom); err != nil {
		return "", fmt.Errorf("parse pom.xml: %w", err)
	}
	groupID := pom.GroupID
	if groupID == "" {
		groupID = pom.Parent.GroupID
	}
	properties := map[string]string{
		"project.groupId":        groupID,
		"groupId":                groupID,
		"project.artifactId":     pom.Artifact,
		"artifactId":             pom.Artifact,
		"project.version":        pom.Version,
		"version":                pom.Version,
		"project.parent.version": pom.Parent.Version,
	}
	for _, property := range pom.Properties.Values {
		properties[property.XMLName.Local] = strings.TrimSpace(property.Value)
	}
	mainClass := properties["start-class"]
	if mainClass == "" {
		mainClass = properties["mainClass"]
	}
	if mainClass == "" {
		for _, plugin := range pom.Build.Plugins {
			value := strings.TrimSpace(plugin.Configuration.MainClass)
			if value == "" {
				value = strings.TrimSpace(plugin.Configuration.Archive.Manifest.MainClass)
			}
			if value == "" {
				continue
			}
			if plugin.ArtifactID == "spring-boot-maven-plugin" {
				mainClass = value
				break
			}
			if mainClass == "" {
				mainClass = value
			}
		}
	}
	if mainClass == "" {
		return "", nil
	}
	propertyRef := regexp.MustCompile(`\$\{([^}]+)\}`)
	for range 10 {
		unresolved := false
		mainClass = propertyRef.ReplaceAllStringFunc(mainClass, func(reference string) string {
			name := propertyRef.FindStringSubmatch(reference)[1]
			value, ok := properties[name]
			if !ok {
				unresolved = true
				return reference
			}
			return value
		})
		if !unresolved {
			return strings.TrimSpace(mainClass), nil
		}
		if !strings.Contains(mainClass, "${") {
			return strings.TrimSpace(mainClass), nil
		}
	}
	return "", fmt.Errorf("main class contains unresolved Maven properties: %s", mainClass)
}

// BuildMavenJar 构建 Maven 普通 JAR（从 target/classes）
func BuildMavenJar(jarPath, mainClass string, expLib bool) error {
	classesDir := "target/classes"
	if _, err := os.Stat(classesDir); err != nil {
		return fmt.Errorf("target/classes not found: %w", err)
	}

	// 如果 target/lib 尚不存在（例如没有声明运行时依赖），运行时依赖列表为空。
	libDir := "target/lib"

	// 收集依赖（用于生成 Class-Path）
	var deps []string
	if _, err := os.Stat(libDir); err == nil {
		filepath.Walk(libDir, func(p string, info fs.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(p, ".jar") {
				deps = append(deps, p)
			}
			return nil
		})
	}

	// 创建 manifest
	manifest, err := os.CreateTemp("", "MANIFEST-*.mf")
	if err != nil {
		return fmt.Errorf("create manifest: %w", err)
	}
	defer os.Remove(manifest.Name())

	if err := WriteManifest(manifest.Name(), mainClass, deps, false); err != nil {
		return err
	}
	manifest.Close()

	// 创建 JAR
	cmd := exec.Command("jar", "cfm", jarPath, manifest.Name(), "-C", classesDir, ".")
	outb, errb, err := Run(cmd)
	if err != nil {
		return fmt.Errorf("jar creation failed: %v\n%s\n%s", err, outb, errb)
	}

	// 写 META-INF/dependencies.lib（如果有依赖）
	if len(deps) > 0 {
		tmpDir := filepath.Join(os.TempDir(), "deps-"+filepath.Base(jarPath))
		metaInf := filepath.Join(tmpDir, "META-INF")
		os.MkdirAll(metaInf, 0755)
		depFile := filepath.Join(metaInf, "dependencies.lib")

		f, err := os.Create(depFile)
		if err == nil {
			for _, d := range deps {
				fmt.Fprintln(f, "lib/"+filepath.Base(d))
			}
			f.Close()

			cmd = exec.Command("jar", "uf", jarPath, "-C", tmpDir, "META-INF/dependencies.lib")
			Run(cmd)
		}

		os.RemoveAll(tmpDir)
	}

	// 如果有依赖，复制到输出目录的 lib/（即使不是 expLib 模式，也需要复制以便 Class-Path 能找到）
	outLibDir := filepath.Join(filepath.Dir(jarPath), "lib")
	if err := os.RemoveAll(outLibDir); err != nil {
		return fmt.Errorf("clean output dependencies: %w", err)
	}
	if len(deps) > 0 {
		if err := os.MkdirAll(outLibDir, 0755); err != nil {
			return fmt.Errorf("create output dependency directory: %w", err)
		}

		for _, dep := range deps {
			destFile := filepath.Join(outLibDir, filepath.Base(dep))
			data, err := os.ReadFile(dep)
			if err == nil {
				os.WriteFile(destFile, data, 0644)
			}
		}
		if expLib {
			Ok("[MAVEN] Copied %d dependencies to %s", len(deps), outLibDir)
		}
	}

	return nil
}

// BuildMavenFatJar 构建 Maven Fat JAR
func BuildMavenFatJar(jarPath, mainClass string) error {
	classesDir := "target/classes"
	if _, err := os.Stat(classesDir); err != nil {
		return fmt.Errorf("target/classes not found: %w", err)
	}

	// 收集所有依赖
	var deps []string
	libDir := "target/lib"
	if _, err := os.Stat(libDir); err == nil {
		filepath.Walk(libDir, func(p string, info fs.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(p, ".jar") {
				abs, _ := filepath.Abs(p)
				deps = append(deps, abs)
			}
			return nil
		})
	}

	Info("[MAVEN] Building Fat JAR with %d dependencies", len(deps))

	// 创建 manifest
	manifest, err := os.CreateTemp("", "MANIFEST-*.mf")
	if err != nil {
		return fmt.Errorf("create manifest: %w", err)
	}
	defer os.Remove(manifest.Name())

	if err := WriteManifest(manifest.Name(), mainClass, deps, true); err != nil {
		return err
	}
	manifest.Close()

	// 使用 BuildFatJar 函数
	return BuildFatJar(jarPath, classesDir, manifest.Name(), deps)
}

// BuildMavenProject 处理 Maven 项目的完整构建流程
func BuildMavenProject(
	mainFQCN, outDir, jarName string,
	skipJar, staticMode, expLib bool,
) (jarPath, mainClass string, err error) {
	// Maven 项目构建流程
	Info("[MAVEN] Detected Maven project")

	// 使用 Maven 编译
	if err := MavenCompile(); err != nil {
		return "", "", err
	}
	Ok("[MAVEN] Compiled successfully")

	// 推断主类
	mainClass = strings.TrimSpace(mainFQCN)
	if mainClass == "" {
		// 尝试从 pom.xml 读取主类
		if mc, err := ExtractMainClassFromPom(); err == nil && mc != "" {
			mainClass = mc
			Info("[MAVEN] Found main class from pom.xml: %s", mainClass)
		}
	}

	if mainClass == "" {
		Info("[HINT] Main class not specified. Set -main com.example.Main if you want a runnable JAR.")
	}

	// 确保输出目录存在
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create out dir: %w", err)
	}

	// 处理依赖和打包
	if !skipJar && mainClass != "" {
		jarPath = filepath.Join(outDir, jarName)

		if staticMode {
			// Fat JAR 模式：先 explib，再合并
			Info("[MAVEN] Building Fat JAR with dependencies")
			if err := MavenCopyDependencies(); err != nil {
				return "", "", err
			}
			// 从 target/classes 和依赖构建 Fat JAR
			if err := BuildMavenFatJar(jarPath, mainClass); err != nil {
				return "", "", err
			}
			Ok("[MAVEN] Built Fat JAR: %s", jarPath)
		} else if expLib {
			// explib 模式：使用 Maven 插件复制依赖
			Info("[MAVEN] Copying dependencies using Maven")
			if err := MavenCopyDependencies(); err != nil {
				return "", "", err
			}
			// 构建普通 JAR
			if err := BuildMavenJar(jarPath, mainClass, expLib); err != nil {
				return "", "", err
			}
			Ok("[MAVEN] Built JAR with external dependencies: %s", jarPath)
		} else {
			// 普通模式：复制当前依赖图后打包项目代码。
			if err := MavenCopyDependencies(); err != nil {
				return "", "", err
			}
			if err := BuildMavenJar(jarPath, mainClass, expLib); err != nil {
				return "", "", err
			}
			Ok("[MAVEN] Built JAR: %s", jarPath)
		}

		Info(`[RUN] java -jar "%s"`, jarPath)
	}

	return jarPath, mainClass, nil
}
