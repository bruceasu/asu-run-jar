# Running Java scripts and applications with ajava

`ajava` runs Java source files and JShell scripts with the selected local JDK. It
resolves Maven dependencies through Maven or the project's Maven Wrapper; it does
not install or require JBang.

The working directory for a `.java`, `.jsh`, or `.jar` run is the directory
containing the input file. Relative application paths and `//FILES` targets
therefore resolve beside the script or JAR.

## Command syntax

```text
ajava run [--jdk <version-or-home>] [--dry-run] <file.java|file.jsh|file.jar> [app-args...]
ajava <file.java|file.jsh|file.jar> [app-args...]
```

Use `ajava run` when passing ajava options. The shorter form runs a file with
default JDK selection and passes following arguments to the application.

```sh
ajava run --jdk 17 tools/report.java --format csv
ajava run --dry-run tools/report.java
ajava tools/report.java --format csv
ajava run app.jar --input data.csv
ajava run console.jsh
```

`--dry-run` prints the resolved Java command without executing it. For `.java`
files, JDK selection uses this precedence: `--jdk`, then `//JDK` in the entry
file, then an installed JDK from PATH/`JAVA_HOME`, then a compatible `jre/` or
`runtime/` image beside the source. A `//JDK <version>` can select a matching
adjacent image if that version is not installed elsewhere. An explicit invalid
JDK path is an error. JARs use PATH/`JAVA_HOME` first, then an adjacent runtime
new enough for the JAR's main class.

## Java source files

For a single `.java` file on JDK 11 or newer, `ajava` uses the JDK source-file
launcher when no compile options are requested. JDK 25 can run compact source
files with a top-level `void main()`. JDK 8, scripts with `//COMPILE_OPTIONS`,
and multi-file scripts use `javac` followed by `java`.

```java
//JDK 17
//RUNTIME_OPTIONS -Xmx1g

void main(String[] args) {
    IO.println("Arguments: " + String.join(", ", args));
}
```

Run it with `ajava run hello.java first second`. For a specific installation,
use either `--jdk <path>` or `//JDK <path>`.

## Directives

Directives must start at the beginning of a line. They can be repeated where
applicable; list each dependency, source, file, or option on its own line for
readability.

| Directive | Behavior |
| --- | --- |
| `//DEPS` | Maven coordinates, imported BOMs, local JARs, directories, and globs. Maven resolves transitive dependencies when available. |
| `//JDK` | Select a local JDK version or installation directory for this script. |
| `//SOURCES` | Add local `.java` files, directories, or globs. Declarations are followed recursively. |
| `//FILES` | Stage local files/directories into the script working directory (the entry file's directory). `target=source` mappings are supported. For `ajava run`, declare resources in the entry file. |
| `//COMPILE_OPTIONS` | Pass additional options to `javac`; this selects compile-then-run for `.java` scripts. |
| `//RUNTIME_OPTIONS` | Pass additional JVM options when running a script. |

### Dependencies follow source files

`ajava run` reads `//DEPS` from the entry file and every Java file reached
through its recursive `//SOURCES` graph. The entry file is processed first.
Repeated coordinates and local JAR paths are de-duplicated. If the same Maven
group and artifact are declared with different explicit versions, dependency
resolution reports a conflict.

Local paths in processed `//SOURCES`, `//FILES`, and `//DEPS` directives are
resolved relative to the file that declares them. Maven coordinates are
resolved using Maven or the project's Maven Wrapper from the entry script's
directory. `//FILES`, `//JDK`, `//COMPILE_OPTIONS`, and `//RUNTIME_OPTIONS`
control the whole run and are read from the entry file.

```text
tools/
├── report.java
└── common/
    ├── DbHelper.java
    └── vendor/
        └── jdbc-driver.jar
```

`report.java`:

```java
//JDK 17
//DEPS org.apache.commons:commons-csv:1.12.0
//SOURCES common/DbHelper.java

void main() throws Exception {
    DbHelper.exportReport();
}
```

`common/DbHelper.java`:

```java
//DEPS vendor/jdbc-driver.jar

public class DbHelper {
    public static void exportReport() {
        // Use the JDBC driver declared above.
    }
}
```

Run from any working directory by passing the script path:

```sh
ajava run /path/to/tools/report.java
```

## Generate a starter tool

`ajava new Tool.java` creates one Java file with Picocli command-line parsing,
JDBC with a local SQLite example, Gson JSON serialization, and the JDK
`HttpClient`. Change the class and dependencies as needed for a remote database
such as PostgreSQL or MySQL.

```sh
ajava new Tool.java
ajava run Tool.java --help
ajava run Tool.java --db-url jdbc:sqlite:tool.db --http https://example.com hello
```

The generated file selects JDK 17. `--http` is optional; without it the example
does not make a network request.

## Create compact JDK images

`ajava export jdk` uses `jlink` to create a compact JDK image containing both
`java` and `javac`, so it can run JARs and compile/run Java source files. The image is
written to `jre/` by default. Run the command from the directory containing the
scripts you want the image to accompany:

```sh
ajava export jdk --profile cli --out jre-cli
ajava export jdk --profile http --out jre-http
ajava export jdk --profile db --out jre-db
ajava export jdk --profile full --out jre-full
```

Profiles include the compiler and the selected feature modules. `cli` is for
basic command-line tools; `http` adds the JDK HTTP client and TLS crypto
provider; `db` adds JDBC, naming, and common database support modules; `full`
combines HTTP and DB support for the generated starter. JDBC vendor drivers
remain Maven dependencies declared with `//DEPS`.

For example, put a version-17 `jre/` beside `Tool.java`, then run the script
without installing that JDK globally. `//JDK 17` selects the matching adjacent
image. The `--jdk` option takes precedence. `jlink` must come from an installed
JDK 9 or newer; pass its home with `--jdk` if it is not on PATH. Existing
output directories are not overwritten. The current JDK's architecture and
operating system determine the exported image, so build one image per target
platform.

### Resources

`//FILES` copies resources into the script directory when the script runs.
Existing target files are preserved. To map a resource to a different
target name, write `target=source`:

```java
//FILES config/app.properties
//FILES templates/
//FILES settings.json=config/dev-settings.json
```

For `ajava build` on a plain Java source project, declared files are also
included in the generated JAR. Maven projects are built by Maven; their
resources and dependencies should be configured in `pom.xml`.

## JShell scripts

Files ending in `.jsh` run through the selected JDK's JShell. Top-level
statements do not need a `main` method. JShell requires JDK 9 or newer. Java
files declared with `//SOURCES` are compiled first and added to the classpath.

```java
//JDK 17
//DEPS com.google.code.gson:gson:2.11.0

import com.google.gson.Gson;

IO.println(new Gson().toJson(java.util.Map.of("status", "ok")));
```

Run with `ajava run query.jsh`.

## Existing JAR files

`ajava run app.jar [app-args...]` runs a JAR with the selected Java executable.
Use the standard `java -jar` entry point and manifest classpath behavior. Script
directives are not read from JAR files.

## Limits

1. No GitHub-to-JitPack conversion or arbitrary remote Java source downloads.
2. No automatic JDK installation or download. JDK versions are selected from
   explicit paths, `JAVA_HOME_<version>` variables, and matching PATH entries.
3. Maven dependencies require Maven or the project's Maven Wrapper. Without
   Maven, only explicitly resolvable dependencies can be used; transitive
   dependency resolution is unavailable.
4. JBang catalogs, aliases, and script installation are outside the current
   scope.
