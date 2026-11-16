package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bruceasu/asu-run-jar/internal/build"
	"github.com/bruceasu/asu-run-jar/internal/ide"
	"github.com/bruceasu/asu-run-jar/internal/runner"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printUsage(os.Stdout)
		return 0
	}
	switch args[0] {
	case "run":
		return runCommand(args[1:])
	case "build":
		return buildCommand(args[1:])
	case "new":
		return newCommand(args[1:])
	case "ide":
		return ideCommand(args[1:])
	case "project":
		return projectCommand(args[1:])
	case "export":
		return exportCommand(args[1:])
	default:
		if strings.HasSuffix(strings.ToLower(args[0]), ".java") || strings.HasSuffix(strings.ToLower(args[0]), ".jsh") || strings.HasSuffix(strings.ToLower(args[0]), ".jar") {
			return runCommand(args)
		}
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", args[0])
		printUsage(os.Stderr)
		return 2
	}
}

func printUsage(w *os.File) {
	fmt.Fprintln(w, `ajava - run Java source files and build Java applications

Usage:
  ajava run [--jdk <version-or-home>] [--dry-run] <file.java|file.jsh|file.jar> [-- app-args...]
  ajava <file.java|file.jsh|file.jar> [app-args...]
  ajava build [build options]
  ajava ide <vsc|eclipse|idea> [directory]
  ajava project <maven|spring> [directory]
  ajava new <Tool.java>
  ajava export deps [--src source-dir] [--out directory]
  ajava export jdk [--profile cli|http|db|full] [--jdk <jdk-home>] [--out directory]

Run directives in Java files:
  //DEPS group:artifact:version [local.jar|local-directory ...]
  //JDK 17                         Select a local JDK version or path
  //SOURCES helper/Helper.java
  //FILES config.properties assets/ target/path.txt=local/path.txt
  //COMPILE_OPTIONS -parameters
  //RUNTIME_OPTIONS -Xmx1g

Build options:
  -src, -out, -main, -jarname, -static, -explib, -nojar, -jar
  -runtime, -runtime-app-name, -runtime-main-class, -runtime-image-dir,
  -runtime-dist-dir, -runtime-java-opts, -runtime-add-modules,
  -runtime-win-console`)
}

func runCommand(args []string) int {
	flags := flag.NewFlagSet("ajava run", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dryRun := flags.Bool("dry-run", false, "Print the resolved Java command without executing")
	flags.BoolVar(dryRun, "dryrun", false, "Print the resolved Java command without executing")
	jdk := flags.String("jdk", "", "Select a JDK version or installation directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	remaining := flags.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(os.Stderr, "run requires a .jar, .java, or .jsh file")
		return 2
	}
	input := remaining[0]
	appArgs := remaining[1:]
	if len(appArgs) > 0 && appArgs[0] == "--" {
		appArgs = appArgs[1:]
	}
	if err := runner.Run(input, appArgs, *dryRun, *jdk); err != nil {
		var exitErr interface{ ExitCode() int }
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		return 1
	}
	return 0
}

func buildCommand(args []string) int {
	flags := flag.NewFlagSet("ajava build", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	srcDir := flags.String("src", "src", "Java source directory (recursive)")
	outDir := flags.String("out", "bin", "Output directory for .class/JAR/runtime app-image")
	mainClass := flags.String("main", "", "Main class FQCN; inferred when possible")
	jarName := flags.String("jarname", "app.jar", "Runnable JAR name")
	staticMode := flags.Bool("static", false, "Package dependencies inside a Fat JAR")
	expLib := flags.Bool("explib", false, "Export dependencies beside the JAR")
	skipJar := flags.Bool("nojar", false, "Do not create a JAR")
	inputJar := flags.String("jar", "", "Use an existing JAR instead of compiling sources")
	runtimeEnable := flags.Bool("runtime", false, "Create a jlink+jpackage app-image")
	runtimeAppName := flags.String("runtime-app-name", "", "Application name for jpackage")
	runtimeMainClass := flags.String("runtime-main-class", "", "Main class for jpackage")
	runtimeImageDir := flags.String("runtime-image-dir", "", "jlink runtime output directory")
	runtimeDistDir := flags.String("runtime-dist-dir", "", "jpackage app-image output directory")
	runtimeJavaOpts := flags.String("runtime-java-opts", "-Xms256m;-Xmx1g", "Semicolon-separated JVM options")
	runtimeAddModules := flags.String("runtime-add-modules", "jdk.unsupported,java.naming", "Extra jlink modules")
	runtimeWinConsole := flags.Bool("runtime-win-console", true, "Enable --win-console on Windows")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "unexpected build arguments: %s\n", strings.Join(flags.Args(), " "))
		return 2
	}

	var jarPath, resolvedMainClass string
	var err error
	switch {
	case strings.TrimSpace(*inputJar) != "":
		jarPath = build.MustAbs(*inputJar)
		resolvedMainClass = strings.TrimSpace(*mainClass)
		build.Info("[JAR] Using existing JAR: %s", jarPath)
	case build.DetectMavenProject():
		jarPath, resolvedMainClass, err = build.BuildMavenProject(*mainClass, *outDir, *jarName, *skipJar, *staticMode, *expLib)
	default:
		jarPath, resolvedMainClass, err = build.BuildJavaProject(*srcDir, *outDir, *jarName, *mainClass, *skipJar, *staticMode, *expLib)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		return 1
	}
	if !*runtimeEnable {
		return 0
	}
	if strings.TrimSpace(jarPath) == "" {
		fmt.Fprintln(os.Stderr, "[ERROR] -runtime requires a JAR; provide -jar or remove -nojar")
		return 1
	}
	mainForRuntime := strings.TrimSpace(*runtimeMainClass)
	if mainForRuntime == "" {
		mainForRuntime = strings.TrimSpace(*mainClass)
	}
	if mainForRuntime == "" {
		mainForRuntime = resolvedMainClass
	}
	if err := build.BuildRuntimeAppImage(jarPath, *outDir, *runtimeAppName, mainForRuntime, *runtimeImageDir, *runtimeDistDir, *runtimeJavaOpts, *runtimeAddModules, *runtimeWinConsole); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		return 1
	}
	return 0
}

