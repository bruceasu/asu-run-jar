# ajava

`ajava` runs Java source files and builds Java applications from the command line. It supports single-file scripts, multi-file source projects, Maven projects, and runnable JARs without requiring JBang.

## Features

- Run `.java`, `.jsh`, and `.jar` files with a selected local JDK.
- Compile Java 8 scripts and multi-file scripts with `javac`; use source-file launch mode on JDK 11+ when applicable.
- Declare script dependencies, source files, and resources with `//DEPS`, `//SOURCES`, and `//FILES` directives.
- Build regular or fat JARs, and package application images with `jlink` and `jpackage`.
- Generate IDE project files, Maven/Spring project starters, and a Java tool template.
- Export project dependencies or compact runtimes that include both `java` and `javac`.

## Requirements

- **To build ajava:** Go 1.25.1 or newer.
- **To run Java programs:** a local JDK. JDK 8 is supported for compile-and-run; `.jsh` scripts require JDK 9 or newer.
- **For Maven projects and transitive Maven dependencies:** Maven or a project Maven Wrapper.
- **To export a compact runtime:** JDK 9 or newer with `jlink`.

## Install

Build the CLI from source:

```sh
go build -o ajava ./cmd/ajava
```

On Windows, use `go build -o ajava.exe ./cmd/ajava`.

## Quick start

Run Java source, JShell, or a JAR:

```sh
ajava run Main.java -- input.txt
ajava run script.jsh
ajava run app.jar -- input.txt
```

Choose a JDK by version or installation path:

```sh
ajava run --jdk 17 Main.java
ajava run --jdk /path/to/jdk-21 Main.java
```

Build a project or create starter files:

```sh
ajava build -src src -main app.Main -static
ajava ide vsc ./my-app
ajava project maven ./my-maven-app
ajava new Tool.java
```

## Commands

| Command | Purpose |
|---------|---------|
| `ajava run <file.java\|file.jsh\|file.jar>` | Run a Java source file, JShell script, or JAR |
| `ajava build [options]` | Compile a source project and optionally package it |
| `ajava ide <vsc\|eclipse\|idea> [directory]` | Generate IDE project configuration |
| `ajava project <maven\|spring> [directory]` | Generate a Maven or Spring project |
| `ajava new <File.java>` | Generate a single-file Java tool template |
| `ajava export deps [options]` | Resolve and export project dependencies |
| `ajava export jdk [options]` | Export a compact runtime with a compiler |

Use `ajava --help` for the top-level usage and `ajava build -h` for build options.

## Documentation

- [Running Java files and scripts](docs/running.md): JDK selection, script directives, dependencies, working directories, and run examples.
- [Building projects](docs/building.md): build options, project generation, dependency/runtime exports, packaging, and examples.
- [JBang reference](docs/jdk25-jbang.md): JBang directives and examples, with notes comparing its behavior to Java source-file launch.

## Examples

See [`examples/`](examples/) for sample projects and Java scripts.
