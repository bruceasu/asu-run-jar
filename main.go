package main

import (
    "archive/zip"
    "bufio"
    "bytes"
    "flag"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
)

var dryRun = flag.Bool("dryrun", false, "Print the java command without executing")

func main() {
    flag.Parse()
    checkJava()
    
    if flag.NArg() < 1 {
        fmt.Println("Usage: jrun [--dryrun] <your.jar> [args...]")
        os.Exit(1)
    }

    jarPath, _ := filepath.Abs(flag.Arg(0))
    args := flag.Args()[1:]

    // 情况 1: jar 在当前目录
    cwd, _ := os.Getwd()
    if filepath.Dir(jarPath) == cwd {
        runOrPrint("java", append([]string{"-jar", filepath.Base(jarPath)}, args...)...)
        return
    }

    // 打开 jar 检查内容
    r, err := zip.OpenReader(jarPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to open jar: %v\n", err)
        os.Exit(1)
    }
    defer r.Close()

    var manifest, depsFile, depsStatic *zip.File
    for _, f := range r.File {
        name := strings.ToLower(f.Name)
        switch name {
        case "meta-inf/dependencies.lib":
            depsFile = f
        case "meta-inf/dependencies-static.lib":
            depsStatic = f
        case "meta-inf/manifest.mf":
            manifest = f
        }
    }

    // 情况 2: 静态 JAR
    if depsStatic != nil {
        runOrPrint("java", append([]string{"-jar", jarPath}, args...)...)
        return
    }

    // 情况 3: 我们的普通 JAR
    if depsFile != nil && manifest != nil {
        mainClass := readMainClass(manifest)
        if mainClass == "" {
            fmt.Fprintln(os.Stderr, "No Main-Class found in manifest")
            os.Exit(1)
        }

        cpParts := []string{jarPath}
        if rc, err := depsFile.Open(); err == nil {
            scanner := bufio.NewScanner(rc)
            jarDir := filepath.Dir(jarPath)
            for scanner.Scan() {
                line := strings.TrimSpace(scanner.Text())
                if line == "" {
                    continue
                }
                depPath := filepath.Join(jarDir, filepath.FromSlash(line))
                cpParts = append(cpParts, depPath)
            }
            rc.Close()
        }

        classpath := strings.Join(cpParts, string(os.PathListSeparator))
        allArgs := append([]string{"-cp", classpath, mainClass}, args...)
        runOrPrint("java", allArgs...)
        return
    }

    // 情况 4: 其它外部 JAR
    runOrPrint("java", append([]string{"-jar", jarPath}, args...)...)
}
// 检查 Java 是否可用
func checkJava() {
    if _, err := exec.LookPath("java"); err == nil {
        return // 找到了 java 命令
    }
    // 尝试从 JAVA_HOME 查
    if home := os.Getenv("JAVA_HOME"); home != "" {
        javaBin := filepath.Join(home, "bin", "java")
        if _, err := os.Stat(javaBin); err == nil {
            return
        }
    }
    fmt.Fprintln(os.Stderr, "[ERR] Java not found in PATH or JAVA_HOME. Please install JDK and set JAVA_HOME or add java to PATH.")
    os.Exit(1)
}
func readMainClass(manifest *zip.File) string {
    rc, err := manifest.Open()
    if err != nil {
        return ""
    }
    defer rc.Close()

    buf := new(bytes.Buffer)
    buf.ReadFrom(rc)
    lines := strings.Split(buf.String(), "\n")
    for _, l := range lines {
        if strings.HasPrefix(strings.ToLower(l), "main-class:") {
            return strings.TrimSpace(strings.SplitN(l, ":", 2)[1])
        }
    }
    return ""
}

func runOrPrint(bin string, args ...string) {
    if *dryRun {
        fmt.Printf("%s %s\n", bin, strings.Join(args, " "))
        return
    }
    cmd := exec.Command(bin, args...)
    cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
    cmd.Run()
}
