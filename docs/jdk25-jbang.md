# JDK 25 与 JBang：Java 命令行脚本指南

本文整理 JDK 25 紧凑源文件语法、标准库 API 与 JBang 脚本功能的配合方式，适用于快速编写、运行和分发 Java 命令行工具。

## 1. 先理解各自负责什么

| 能力 | JDK `java` 命令 | JBang |
| --- | --- | --- |
| 直接运行 `.java` 源文件 | 支持，JDK 11 起提供源文件启动模式 | 支持，并提供额外脚本能力 |
| `void main()` 等紧凑源文件语法 | 取决于所用 JDK；本文示例使用 JDK 25 | 通过指定 Java 版本使用相应语法 |
| `//DEPS` Maven 依赖声明 | 不识别 | 识别并解析依赖及传递依赖 |
| 多源文件脚本 | 可使用 Java 的源码路径等机制 | 可通过 `//SOURCES` 显式引入源码 |
| 生成可分发 Jar | 需要自行配置构建工具或命令 | 提供 `jbang export` 命令 |

原生 `java` 适合只依赖 JDK 的源文件。需要 Maven 依赖、便捷的多文件脚本或导出 Jar 时，可以使用 JBang。JBang 是额外工具；下文把它作为可选的脚本运行和打包工具介绍。

## 2. 安装与运行第一个脚本

可通过系统包管理器安装 JBang：

| 平台 | 示例命令 |
| --- | --- |
| macOS | `brew install jbang` |
| Linux | `curl -Ls https://sh.jbang.dev | bash -s - app setup` |
| Windows | `winget install jbang` 或 `scoop install jbang` |

保存为 `hello.java`：

```java
///usr/bin/env jbang "$0" "$@" ; exit $?
//JAVA 25

void main(String[] args) {
    IO.println("Hello, Java 25!");
    IO.println("参数：" + String.join(", ", args));
}
```

运行方式：

```sh
jbang hello.java one two
```

在 Linux 或 macOS 上，也可以给文件增加可执行权限后运行 `./hello.java one two`。文件头部的 shebang 用于这种直接执行方式；在 Windows 上通常使用 `jbang hello.java`。

## 3. 声明依赖并使用第三方库

JBang 通过源文件开头的 `//DEPS` 声明 Maven 坐标。JBang 会解析所声明依赖及其传递依赖，并在本地缓存下载内容。

```java
///usr/bin/env jbang "$0" "$@" ; exit $?
//JAVA 25
//DEPS com.google.code.gson:gson:2.11.0

import com.google.gson.Gson;
import java.util.Map;

void main(String[] args) {
    var json = new Gson().toJson(Map.of("tool", "jdk25", "args", args));
    IO.println(json);
}
```

`//DEPS` 的格式是 `groupId:artifactId:version`。版本应由脚本作者明确选择；可以在团队约定或版本目录中集中管理常用版本。

### 3.1 CSV 文件

下面用 Apache Commons CSV 写入并读取带表头的文件：

```java
///usr/bin/env jbang "$0" "$@" ; exit $?
//JAVA 25
//DEPS org.apache.commons:commons-csv:1.12.0

import org.apache.commons.csv.*;

void main() throws Exception {
    var format = CSVFormat.DEFAULT.builder()
            .setHeader("ID", "Name")
            .get();

    try (var writer = Files.newBufferedWriter(Path.of("users.csv"));
         var printer = new CSVPrinter(writer, format)) {
        printer.printRecord(1, "Alice");
        printer.printRecord(2, "Bob");
    }

    var readFormat = CSVFormat.DEFAULT.builder()
            .setHeader()
            .setSkipHeaderRecord(true)
            .get();
    try (var reader = Files.newBufferedReader(Path.of("users.csv"));
         var parser = readFormat.parse(reader)) {
        for (var record : parser) {
            IO.println("ID: %s, Name: %s".formatted(
                    record.get("ID"), record.get("Name")));
        }
    }
}
```

### 3.2 JDBC 数据库

