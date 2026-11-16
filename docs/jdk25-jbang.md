- **结合 JDK 25 的紧凑源文件特性与 JBang 工具，你可以实现“零工程配置、自动拉取 Maven 依赖、一行命令直接运行”的脚本开发体验。** 下面是为你整理的 **JDK 25 命令行小工具实战速查表**，涵盖标准库常用场景以及 CSV、数据库（DB）、JSON 等第三方库的极简用法。

-  

- 一、 引入第三方库的标准姿势：JBang + JDK 25

- 原生的 java 命令虽然能直接跑 .java 文件，但遇到 CSV、数据库驱动、JSON 等第三方库时，手动下载 .jar 并拼接 -cp 参数非常痛苦。**JBang** 是目前公认的最佳搭档，它完全兼容 JDK 25 的 void main() 紧凑语法，并支持在代码注释中直接声明 Maven 依赖。

- - **安装 JBang**（任选一种）：
  - macOS / Linux：brew install jbang 或 curl -Ls https://jbang.dev | bash -s      - app setup
  - Windows：winget install jbang 或 scoop      install jbang
  - **基础脚本模板**（保存为 tool.java，直接运行 jbang      tool.java 或 ./tool.java）：
                java
                ///usr/bin/env jbang "$0" "$@" ; exit      $?
                //JAVA 25
                //DEPS com.google.code.gson:gson:2.11.0
                
                import com.google.gson.Gson;
                
                void main(String[] args) {
                  var json = new      Gson().toJson(Map.of("tool", "jdk25",      "args", args));
                  IO.println(json);
                }
                
                Use code with caution.

-  

- 二、 第三方库实战速查（CSV / 数据库 / JSON）

- **1. 处理 CSV 文件（使用 Apache Commons CSV）**

- 无需建复杂的实体类，直接按列名读取或写入 CSV：

- java

- ///usr/bin/env jbang "$0" "$@" ; exit $?
    //JAVA 25
    //DEPS org.apache.commons:commons-csv:1.12.0

- import org.apache.commons.csv.*;

- void main() throws Exception {
      // 1. 写入 CSV
      try (var printer = new CSVPrinter(new FileWriter("users.csv"), CSVFormat.DEFAULT.builder().setHeader("ID", "Name").build())) {
        printer.printRecord(1, "Alice");
        printer.printRecord(2, "Bob");
      }
  
- // 2. 读取 CSV（自动按首行表头解析）
      var format = CSVFormat.DEFAULT.builder().setHeader().setSkipHeaderRecord(true).build();
      try (var reader = new FileReader("users.csv");
         var parser = format.parse(reader)) {
        for (var record : parser) {
          IO.println("ID: %s, 姓名: %s".formatted(record.get("ID"), record.get("Name")));
        }
      }
    }
  
- Use code with caution.

-  

- **2. 操作数据库 DB（以 SQLite 为例，支持 MySQL/PG）**

- 利用 JDK 25 的 import module java.sql; 一键导入 JDBC 核心类，搭配 JBang 自动下载数据库驱动：

- java

- ///usr/bin/env jbang "$0" "$@" ; exit $?
    //JAVA 25
    //DEPS org.xerial:sqlite-jdbc:3.47.1.0
    // 若连 MySQL 可替换为: com.mysql:mysql-connector-j:9.1.0
    
- import module java.sql;

- void main() throws Exception {
      // 连接本地 SQLite 数据库（不存在会自动创建）
      try (var conn = DriverManager.getConnection("jdbc:sqlite:local_data.db");
         var stmt = conn.createStatement()) {
        
        stmt.execute("CREATE TABLE IF NOT EXISTS tasks (id INTEGER PRIMARY KEY, title TEXT, done INT)");
        stmt.execute("INSERT INTO tasks (title, done) VALUES ('迁移Python脚本', 1)");
  
- var rs = stmt.executeQuery("SELECT id, title, done FROM tasks");
        while (rs.next()) {
          IO.println("[%d] %s - 完成状态: %d".formatted(
             rs.getInt("id"), rs.getString("title"), rs.getInt("done")
          ));
        }
      }
    }
    
- Use code with caution.

-  

- **3. 命令行参数解析（使用 PicoCLI）**

- 写复杂工具时，自动生成 --help 帮助文档和类型转换（无需自己手写 if (args[0].equals(...))）：

- java

- ///usr/bin/env jbang "$0" "$@" ; exit $?
    //JAVA 25
    //DEPS info.picocli:picocli:4.7.6

- import picocli.CommandLine;
    import picocli.CommandLine.*;

- @Command(name = "sync-tool", mixinStandardHelpOptions = true, description = "数据同步小工具")
    class SyncTool implements Runnable {
      @Option(names = {"-f", "--file"}, required = true, description = "输入文件路径")
      Path inputFile;
    
- @Option(names = {"-n", "--limit"}, defaultValue = "10", description = "最大处理行数")
      int limit;
  
- public void run() {
        IO.println("正在处理文件: %s, 限制行数: %d".formatted(inputFile, limit));
      }
    }
    
