package ide

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var javaClassName = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

// GenerateToolScript creates a single-file CLI starter with database, JSON,
// and HTTP client examples.
func GenerateToolScript(path string) error {
	if !strings.EqualFold(filepath.Ext(path), ".java") {
		return fmt.Errorf("tool template output must have a .java extension")
	}
	className := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if !javaClassName.MatchString(className) {
		return fmt.Errorf("%q is not a valid Java class name", className)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}
	if _, err := os.Stat(absPath); err == nil {
		return fmt.Errorf("file already exists: %s", absPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect output path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	content := strings.ReplaceAll(toolScriptTemplate, "{{CLASS}}", className)
	file, err := os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create tool script: %w", err)
	}
	if _, err := file.WriteString(content); err != nil {
		file.Close()
		os.Remove(absPath)
		return fmt.Errorf("write tool script: %w", err)
	}
	if err := file.Close(); err != nil {
		os.Remove(absPath)
		return fmt.Errorf("close tool script: %w", err)
	}
	fmt.Printf("Generated Java tool template: %s\n", absPath)
	return nil
}

const toolScriptTemplate = `//JDK 17
//DEPS info.picocli:picocli:4.7.6
//DEPS org.xerial:sqlite-jdbc:3.47.1.0
//DEPS com.google.code.gson:gson:2.11.0

import com.google.gson.Gson;
import picocli.CommandLine;
import picocli.CommandLine.Command;
import picocli.CommandLine.Option;
import picocli.CommandLine.Parameters;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.sql.DriverManager;
import java.util.Map;
import java.util.concurrent.Callable;

@Command(name = "{{CLASS}}", mixinStandardHelpOptions = true,
        description = "Command-line tool starter")
public class {{CLASS}} implements Callable<Integer> {
    @Option(names = "--db-url", defaultValue = "jdbc:sqlite:tool.db",
            description = "JDBC URL (defaults to a local SQLite database)")
    String dbUrl;

    @Option(names = "--db-user", description = "Database user (or set DB_USER)")
    String dbUser = System.getenv("DB_USER");

    @Option(names = "--db-password", description = "Database password (or set DB_PASSWORD)")
    String dbPassword = System.getenv("DB_PASSWORD");

    @Option(names = "--http", description = "Optional URL to fetch with the JDK HttpClient")
    URI httpUrl;

    @Parameters(index = "0", arity = "0..1", description = "Message to store and print")
    String message = "hello";

    @Override
    public Integer call() throws Exception {
        var json = new Gson().toJson(Map.of("message", message));
        System.out.println("JSON: " + json);

        // JDBC works with any database that provides a compatible driver.
        // SQLite is included as a zero-configuration local example.
        var connection = dbUser == null && dbPassword == null
                ? DriverManager.getConnection(dbUrl)
                : DriverManager.getConnection(dbUrl, dbUser, dbPassword);
        try (connection;
             var statement = connection.createStatement()) {
            statement.executeUpdate("CREATE TABLE IF NOT EXISTS messages (text TEXT NOT NULL)");
            try (var insert = connection.prepareStatement("INSERT INTO messages(text) VALUES (?)")) {
                insert.setString(1, message);
                insert.executeUpdate();
            }
        }

        if (httpUrl != null) {
            var request = HttpRequest.newBuilder(httpUrl).GET().build();
            var response = HttpClient.newHttpClient().send(
                    request, HttpResponse.BodyHandlers.ofString());
            System.out.printf("HTTP %d (%d chars)%n",
                    response.statusCode(), response.body().length());
        }
        return 0;
    }

    public static void main(String[] args) {
        System.exit(new CommandLine(new {{CLASS}}()).execute(args));
    }
}
`