JDBC API 来自 JDK，数据库驱动则作为 Maven 依赖声明。以下示例使用 SQLite；连接 MySQL 或 PostgreSQL 时，替换 JDBC URL 和驱动坐标即可。

```java
///usr/bin/env jbang "$0" "$@" ; exit $?
//JAVA 25
//DEPS org.xerial:sqlite-jdbc:3.47.1.0

import java.sql.*;

void main() throws Exception {
    try (var conn = DriverManager.getConnection("jdbc:sqlite:local_data.db");
         var stmt = conn.createStatement()) {
        stmt.execute("CREATE TABLE IF NOT EXISTS tasks "
                + "(id INTEGER PRIMARY KEY, title TEXT, done INTEGER)");
        stmt.executeUpdate("INSERT INTO tasks (title, done) "
                + "VALUES ('迁移脚本', 1)");

        try (var rs = stmt.executeQuery("SELECT id, title, done FROM tasks")) {
            while (rs.next()) {
                IO.println("[%d] %s - 完成状态：%d".formatted(
                        rs.getInt("id"), rs.getString("title"), rs.getInt("done")));
            }
        }
    }
}
```

常见驱动坐标示例：

| 数据库 | Maven 坐标示例 |
| --- | --- |
| PostgreSQL | `org.postgresql:postgresql:42.7.14` |
| MySQL | `com.mysql:mysql-connector-j:9.1.0` |
| SQLite | `org.xerial:sqlite-jdbc:3.47.1.0` |

以上版本号是示例。使用前应按项目兼容性选择并固定版本。数据库密码建议从环境变量读取，不要写入脚本：

```java
var password = System.getenv("DB_PASSWORD");
```

### 3.3 命令行参数

PicoCLI 可以生成帮助信息并完成参数解析：

```java
///usr/bin/env jbang "$0" "$@" ; exit $?
//JAVA 25
//DEPS info.picocli:picocli:4.7.6

import picocli.CommandLine;
import picocli.CommandLine.*;

@Command(name = "sync-tool", mixinStandardHelpOptions = true)
class SyncTool implements Runnable {
    @Option(names = {"-f", "--file"}, required = true)
    Path inputFile;

    @Option(names = {"-n", "--limit"}, defaultValue = "10")
    int limit;

    public void run() {
        IO.println("处理文件：%s，最多处理 %d 条".formatted(inputFile, limit));
    }
}

void main(String[] args) {
    System.exit(new CommandLine(new SyncTool()).execute(args));
}
```

## 4. 不依赖第三方库时使用 JDK API

只用 JDK 自带能力时，可以直接用 `java SomeScript.java` 运行，不需要 JBang。JDK 25 紧凑源文件可以省略显式的外层类；`IO`、`Files`、`Path` 等 API 可用于常见命令行任务。

| 需求 | 示例 |
| --- | --- |
| 读取文本文件 | `var text = Files.readString(Path.of("a.txt"));` |
| 按行读取 | `try (var lines = Files.lines(Path.of("a.txt"))) { lines.forEach(IO::println); }` |
| 写入文本 | `Files.writeString(Path.of("out.txt"), "hello\n");` |
| 遍历目录 | `Files.walk(Path.of("."))` |
| 正则提取 | `var m = Pattern.compile("id=(\\d+)").matcher(text);` |
| 日期格式化 | `LocalDateTime.now().format(DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm"))` |
| 读取环境变量 | `System.getenv("DB_PASSWORD")` |

上例中使用的 `Files`、`Path`、`Pattern`、`LocalDateTime` 等类型仍需导入，除非当前语言语法或导入方式已覆盖它们。若使用 JDK 25 的模块导入语法，也可以按需写 `import module java.base;`；数据库 API 可写 `import module java.sql;`。

## 5. 拆分和复用多个源文件

JBang 使用 `//SOURCES` 将其他本地 Java 源文件纳入当前脚本。辅助文件应定义显式的类或 record；主文件可以使用紧凑源文件语法。

目录示例：

```text
my-scripts/
├── DbHelper.java
└── export_report.java
```

