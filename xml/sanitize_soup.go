package xml

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
)

func sanitizeSoup(r io.Reader) io.Reader {
	data, err := io.ReadAll(r)
	if err != nil {
		return r
	}

	rawTags := []string{"script", "style", "code", "pre", "textarea"}

	for _, tag := range rawTags {
		// Regex: Busca <tag ...> CONTENIDO </tag>
		// (?is) hace que el punto (.) coincida con saltos de línea y sea case-insensitive
		pattern := fmt.Sprintf(`(?is)(<%s(?:>|\s[^>]*>))(.*?)(</%s>)`, tag, tag)
		re := regexp.MustCompile(pattern)

		data = re.ReplaceAllFunc(data, func(match []byte) []byte {
			parts := re.FindSubmatch(match)
			if len(parts) < 4 {
				return match
			}

			openTag := parts[1]
			content := parts[2]
			closeTag := parts[3]

			// TRUCO: Si encontramos un "]]>" dentro del JS, lo partimos.
			// ]]> se convierte en: ]]]]><![CDATA[>
			// Esto cierra el CDATA actual, mete un ">" y abre uno nuevo.
			escapedContent := bytes.ReplaceAll(content, []byte("]]>"), []byte("]]]]><![CDATA[>"))

			var buf bytes.Buffer
			buf.Write(openTag)
			buf.WriteString("<![CDATA[")
			buf.Write(escapedContent)
			buf.WriteString("]]>")
			buf.Write(closeTag)

			return buf.Bytes()
		})
	}

	return bytes.NewReader(data)
}