- void main(String[] args) {
      int exitCode = new CommandLine(new SyncTool()).execute(args);
      System.exit(exitCode);
    }

- Use code with caution.

-  

-  

- 三、 原生 JDK 25 高频 API 速查（零依赖直接用）

- 对于不涉及外部格式的基础操作，直接用 JDK 25 内置 API 即可，运行速度极快：

- | **常见需求**         | **JDK 25 极简写法（无需任何** **import****）**               |
  | -------------------- | ------------------------------------------------------------ |
  | **读取整个文本文件** | String text   = Files.readString(Path.of("a.txt"));          |
  | **按行读取大文件**   | try (var   lines = Files.lines(Path.of("a.txt"))) {   lines.forEach(IO::println); } |
  | **写入文本文件**     | Files.writeString(Path.of("out.txt"),   "hello\n", StandardOpenOption.CREATE); |
  | **遍历目录找文件**   | Files.walk(Path.of(".")).filter(p   -> p.toString().endsWith(".csv")).toList(); |
  | **正则提取文本**     | var m =   Pattern.compile("id=(\\d+)").matcher(str); if (m.find())   IO.println(m.group(1)); |
  | **日期时间格式化**   | var now =   LocalDateTime.now().format(DateTimeFormatter.ofPattern("yyyy-MM-dd   HH:mm")); |
  | **多线程并发处理**   | urls.parallelStream().map(this::fetch).toList();             |
  | **读取环境变量**     | String   dbPwd = System.getenv("DB_PASSWORD");               |

- 实用小贴士：如何管理常用的“依赖头”？

- 如果你经常要写包含一堆 //DEPS 的脚本，可以用 JBang 的模板功能初始化脚本，无需每次手敲头部注释：

- - 执行 jbang init -t cli mytool.java 可以直接生成一个带 PicoCLI      参数解析的脚手架文件。
  - 首次运行 jbang      mytool.java 时会自动下载依赖并缓存，**第二次及以后运行都是毫秒级启动**。

- 需要我针对你常用的具体数据库类型（如 MySQL / PostgreSQL）或某个特定的数据处理场景，为你组合一个开箱即用的完整脚本模板吗？

-  

-  

-  

- **结合 JDK 25 的紧凑源文件特性与 JBang 工具，你可以用单文件脚本同时连接 PostgreSQL 与 MySQL，并完成跨库数据迁移或 CSV 导出导入。** 下面为你提供一套开箱即用的完整脚本模板，涵盖了最新的驱动依赖、环境变量读取以及批量事务处理。

-  

- \1. 完整脚本模板：跨库同步与 CSV 导出（db_sync.java）

- 这个脚本演示了三个高频小工具场景：**从 MySQL 查询数据 -> 导出为本地 CSV 备份 -> 批量写入 PostgreSQL**。全部代码无需写外层 class，直接用 jbang db_sync.java 即可运行。

- java

- ///usr/bin/env jbang "$0" "$@" ; exit $?
    //JAVA 25
    //DEPS org.postgresql:postgresql:42.7.14
    //DEPS com.mysql:mysql-connector-j:26.10.0
    //DEPS org.apache.commons:commons-csv:1.14.1

- import module java.sql;
    import org.apache.commons.csv.*;

- // 使用 JDK 25 的 Record 极简定义数据行结构，省去 Getter/Setter
    record UserRow(long id, String username, String email) {}

- void main() throws Exception {
      // 1. 优先从环境变量读取密码，避免硬编码泄露
      var mysqlUrl = "jdbc:mysql://localhost:3306/source_db?useSSL=false&serverTimezone=UTC";
      var mysqlPwd = System.getenv().getOrDefault("MYSQL_PWD", "123456");
  
- var pgUrl = "jdbc:postgresql://localhost:5432/target_db";
      var pgPwd = System.getenv().getOrDefault("PG_PWD", "123456");

- IO.println("1. 正在从 MySQL 读取数据...");
      var users = fetchFromMysql(mysqlUrl, "root", mysqlPwd);
      IO.println("共读取到 " + users.size() + " 条记录。");
  
- IO.println("2. 正在导出 CSV 备份...");
      exportToCsv(users, Path.of("users_backup.csv"));
  
- IO.println("3. 正在批量写入 PostgreSQL...");
      batchInsertToPostgres(pgUrl, "postgres", pgPwd, users);
       IO.println("全部处理完成！");
    }
  