`DbHelper.java`：

```java
import java.sql.*;
import java.util.*;

public class DbHelper {
    public static List<Map<String, Object>> query(
            String url, String user, String password, String sql) throws SQLException {
        var rows = new ArrayList<Map<String, Object>>();
        try (var conn = DriverManager.getConnection(url, user, password);
             var stmt = conn.prepareStatement(sql);
             var rs = stmt.executeQuery()) {
            var meta = rs.getMetaData();
            while (rs.next()) {
                var row = new LinkedHashMap<String, Object>();
                for (int i = 1; i <= meta.getColumnCount(); i++) {
                    row.put(meta.getColumnLabel(i), rs.getObject(i));
                }
                rows.add(row);
            }
        }
        return rows;
    }
}
```

`export_report.java`：

```java
///usr/bin/env jbang "$0" "$@" ; exit $?
//JAVA 25
//DEPS org.postgresql:postgresql:42.7.14
//SOURCES DbHelper.java

void main() throws Exception {
    var rows = DbHelper.query(
            "jdbc:postgresql://localhost:5432/analytics",
            "postgres",
            System.getenv("PG_PWD"),
            "SELECT id, order_no, amount FROM orders");

    IO.println("查询到 %d 条记录".formatted(rows.size()));
    for (var row : rows) {
        IO.println("订单号：%s，金额：%s".formatted(
                row.get("order_no"), row.get("amount")));
    }
}
```

运行主文件：

```sh
jbang export_report.java
```

公共源码也可以放在其他目录，并在 `//SOURCES` 中使用相对或绝对路径。JBang 支持用通配符引入一组源码，例如 `//SOURCES ../common/*.java`。依赖声明集中放在入口脚本上，辅助类负责实现逻辑。

原生 `java` 与 JBang 的多文件处理方式不同。原生源文件启动适合 JDK 自带依赖的场景；涉及外部依赖时，需自行准备 classpath 或使用构建工具。不要把 JBang 的 `//SOURCES`、`//DEPS` 当作原生 `java` 命令的功能。

## 6. 数据库迁移脚本的组织示例

较复杂的任务可以拆成“读取配置、查询源库、备份 CSV、写入目标库”几个步骤。下面给出结构示意，SQL 和字段应按实际数据库调整：

```java
///usr/bin/env jbang "$0" "$@" ; exit $?
//JAVA 25
//DEPS org.postgresql:postgresql:42.7.14
//DEPS com.mysql:mysql-connector-j:9.1.0
//DEPS org.apache.commons:commons-csv:1.12.0

import java.sql.*;
import org.apache.commons.csv.*;

record UserRow(long id, String username, String email) {}

void main() throws Exception {
    var mysqlUrl = "jdbc:mysql://localhost:3306/source_db";
    var pgUrl = "jdbc:postgresql://localhost:5432/target_db";
    var mysqlPassword = System.getenv("MYSQL_PWD");
    var pgPassword = System.getenv("PG_PWD");

    var users = fetchUsers(mysqlUrl, "root", mysqlPassword);
    exportCsv(users, Path.of("users_backup.csv"));
    insertUsers(pgUrl, "postgres", pgPassword, users);
    IO.println("处理完成：%d 条记录".formatted(users.size()));
}

List<UserRow> fetchUsers(String url, String user, String password) throws SQLException {
    var rows = new ArrayList<UserRow>();
    var sql = "SELECT id, username, email FROM users WHERE active = 1";
    try (var conn = DriverManager.getConnection(url, user, password);
         var stmt = conn.prepareStatement(sql);
         var rs = stmt.executeQuery()) {
        while (rs.next()) {
            rows.add(new UserRow(rs.getLong("id"),
                    rs.getString("username"), rs.getString("email")));
        }
    }
    return rows;
}

void exportCsv(List<UserRow> rows, Path output) throws IOException {
    var format = CSVFormat.DEFAULT.builder()
            .setHeader("ID", "Username", "Email")
            .get();
    try (var writer = Files.newBufferedWriter(output);
         var printer = new CSVPrinter(writer, format)) {
        for (var row : rows) {
            printer.printRecord(row.id(), row.username(), row.email());
        }
    }
}

void insertUsers(String url, String user, String password,
                 List<UserRow> rows) throws SQLException {
    var sql = "INSERT INTO users (id, username, email) VALUES (?, ?, ?)";
    try (var conn = DriverManager.getConnection(url, user, password)) {
        conn.setAutoCommit(false);
        try (var stmt = conn.prepareStatement(sql)) {
            for (var row : rows) {
                stmt.setLong(1, row.id());
                stmt.setString(2, row.username());
                stmt.setString(3, row.email());
                stmt.addBatch();
            }
            stmt.executeBatch();
            conn.commit();
        } catch (SQLException e) {
            conn.rollback();
            throw e;
        }
    }
}
```

