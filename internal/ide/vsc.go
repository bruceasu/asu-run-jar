package ide

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bruceasu/asu-run-jar/internal/build"
)

func GenerateVSC(folder string) error {
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
	dirs := []string{filepath.Join(baseDir, ".vscode"), filepath.Join(baseDir, "src"), filepath.Join(baseDir, "lib"), filepath.Join(baseDir, "classes"), filepath.Join(baseDir, "bin")}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create .vscode/settings.json
	settingsPath := filepath.Join(baseDir, ".vscode", "settings.json")
	settingsContent := `{
"java.project.sourcePaths": ["src"],
"java.project.referencedLibraries": ["lib/**/*.jar"],
"java.project.outputPath": "classes",
"java.project.outputPathForJar": "bin",
"java.debug.settings.vmArgs": "-Dfile.encoding=UTF-8",
"java.settings.compilerArgs": ["-encoding", "UTF-8", "-Xlint:unchecked", "-Xlint:deprecation", "-parameters"],
"files.encoding": "utf8",
"java.jdt.ls.vmargs": "-Dfile.encoding=UTF-8",

 // 自动构建
 "java.autobuild.enabled": true,
 // 补全风格
  "editor.suggest.snippetsPreventQuickSuggestions": false,
  "java.completion.favoriteStaticMembers": [
    "org.junit.Assert.*",
    "org.mockito.Mockito.*",
    "org.mockito.ArgumentMatchers.*"
  ],
   // 保存自动格式化
  "editor.formatOnSave": true,
  "java.format.settings.url": "https://raw.githubusercontent.com/google/styleguide/gh-pages/eclipse-java-google-style.xml",
  "java.format.settings.profile": "GoogleStyle"
}`
	if err := os.WriteFile(settingsPath, []byte(settingsContent), 0644); err != nil {
		return fmt.Errorf("failed to write settings.json: %w", err)
	}

	tasksPath := filepath.Join(baseDir, ".vscode", "tasks.json")
	taskContent := `{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "build-java",
      "type": "shell",
      "command": "javac",
      "args": [
        "-encoding", "UTF-8",
        "-d", "classes",
        "-sourcepath", "src",
        "src/Main.java"
      ],
      "group": { "kind": "build", "isDefault": true },
      "problemMatcher": "$javac"
    }
  ]
}`
	if err := os.WriteFile(tasksPath, []byte(taskContent), 0644); err != nil {
		return fmt.Errorf("failed to write tasks.json: %w", err)
	}

	launchPath := filepath.Join(baseDir, ".vscode", "launch.json")
	launchContent := `{
  "version": "0.2.0",
  "configurations": [
    {
      "type": "java",
      "name": "Run Main",
      "request": "launch",
      "mainClass": "Main",
      "cwd": "${workspaceFolder}",
      "vmArgs": "-Dfile.encoding=UTF-8"
    }
  ]
}`
	if err := os.WriteFile(launchPath, []byte(launchContent), 0644); err != nil {
		return fmt.Errorf("failed to write launch.json: %w", err)
	}

	// Create sample Main.java
	mainJavaPath := filepath.Join(baseDir, "src", "Main.java")
	mainJavaContent := `public class Main {
    public static void main(String[] args) {
        System.out.println("Hello from VS Code Java Project!");
    }
}
`
	if err := os.WriteFile(mainJavaPath, []byte(mainJavaContent), 0644); err != nil {
		return fmt.Errorf("failed to write Main.java: %w", err)
	}

	build.Info("Generated VS Code Java project settings in %s", baseDir)

	if err := build.EnsureGitSetup(baseDir); err != nil {
		build.Info("[GIT] %v", err)
	}
	return nil
}