- // 从 MySQL 查询数据
    List<UserRow> fetchFromMysql(String url, String user, String pwd) throws SQLException {
      var list = new ArrayList<UserRow>();
      var sql = "SELECT id, username, email FROM users WHERE active = 1";

- try (var conn = DriverManager.getConnection(url, user, pwd);
         var stmt = conn.prepareStatement(sql);
         var rs = stmt.executeQuery()) {
        while (rs.next()) {
          list.add(new UserRow(
             rs.getLong("id"),
             rs.getString("username"),
             rs.getString("email")
          ));
        }
      }
      return list;
    }

- // 导出到 CSV（Apache Commons CSV 1.14+ 推荐用法）
    void exportToCsv(List<UserRow> rows, Path output) throws IOException {
      var format = CSVFormat.DEFAULT.builder()
          .setHeader("ID", "Username", "Email")
          .get();

- try (var writer = Files.newBufferedWriter(output);
         var printer = new CSVPrinter(writer, format)) {
        for (var row : rows) {
           printer.printRecord(row.id(), row.username(), row.email());
        }
      }
    }

- // 批量写入 PostgreSQL（带事务控制与冲突更新）
    void batchInsertToPostgres(String url, String user, String pwd, List<UserRow> rows) throws SQLException {
      var sql = """
          INSERT INTO users (id, username, email)
          VALUES (?, ?, ?)
          ON CONFLICT (id) DO UPDATE SET
            username = EXCLUDED.username,
            email = EXCLUDED.email
          """;

- try (var conn = DriverManager.getConnection(url, user, pwd)) {
        conn.setAutoCommit(false); // 开启手动事务，大幅提升批量写入性能
        try (var pstmt = conn.prepareStatement(sql)) {
          for (var row : rows) {
            pstmt.setLong(1, row.id());
            pstmt.setString(2, row.username());
            pstmt.setString(3, row.email());
            pstmt.addBatch();
          }
          pstmt.executeBatch();
          conn.commit();
        } catch (Exception e) {
          conn.rollback();
          throw e;
        }
      }
    }
    
- Use code with caution.

-  

-  

- \2. 脱离 IDE 写数据库脚本的实用技巧

