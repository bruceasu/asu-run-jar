package ide

import (
	"bytes"
	"encoding/xml"
)

func escapeXML(value string) string {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(value))
	return escaped.String()
}