func newCommand(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "new requires a .java output path, such as Tool.java")
		return 2
	}
	if strings.EqualFold(filepath.Ext(args[0]), ".java") {
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "new <Tool.java> accepts exactly one output path")
			return 2
		}
		if err := ide.GenerateToolScript(args[0]); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(os.Stderr, "unknown Java template output %q; expected a .java file\n", args[0])
	return 2
}

func ideCommand(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: ajava ide <vsc|eclipse|idea> [directory]")
		return 2
	}
	kind := strings.ToLower(args[0])
	folder := "."
	if len(args) == 2 {
		folder = args[1]
	} else if len(args) > 2 {
		fmt.Fprintln(os.Stderr, "ide accepts one optional output directory")
		return 2
	}
	var err error
	switch kind {
	case "vsc", "vscode":
		err = ide.GenerateVSC(folder)
	case "eclipse":
		err = ide.GenerateEclipse(folder)
	case "idea", "intellij":
		err = ide.GenerateIdea(folder)
	default:
		fmt.Fprintf(os.Stderr, "unknown IDE project template %q\n", kind)
		return 2
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		return 1
	}
	return 0
}

func projectCommand(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: ajava project <maven|spring> [directory]")
		return 2
	}
	kind := strings.ToLower(args[0])
	folder := "."
	if len(args) == 2 {
		folder = args[1]
	} else if len(args) > 2 {
		fmt.Fprintln(os.Stderr, "project accepts one optional output directory")
		return 2
	}
	var err error
	switch kind {
	case "maven":
		err = ide.GenerateMaven(folder, build.FindMavenCommand)
	case "spring", "spring-boot":
		err = ide.GenerateSpring(folder)
	default:
		fmt.Fprintf(os.Stderr, "unknown project template %q; choose maven or spring\n", kind)
		return 2
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		return 1
	}
	return 0
}

func exportCommand(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: ajava export <deps|jdk> [options]")
		return 2
	}
	switch strings.ToLower(args[0]) {
	case "deps":
		return exportDependenciesCommand(args[1:])
	case "jdk":
		return exportJDKCommand(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown export target %q; choose deps or jdk\n", args[0])
		return 2
	}
}

func exportDependenciesCommand(args []string) int {
	flags := flag.NewFlagSet("ajava export deps", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	sourceDir := flags.String("src", "src", "Java source directory (recursive); ignored for Maven projects")
	output := flags.String("out", filepath.Join("bin", "lib"), "Dependency output directory")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "unexpected export deps arguments: %s\n", strings.Join(flags.Args(), " "))
		return 2
	}
	if err := build.ExportDependencies(*sourceDir, *output); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		return 1
	}
	return 0
}

func exportJDKCommand(args []string) int {
	flags := flag.NewFlagSet("ajava export jdk", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	profile := flags.String("profile", "full", "Runtime profile: cli, http, db, or full")
	jdkHome := flags.String("jdk", "", "JDK home containing jlink (defaults to PATH/JAVA_HOME)")
	output := flags.String("out", "jre", "Output directory for the compact JDK image")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "unexpected export jdk arguments: %s\n", strings.Join(flags.Args(), " "))
		return 2
	}
	if err := build.BuildCompactJDK(*profile, *output, *jdkHome); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		return 1
	}
	return 0
}