- - **一行代码直接把 SQL 查询结果转成 CSV**：
                如果你只是想把某个查询结果快速导出，[Apache       Commons CSV](https://commons.apache.org/csv/) 的 CSVPrinter 原生支持直接传入 JDBC 的 ResultSet，连实体类都不用建：
                java
                var format = CSVFormat.DEFAULT.builder().setHeader(rs).get();      // 自动读取表字段名作为表头
                try (var printer = new CSVPrinter(new      FileWriter("dump.csv"), format)) {
                  printer.printRecords(rs);      // 自动遍历并写入所有行
                }
                
            
                Use code with caution.
            
  - **使用文本块（****"""****）写复杂 SQL**：
                在脚本里写多表 JOIN 或长 SQL      时，直接用三个双引号换行书写，保留完整的缩进可读性，排查报错时比拼接字符串轻松得多。
  - **驱动坐标注意版本匹配**：
  - **PostgreSQL 驱动**：统一使用 org.postgresql:postgresql（当前最新稳定版为 [42.7.14](https://mvnrepository.com/artifact/org.postgresql/postgresql)）。
- **MySQL 驱动**：注意新版坐标已统一为 com.mysql:mysql-connector-j（当前最新为 [26.10.0](https://mvnrepository.com/artifact/com.mysql/mysql-connector-j)），不要再使用旧的 mysql:mysql-connector-java。 [[1](https://dbschema.com/blog/mysql/mysql-connector-j-mysql-connector-j-vs-mysql-connector-java/), [2](https://mvnrepository.com/artifact/org.postgresql/postgresql), [3](https://mvnrepository.com/artifact/com.mysql/mysql-connector-j)]
  
- 如果你想进一步简化数据库操作（例如不想手写原生 JDBC 的 ResultSet 映射，想用像 Python 那样直接返回 List<Map<String, Object>> 的轻量库），我可以为你推荐几款极简的 **JDBC 封装库（如 JDBI 或 Apache DbUtils）** 及其 JBang 用法，需要了解一下吗？

-  

-  

-  

- **不需要强制放在同一个 Package 里，你可以根据脚本的运行方式来灵活选择最顺手的组织方式。**

- 具体分为以下两种情况：

- \1. 用原生 java 命令直接运行（JDK 22+ / 25 多文件特性）

- **最省事的做法是：全部放在同一个文件夹下，且全部不写** **package** **声明（即默认无名包）。**

- - **默认包（推荐）**：只要几个 .java 文件都在同一个目录下，且都没有写 package 语句，主文件 Main.java      就可以直接调用同目录下其他文件里的类（比如 DbHelper.java），**一行** **import** **都不用写**。运行时直接敲 java      Main.java，JVM 会自动查找并编译同目录下的关联文件。
  - **如果非要建子目录分 Package**：你必须严格遵守 Java 的目录映射规则（例如 package      utils; 必须放在 ./utils/ 子文件夹里），并且在主文件里手动写 import      utils.DbHelper;。写日常小工具时这样做反而增加了多余的目录层级，**通常不建议分包**。

- \2. 用 JBang 运行（带第三方库依赖时）

- 如果你使用了 **JBang** 来引入 MySQL/PG 驱动或 CSV 库，它对多文件拆分的处理更加灵活，甚至支持跨目录复用公共组件：

- - **方式 A：同目录不写 Package（和原生一样简单）
    **            把 main.java 和 DbUtil.java 放在同一个文件夹里，都不写 package。只要在主文件顶部声明一次      //SOURCES DbUtil.java（或者直接用通配符 //SOURCES *.java），JBang      就会把它们当作一个整体自动编译运行。
  - **方式 B：跨文件夹复用公共库（写多个脚本时强烈推荐）
    **            如果你以后写了 10 个不同的数据库小工具，不想每个文件夹都复制一份 DbUtil.java，你可以把公共代码放在单独的文件夹（如      ~/scripts/common/DbUtil.java），里面依然**不需要写** **package**。
                在其他任意目录的脚本顶部，只需要加一行相对路径或绝对路径声明：
                java
                ///usr/bin/env jbang "$0" "$@" ; exit      $?
                //JAVA 25
                //DEPS org.postgresql:postgresql:42.7.14
                //SOURCES ../common/DbUtil.java
                
                void main() {
                  // 直接调用 DbUtil 的方法，无需      import
                  DbUtil.query("SELECT      1");
                }
                
                Use code with caution.

- 总结与避坑提醒

- 1. **辅文件必须有显式的** **class** **/** **record**：主入口文件可以用 JDK 25 的“紧凑源文件”（不写 class，直接写      void main()），但**被其他文件调用的辅助文件（如** **DbUtil.java****），里面必须老老实实写** **class DbUtil { ...      }** **或** **record ...**。因为紧凑源文件生成的类名是隐式的，外部文件无法引用它。
  2. **第三方依赖统一写在主文件**：如果用 JBang，//DEPS      依赖声明建议集中写在主入口文件的顶部，辅文件里直接 import 对应的类即可，这样依赖管理最清晰。

- 需要我为你演示一个**“主脚本 + 独立数据库工具类（双文件结构）”**的完整代码拆分示例，方便你直接作为多文件小工具的起手模板吗？

-  

-  

-  

-  

- **使用“主入口紧凑源文件 + 独立工具类文件”的双文件结构，既能保留** **void main()** **的极简体验，又能实现代码的模块化和跨脚本复用。** 下面为你提供一套完整的双文件拆分模板，包含数据库查询（返回 List<Map> 摆脱繁琐的 ResultSet 遍历）以及 CSV 导出功能。

-  

- 目录结构（无需建子文件夹，也无需写 package）

- 假设你在任意目录下创建了以下两个文件：

- text

- my-scripts/
    ├── DbHelper.java  <-- 公共工具类（提供极简 DB 查询和 CSV 写入）
    └── export_report.java <-- 主执行脚本（使用 JDK 25 紧凑语法）
    
- Use code with caution.

-  

-  

- 文件 1：DbHelper.java（独立公共工具类）

- 注意：**被调用的辅助文件必须定义显式的** **class**，这样主脚本才能通过类名直接调用它的方法。

- java

- // 不写 package，默认处于无名包中，同目录或通过 //SOURCES 引入即可直接调用
    import module java.sql;
    import org.apache.commons.csv.*;
    
- public class DbHelper {

- // 1. 通用查询：直接把 ResultSet 转成 List<Map>，像 Python 一样按列名取值
      public static List<Map<String, Object>> query(String url, String user, String pwd, String sql, Object... params) throws SQLException {
        var result = new ArrayList<Map<String, Object>>();

- try (var conn = DriverManager.getConnection(url, user, pwd);
           var stmt = conn.prepareStatement(sql)) {
       
          // 自动绑定 ? 占位符参数
          for (int i = 0; i < params.length; i++) {
            stmt.setObject(i + 1, params[i]);
          }


- try (var rs = stmt.executeQuery()) {
            var meta = rs.getMetaData();
            int colCount = meta.getColumnCount();

- while (rs.next()) {
              // 使用 LinkedHashMap 保持查询列的原始顺序
              var row = new LinkedHashMap<String, Object>();
              for (int i = 1; i <= colCount; i++) {
                 row.put(meta.getColumnLabel(i), rs.getObject(i));
              }
               result.add(row);
            }
          }
        }
        return result;
      }


- // 2. 通用导出：直接将查询出来的 List<Map> 写入 CSV 文件
      public static void saveToCsv(List<Map<String, Object>> rows, Path outputFile) throws IOException {
        if (rows.isEmpty()) {
           IO.println("警告：数据为空，跳过生成 CSV。");
          return;
        }
  
- // 自动提取第一行的 Key 作为 CSV 表头
        var headers = rows.getFirst().keySet().toArray(String[]::new);
        var format = CSVFormat.DEFAULT.builder().setHeader(headers).get();

- try (var writer = Files.newBufferedWriter(outputFile);
           var printer = new CSVPrinter(writer, format)) {
          for (var row : rows) {
             printer.printRecord(row.values());
          }
        }
      }
    }

- Use code with caution.

-  

-  

- 文件 2：export_report.java（主业务脚本）

- 主脚本采用 **JDK 25 紧凑源文件语法**（无外层 class），通过 //SOURCES 引入工具类，业务逻辑非常干净：

- java

- ///usr/bin/env jbang "$0" "$@" ; exit $?
    //JAVA 25
    //DEPS org.postgresql:postgresql:42.7.14
    //DEPS com.mysql:mysql-connector-j:26.10.0
    //DEPS org.apache.commons:commons-csv:1.14.1
    //SOURCES DbHelper.java

- void main() throws Exception {
      // 1. 从 PostgreSQL 查询数据（支持直接传 SQL 参数，防注入）
      var pgUrl = "jdbc:postgresql://localhost:5432/analytics";
      var pgRows = DbHelper.query(
        pgUrl, "postgres", System.getenv("PG_PWD"),
        "SELECT id, order_no, amount FROM orders WHERE status = ? AND amount > ?",
        "PAID", 100
      );
  
- IO.println("从 PostgreSQL 查出 " + pgRows.size() + " 条订单：");
      for (var row : pgRows) {
        // 像操作字典一样直接取值
        IO.println("订单号: %s | 金额: %s".formatted(row.get("order_no"), row.get("amount")));
      }
  
- // 2. 一行代码导出为 CSV
      DbHelper.saveToCsv(pgRows, Path.of("paid_orders.csv"));
      IO.println("CSV 导出成功！");
  
- // 3. 同样的方法直接连 MySQL 查数据
      var mysqlUrl = "jdbc:mysql://localhost:3306/shop?useSSL=false&serverTimezone=UTC";
      var mysqlRows = DbHelper.query(mysqlUrl, "root", System.getenv("MYSQL_PWD"), "SELECT count(*) AS total FROM users");
      IO.println("MySQL 用户总数: " + mysqlRows.getFirst().get("total"));
    }
  
- Use code with caution.

-  

-  

- \3. 运行与复用技巧

- - **直接运行**：在终端执行 jbang export_report.java（Linux/macOS      执行 chmod +x export_report.java 后可直接 ./export_report.java 运行）。
  - **跨目录复用**：以后你在其他文件夹新建脚本（例如 ~/tools/daily_check.java）时，不需要复制 DbHelper.java，只需把主脚本顶部的引入路径改为绝对或相对路径即可：
                java
                //SOURCES /Users/你的用户名/my-scripts/DbHelper.java
                
            
                Use code with caution.
- **不带第三方库时的原生运行**：如果你的工具类只用到了 JDK 自带 API（没有用到 CSV      或外部数据库驱动），甚至不需要安装 JBang，直接运行 java export_report.java，JDK 25      也会自动编译同目录下的 DbHelper.java。
  
- 这套模板已经涵盖了日常数据库和 CSV 脚本的核心骨架。如果你在配置环境时遇到任何问题，或者还想了解如何给脚本添加**命令行参数解析（替代 Python 的** **argparse****）**，随时告诉我！

-  

-  

-  

-  

- **是否“直接用 Jar 更好”，完全取决于这个小工具的生命周期和分发需求：自己日常临时改写用源码脚本（****.java** **+ JBang）更好，固定下来长期高频运行或发给别人用则打包成 Fat Jar 更好。**

- 两者的核心差异与适用场景如下：

- \1. 源码脚本 vs 独立 Jar 包对比

- | **对比维度**         | **源码脚本（****.java** **+   JBang）**                      | **独立可运行 Jar（Fat Jar）**                                |
  | -------------------- | ------------------------------------------------------------ | ------------------------------------------------------------ |
  | **修改迭代速度**     | **极快**。打开文本编辑器改两行保存即生效，像 Python 一样所见即所得。 | **较慢**。每次改代码都要重新执行打包命令覆盖旧 Jar。         |
  | **冷启动速度**       | 首次运行或修改代码后有约   0.5~1 秒编译延迟；未修改时走缓存，接近原生速度。 | **极快**。字节码已预编译完成，JVM 直接加载执行，零编译开销。 |
  | **分发与部署**       | 依赖本地网络首次拉取   Maven 依赖，对方机器需装 JBang 或手动配依赖。 | **极佳**。只需把一个 .jar 文件拷贝到任何装有 JDK 25 的机器，运行 java -jar tool.jar 即可，支持完全离线环境。 |
  | **定时任务（Cron）** | 适合，但路径和缓存环境变量需配置稳妥。                       | **最稳妥**。生产级 Crontab 定时任务的标准姿势，不依赖外部缓存状态。 |

- \2. 最佳折中方案：用 JBang 平时当脚本写，成熟后一键导出 Jar

- 你完全不需要在“建 Maven/Gradle 重型工程打 Jar”和“纯写脚本”之间二选一。**JBang 自带了一键打包 Fat Jar 的能力**，让你享受“脚本的开发速度 + Jar 的运行分发体验”：

- - **开发阶段（改得频繁时）**：
                直接编辑 export_report.java，用 jbang export_report.java      随时测试，改完即跑，无需任何构建配置文件（不需要 pom.xml 或 build.gradle）。
  - **定型阶段（准备放进定时任务或发给同事）**：
                在终端执行一行命令：
                bash
                jbang export fatjar export_report.java
                
                Use code with caution.
                
                JBang 会自动把你的代码以及 //DEPS 里声明的 PostgreSQL 驱动、MySQL 驱动、Commons CSV      全部打包进一个独立的 export_report.jar 文件中。
  - **最终运行**：
                之后你甚至可以脱离 JBang，在任何机器上直接用原生 JDK 25 运行：
                bash
                java -jar export_report.jar
                
            
              Use code with caution.
  
- \3. 给你的落地建议

- - **90% 的个人日常小工具**：直接保留 .java 源码形式配合      JBang 运行即可。JBang 会把编译结果和下载的依赖缓存在 ~/.jbang/cache 中，只要你不改代码，第二次运行的速度和直接跑 Jar      几乎没有体感差异，同时保留了随时打开文本编辑器就能修改源码的灵活性。
  - **10% 的高频定时脚本或团队共享工具**：用 jbang export fatjar 导出成一个独立的 .jar，丢进服务器或定时任务里运行，省心且稳定。

- 你这些小工具主要是**自己本地按需手动执行**，还是准备部署到**服务器跑定时任务（如 Crontab）**？如果是部署到服务器，我可以帮你补充一套**免装 JBang 直接用原生** **java -cp** **管理本地 Jar 依赖**的纯离线目录方案。

-  

-  

- **对于个人小工具开发而言，绝大多数情况下不值得把公共类做成 Jar，直接保留** **.java** **源码形式复用是最高效的选择。** 只有在极少数特定条件下，打包成 Jar 才有意义。

- 以下是具体的评估维度与实操建议：

- \1. 为什么不建议做成 Jar（源码复用的优势）

- 在 JDK 25 + JBang 的工作流下，直接复用公共 .java 源码比维护 Jar 包顺手得多：

- - **零维护成本**：公共工具类（如数据库连接、CSV 读写、HTTP      封装）在写小工具的过程中经常需要顺手加个新方法或改个小逻辑。如果是源码，改完保存立刻生效；如果做成      Jar，每次改一行代码都要重新编译、打包、更新版本号，非常打断心流。
  - **编译缓存已解决性能问题**：无论是原生 JDK 25 还是      JBang，都会在底层自动缓存编译结果。只要公共源码没改，运行时不会重复编译，执行效率与直接引用 Jar 完全一致。
  - **排查报错更直观**：运行脚本报错时，直接引用源码能精准定位到具体的行号并随时打开修改，不需要反编译 Jar      包。

- \2. 什么时候才值得做成 Jar？

- 只有当你的公共代码满足以下**任意一条**时，才建议打包成 Jar：

- - **代码已经极度稳定且庞大**：比如你积累了十几个公共类，包含复杂的业务规则、自定义注解或大量资源文件，且半年都不会改动一次。
  - **需要分享给团队其他人使用**：你不想让同事看到或随意修改你的底层实现，只想给他们提供一个黑盒工具包，让他们在脚本里直接加一行      //DEPS 就能调用。
  - **纯原生 JDK 离线环境（不用 JBang）**：如果你在服务器上只用原生的 java -cp      libs/* script.java 运行脚本，把稳定的公共类打成一个 my-utils.jar 丢进 libs/      文件夹里，确实比每次在命令行里罗列一堆 .java 文件路径更整洁。

- \3. 个人工具库的最佳目录组织方式（推荐）

- 如果你想在多个不同目录的脚本中优雅地复用公共类，又不想折腾 Jar 打包，推荐建立一个统一的本地源码库目录：

- text

- ~/my-java-tools/
    ├── common/         <-- 存放所有公共 .java 源码（不写 package）
    │  ├── DbHelper.java    <-- PostgreSQL / MySQL 极简查询封装
    │  └── CsvHelper.java   <-- CSV 读写封装
    └── tasks/         <-- 存放各个独立的小工具脚本
      ├── sync_orders.java
      └── clean_logs.java
    
- Use code with caution.

-  

- - **使用 JBang 时**：在 tasks/ 下的任意脚本顶部加上      //SOURCES ../common/*.java（甚至支持直接引入远程 GitHub/GitLab 仓库里的      .java 文件 URL），即可一键引入整个 common 目录下的所有公共类。
  - **使用原生** **java** **命令时**：如果只用到了 JDK      自带 API，运行脚本时加上源码路径参数 java --source-path ~/my-java-tools/common      sync_orders.java 即可自动关联编译。

- 你目前手头有哪些**高频重复的逻辑**打算抽离成公共类（比如数据库连接池配置、特定格式的日期/正则处理，还是钉钉/飞书 Webhook 消息通知）？我可以帮你直接设计一个通用的 CommonUtil.java 骨架。

-  

-  

- 是的，jbang export 会自动把主脚本通过 //SOURCES 引入的所有公共 .java 文件一起编译并打包进最终的 Jar 中。

- 无论是生成可独立运行的 fatjar，还是只包含你自己代码的轻量 jar，JBang 都会自动追踪依赖图，不会漏掉任何关联的公共源码。

- \1. 打包包含的具体内容（按导出类型区分）

- 当你执行打包命令时，JBang 会自动处理主文件和公共 .java 文件：

- - **导出胖包（****jbang export      fatjar main.java****）**：
                生成一个独立的 main.jar。里面完整包含了：

- 1. 主脚本 main.java 编译后的 .class      文件。
  2. 所有通过 //SOURCES 引入的公共 .java 编译后的 .class      文件（哪怕它们位于上一级目录如 ../common/DbHelper.java）。
  3. 所有通过 //DEPS      声明的第三方库依赖（如 PostgreSQL/MySQL 驱动、Commons CSV 等）。

- - **结果**：这个 main.jar 是完全自包含的，拷贝到任何装有对应 JDK      的机器上，直接运行 java -jar main.jar 即可，不再依赖任何外部 .java 源码或外部 Jar。
  - **导出轻量包（****jbang export jar main.java****）**：
                同样会把主脚本和所有 //SOURCES 公共 .java 编译打进 main.jar，但不会把第三方      Maven 依赖塞进去（而是写进 MANIFEST.MF 的 Class-Path 中，适合配合 jbang      export portable 生成带有 libs/ 目录的便携包）。

- \2. 打包时的两个实用小细节

- - **支持通配符批量打包**：
                如果你的脚本顶部写的是 //SOURCES ../common/*.java，执行 jbang      export fatjar 时，JBang 会自动扫描该文件夹下所有匹配的 .java 文件，全部编译并打进      Jar 包。
  - **指定输出文件名**：
                默认生成的 Jar 名称与主脚本一致，如果你希望直接导出到特定部署目录并重命名，可以加上 -O 参数：
                bash
                jbang export fatjar -O /opt/tools/sync-db.jar      export_report.java
                
            
              Use code with caution.
  
- \3. 完整闭环工作流总结

- 这意味着你可以享受最完美的开发与部署体验：

- 1. **平时开发**：主脚本和公共      .java 全部以纯源码形式存放，用 jbang main.java      随时修改、秒级运行。
  2. **需要交付或上服务器定时任务时**：敲一行 jbang export fatjar main.java，JBang      自动把主代码、公共 .java、数据库驱动全部揉进一个 .jar 文件里，直接拿去部署即可。

- 你现在已经掌握了从单文件脚本、多文件公共类复用，到最终一键打包独立 Jar 的完整链路。需要我帮你测试或调整某个具体的脚本场景（比如**定时任务脚本的日志输出与异常处理模板**），还是你已经准备动手写第一个 JDK 25 工具了？

-  

-  

-  

-  

- 在 Vim、Zed、VS Code、Emacs 或 Micro 等轻量编辑器中，无需启动重型 IDE（如 IntelliJ IDEA），通过“配置轻量 LSP 补全”、“定义个人代码片段（Snippets）”以及“编写极简外观（Facade）封装类”三种方式，就能彻底解决 Java 方法名长、难记忆的问题。

- 针对你提到的几款编辑器和 Java 特性，以下是具体的落地方案：

-  

- 一、 编辑器级方案：零配置接入轻量 LSP（获得精确自动补全）

- 单文件 .java 脚本不需要构建 Maven/Gradle 工程，只需挂载 Eclipse 的轻量语言服务器（**jdtls**），编辑器就能识别 JDK 25 和 JBang 的 //DEPS 依赖，提供精确的 . 方法提示。

- - **Zed（最推荐，原生开箱即用）**：
                Zed 内置了 Java 支持，打开 .java 文件时会自动下载并启动 jdtls。如果你用了      JBang 写了 //DEPS，只需在终端执行一次 jbang export portable --fresh      你的脚本.java 或在 Zed 中配合 JBang 插件，就能实现带第三方库的丝滑补全。
  - **VS Code（轻量模式）**：
                不需要装庞大的 "Extension Pack for Java"，只安装 **Language Support for Java(TM) by Red Hat** 即可。同时安装 **JBang** 官方扩展，它会自动解析脚本顶部的 //DEPS 注释，让你敲 DbHelper. 或 CSVFormat.      时直接弹出准确的方法列表。
  - **Vim / Neovim & Emacs**：
  - **Neovim**：通过 nvim-lspconfig 启用 jdtls，配合 nvim-cmp 补全插件即可。
  - **Emacs**：使用内置的 eglot（Emacs 29+ 自带，零额外插件），打开 .java 文件后执行 M-x eglot，会自动连接系统安装的      jdtls 提供补全。
  - **Micro（超轻量终端编辑器）**：
                Micro 自身不带完整 LSP，但可以安装官方插件 lsp（执行 plugin install lsp），并在配置文件中将 java 指向本机的 jdtls 路径。

-  

- 二、 编辑器级方案：配置 Snippets（代码片段缩写）

- 在写脚本时，80% 的长方法名都集中在固定的几个高频动作上（比如读写文件、打印输出、格式化字符串）。在 Vim（UltiSnips/LuaSnip）、Zed 或 VS Code 中配置几个 2-3 个字母的缩写，按 Tab 键直接展开：

- | **输入缩写** | **按 Tab 展开后的完整 JDK 25 代码**                          | **说明**         |
  | ------------ | ------------------------------------------------------------ | ---------------- |
  | main         | void main()   throws Exception {\n $0\n}                     | 极简脚本入口     |
  | pr           | IO.println($0);                                              | 控制台打印       |
  | rf           | Files.readString(Path.of("$1"))$0                            | 读取整个文本文件 |
  | wf           | Files.writeString(Path.of("$1"),   $2);$0                    | 写入文本文件     |
  | fmt          | "$1".formatted($2)$0                                         | 字符串格式化     |
  | deps         | ///usr/bin/env   jbang "$0" "$@" ; exit $?\n//JAVA 25\n//DEPS $0 | JBang 脚本头部   |

-  

- 三、 代码级方案：给自己写一套“短名门面工具类”（最治本）

- Java 标准库的方法名长，是因为它追求企业级工程的严谨语义（比如 Files.readString(Path.of(...))）。但在你自己的 common/ 公共源码目录里，**你可以用自己习惯的短名字把它们重新包装一层**。

- 比如在 common/Kit.java 里定义一组像 Python 或 Shell 一样简短的静态方法：

- java

- import module java.sql;
    import org.apache.commons.csv.*;

- public class Kit {
      // 1. 极简打印与输入（比 IO.println 更短）
      public static void p(Object msg) { IO.println(msg); }
      public static String ask(String prompt) { return IO.readln(prompt); }
  
- // 2. 极简文件读写（免去每次敲 Path.of）
      public static String read(String file) throws IOException {
        return Files.readString(Path.of(file));
      }
      public static void write(String file, String content) throws IOException {
         Files.writeString(Path.of(file), content);
      }
      public static List<String> lines(String file) throws IOException {
        return Files.readAllLines(Path.of(file));
      }

- // 3. 极简正则提取
      public static String regex(String text, String pattern) {
        var m = Pattern.compile(pattern).matcher(text);
        return m.find() ? m.group(1) : null;
      }

- // 4. 极简 HTTP GET
      public static String get(String url) throws Exception {
        var client = java.net.http.HttpClient.newHttpClient();
        var req = java.net.http.HttpRequest.newBuilder(URI.create(url)).build();
        return client.send(req, java.net.http.HttpResponse.BodyHandlers.ofString()).body();
      }
    }

- Use code with caution.

-  

- **配合静态导入（****import static****），写脚本的最终体验：**

- 在你的日常脚本顶部加一行 import static Kit.*;，接下来写起代码来比 Python 还要短：

- java

- ///usr/bin/env jbang "$0" "$@" ; exit $?
    //JAVA 25
    //SOURCES ../common/Kit.java
    import static Kit.*;

- void main() throws Exception {
      var name = ask("请输入文件名: ");
      var content = read(name);
      var id = regex(content, "order_id=(\\d+)");
      
      p("提取到的订单号: " + id);
      write("result.txt", "ID=" + id);
    }
  
- Use code with caution.

-  

- 总结建议

- 1. **日常写代码**：在你的公共目录里维护一个      Kit.java（封装文件和 DB      的短名方法），配合 import static Kit.*;，彻底告别长方法名，哪怕在没有任何补全的      micro 或 nano 里盲打也毫无压力。
  2. **编辑器选择**：如果你日常主力是 **Zed** 或 **VS Code**，直接利用它们的轻量 LSP 配合 JBang 插件，就能在不启动重型 IDE      的前提下拥有 90% 的智能提示体验。

- 你平时最主力使用的是哪一款编辑器（比如 **Zed、Neovim 还是 VS Code**）？我可以为你提供该编辑器对接 **JBang + JDK 25 自动补全**的具体配置步骤。

-  

- 