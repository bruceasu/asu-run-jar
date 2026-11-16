package ide

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
)

func configureMavenPom(pom, mainClass, javaVersion string) (string, error) {
	pom = setMavenProperty(pom, "maven.compiler.source", javaVersion)
	pom = setMavenProperty(pom, "maven.compiler.target", javaVersion)
	if javaVersion == "8" {
		pom = removeMavenProperty(pom, "maven.compiler.release")
	} else {
		pom = setMavenProperty(pom, "maven.compiler.release", javaVersion)
	}

	plugins := activeBuildPlugins(pom)
	var additions []string
	if !plugins["maven-jar-plugin"] {
		additions = append(additions, fmt.Sprintf(`<plugin>
      <groupId>org.apache.maven.plugins</groupId>
      <artifactId>maven-jar-plugin</artifactId>
      <version>3.4.2</version>
      <configuration>
        <archive><manifest><mainClass>%s</mainClass></manifest></archive>
      </configuration>
    </plugin>`, escapeXML(mainClass)))
	}
	if !plugins["exec-maven-plugin"] {
		additions = append(additions, fmt.Sprintf(`<plugin>
      <groupId>org.codehaus.mojo</groupId>
      <artifactId>exec-maven-plugin</artifactId>
      <version>3.5.0</version>
      <configuration><mainClass>%s</mainClass></configuration>
    </plugin>`, escapeXML(mainClass)))
	}
	if len(additions) == 0 {
		return pom, nil
	}
	return insertMavenBuildPlugins(pom, strings.Join(additions, "\n"))
}

func setMavenProperty(pom, name, value string) string {
	openTag, closeTag := "<"+name+">", "</"+name+">"
	if strings.Contains(pom, openTag) {
		pattern := regexp.MustCompile(`(?s)<` + regexp.QuoteMeta(name) + `>.*?</` + regexp.QuoteMeta(name) + `>`)
		return pattern.ReplaceAllString(pom, openTag+escapeXML(value)+closeTag)
	}
	if strings.Contains(pom, "</properties>") {
		return strings.Replace(pom, "</properties>", "  "+openTag+escapeXML(value)+closeTag+"\n</properties>", 1)
	}
	block := "<properties>\n  " + openTag + escapeXML(value) + closeTag + "\n</properties>\n"
	return strings.Replace(pom, "</project>", block+"</project>", 1)
}

func removeMavenProperty(pom, name string) string {
	pattern := regexp.MustCompile(`(?s)\s*<` + regexp.QuoteMeta(name) + `>.*?</` + regexp.QuoteMeta(name) + `>`)
	return pattern.ReplaceAllString(pom, "")
}

func activeBuildPlugins(pom string) map[string]bool {
	plugins := make(map[string]bool)
	decoder := xml.NewDecoder(strings.NewReader(pom))
	var stack []string
	var currentPluginID strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch value := token.(type) {
		case xml.StartElement:
			stack = append(stack, value.Name.Local)
			if equalXMLPath(stack, "project", "build", "plugins", "plugin", "artifactId") {
				currentPluginID.Reset()
			}
		case xml.CharData:
			if equalXMLPath(stack, "project", "build", "plugins", "plugin", "artifactId") {
				currentPluginID.Write([]byte(value))
			}
		case xml.EndElement:
			if equalXMLPath(stack, "project", "build", "plugins", "plugin", "artifactId") {
				plugins[strings.TrimSpace(currentPluginID.String())] = true
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	return plugins
}

func insertMavenBuildPlugins(pom, pluginsXML string) (string, error) {
	data := []byte(pom)
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var stack []string
	pluginsClose, buildClose, projectClose := -1, -1, -1
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch value := token.(type) {
		case xml.StartElement:
			stack = append(stack, value.Name.Local)
		case xml.EndElement:
			endOffset := int(decoder.InputOffset())
			if equalXMLPath(stack, "project", "build", "plugins") {
				pluginsClose = xmlClosingTagStart(data, endOffset)
			} else if equalXMLPath(stack, "project", "build") {
				buildClose = xmlClosingTagStart(data, endOffset)
			} else if equalXMLPath(stack, "project") {
				projectClose = xmlClosingTagStart(data, endOffset)
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}

	if pluginsClose >= 0 {
		return string(data[:pluginsClose]) + "\n    " + strings.ReplaceAll(pluginsXML, "\n", "\n    ") + "\n  " + string(data[pluginsClose:]), nil
	}
	if buildClose >= 0 {
		block := "\n    <plugins>\n      " + strings.ReplaceAll(pluginsXML, "\n", "\n      ") + "\n    </plugins>\n  "
		return string(data[:buildClose]) + block + string(data[buildClose:]), nil
	}
	if projectClose >= 0 {
		block := "\n  <build>\n    <plugins>\n      " + strings.ReplaceAll(pluginsXML, "\n", "\n      ") + "\n    </plugins>\n  </build>\n"
		return string(data[:projectClose]) + block + string(data[projectClose:]), nil
	}
	return "", fmt.Errorf("could not locate the project element in generated pom.xml")
}

func xmlClosingTagStart(data []byte, endOffset int) int {
	if endOffset < 0 || endOffset > len(data) {
		return -1
	}
	start := bytes.LastIndex(data[:endOffset], []byte("</"))
	return start
}

func equalXMLPath(path []string, expected ...string) bool {
	if len(path) != len(expected) {
		return false
	}
	for i := range expected {
		if path[i] != expected[i] {
			return false
		}
	}
	return true
}