导出 CSV 时，也可以按查询结果生成表头并写入记录；对于大结果集，应考虑分批读取和写入，避免把全部数据放进内存。跨数据库迁移还应处理字段类型差异、主键冲突、事务边界和失败后的恢复策略。

## 7. 开发与分发：源码脚本还是 Jar

| 方式 | 适合场景 | 需要考虑 |
| --- | --- | --- |
| `.java` 脚本 + JBang | 个人工具、快速修改、依赖由 `//DEPS` 管理 | 首次运行需要取得工具和依赖；目标环境需要 JBang 或等效运行环境 |
| Fat Jar | 长期运行、团队分发、希望以单个文件交付 | 需要重新打包发布；应在目标 Java 版本和环境中验证 |
| 普通 Jar / portable 导出 | 希望把程序与依赖分开分发 | 需要同时部署依赖文件，并正确设置 classpath |

导出示例：

```sh
jbang export fatjar export_report.java
```

指定输出路径的示例：

```sh
jbang export fatjar -O ./dist/export-report.jar export_report.java
```

普通 Jar 和 portable 导出的用法可按 JBang 版本查看 `jbang export --help`。导出后应检查产物内容、入口类和依赖是否齐全，并在目标机器上验证启动命令；不要仅凭命令成功就假设所有依赖都能在任意运行环境正常工作。

公共工具类是否打包成 Jar，取决于复用范围和发布方式。个人脚本之间少量共享代码可以用一个本地 `common/` 源码目录配合 `//SOURCES`；团队共享、版本化发布或希望隐藏实现时，Jar 更合适。

## 8. 编辑器辅助

脚本不一定需要完整 Maven/Gradle 工程，但编辑器的语言服务通常需要 JDK、Java Language Server（例如 Eclipse JDT Language Server）以及正确的依赖 classpath 才能提供可靠补全。

1. VS Code、Zed、Neovim、Emacs 等编辑器可以连接 Java LSP；具体安装和配置方式取决于编辑器及其 Java 扩展。
2. JBang 脚本中的 `//DEPS` 并不保证所有编辑器都会自动解析。若补全缺少第三方类型，可以使用编辑器支持的 JBang 集成，或生成临时/便携项目配置，再由语言服务加载依赖。
3. 简单脚本也可以通过 Snippets 提供常用模板，例如入口方法、`IO.println`、文件读写和 JBang 依赖头部。

一个轻量的公共门面类可以减少重复输入，但应只封装项目确实反复使用的操作。例如可以在 `Kit.java` 中放置 `read(String)`、`write(String, String)` 等方法，再由脚本用 `//SOURCES` 引入。静态导入虽然能缩短调用代码，也会隐藏方法来源，建议仅在个人脚本或团队风格明确时使用。

## 9. 相关命令速查

| 目标 | 命令 |
| --- | --- |
| 直接运行脚本 | `jbang tool.java` |
| 创建脚本模板 | `jbang init mytool.java` |
| 查看导出选项 | `jbang export --help` |
| 导出 Fat Jar | `jbang export fatjar mytool.java` |
| 原生运行无外部依赖的源文件 | `java mytool.java` |
