# asu-run-jar

`asu-run-jar` is a lightweight Java launcher written in Go.
It is designed to run applications packaged with **[asu-java-build](https://github.com/bruceasu/asu-java-build)**, automatically detecting whether a JAR is standalone, fat-JAR, or dependency-aware, and running it with the correct classpath.

---

## Features

* **Automatic detection of JAR type**

  * If the JAR contains `META-INF/dependencies-static.lib` → run with `java -jar`
  * If the JAR contains `META-INF/dependencies.lib` → reconstruct classpath and run
  * Otherwise → fallback to `java -jar`
* **Classpath resolution**

  * Reads dependencies from `dependencies.lib` relative to the JAR location
* **Cross-platform**

  * Works on Windows, Linux, and macOS
* **Debug mode**

  * `-dryrun` option prints the `java` command without executing

---

## Installation

Requirements:

* JDK (with `java` in PATH)

Build from source:

```bash
git clone https://github.com/bruceasu/asu-run-jar.git
cd asu-run-jar
go build -o asu-run-jar main.go
```

On Windows, the output will be `asu-run-jar.exe`.

---

## Usage

```
asu-run-jar [options] <path-to-jar> [-- args...]
```

### Options

| Option    | Description                                   |
| --------- | --------------------------------------------- |
| `-dryrun` | Print the resolved `java` command without run |

### Examples

Run a JAR with automatic detection:

```bash
asu-run-jar bin/app.jar
```

Dry-run mode:

```bash
asu-run-jar -dryrun bin/app.jar
```

Output:

```
java -cp bin/app.jar;bin/lib/a.jar;bin/lib/b.jar com.example.Main
```

Pass program arguments:

```bash
asu-run-jar bin/app.jar -- arg1 arg2
```

---

## How It Works

1. **Fat-JAR (static)**

   * Detected by `META-INF/dependencies-static.lib`
   * Runs with:

     ```bash
     java -jar app.jar
     ```

2. **Standard JAR**

   * Detected by `META-INF/dependencies.lib`
   * Reads dependency list and builds classpath:

     ```bash
     java -cp app.jar;lib/a.jar;lib/b.jar com.example.Main
     ```

3. **Other JARs**

   * Falls back to:

     ```bash
     java -jar app.jar
     ```

---

## Related Projects

* **[asu-java-build](https://github.com/bruceasu/asu-java-build)**: The companion build tool that compiles, packages, and manages dependencies for Java projects.

---

## License

```
Apache License 2.0

Copyright 2025 BruceAsu

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```


