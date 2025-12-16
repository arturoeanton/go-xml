# go-xml: The Enterprise Single-File XML Parser
> Stop writing structs for dynamic XML.

`go-xml` (v1.0) is a robust, schemaless XML parser and serializer for Go. It is designed to handle complex, dynamic, or "dirty" XML without defining rigid Go structs for every single tag. It offers a complete solution ranging from in-memory dynamic maps to high-performance streaming for gigabyte-sized files.

## 🚀 Key Features

*   **Schemaless Parsing**: content is parsed into `map[string]any`, allowing you to traverse unknown XML structures dynamically.
*   **Streaming Support**:
    *   **Decoder**: Process multi-gigabyte files with constant memory usage using Generic `Stream[T]`.
    *   **Encoder**: Write XML directly to an `io.Writer` for high-efficiency pipeline processing.
*   **Robust & Lenient**: Capable of reading "soup" HTML/XML (unclosed tags like `<br>`, `<meta>`) with lenient mode. Use `EnableExperimental()` to activate Soup Mode (automatic sanitization of `<script>` tags, lowercase normalization, and HTML void tag support).
*   **Advanced Querying**: XPath-style deep querying utilities (e.g., `users/user[0]/name`).
*   **Validation Engine**: Define business rules for your data (Regex, Range, Enum, Type).
*   **Attributes as Data**: Attributes are treated as first-class citizens, accessible via a simple `@` prefix convention.
*   **Namespaces Helpers**: Register aliases to work with short keys instead of full URLs.
*   **Value Hooks**: Define custom logic to transform strings into native Go types (Dates, Enums, etc.) during parsing.
*   **Built-in CLI**: Includes a terminal tool for quick XML querying.
*   **Legacy Charsets**: Built-in support for ISO-8859-1 and Windows-1252 parsing via `EnableLegacyCharsets()`.

## 📦 Installation

```bash
go get github.com/arturoeanton/go-xml
```

## 📖 Usage Guide

### 1. Basic Parsing (MapXML)
The core function `MapXML` reads data into a dynamic map.

```go
package main

import (
    "fmt"
    "strings"
    "github.com/arturoeanton/go-xml/xml" 
)

func main() {
    xmlData := `<library><book id="1">The Little Prince</book></library>`

    // Parse without defining structs
    m, err := xml.MapXML(strings.NewReader(xmlData))
    if err != nil {
        panic(err)
    }

    // Access data manually
    lib := m["library"].(map[string]any)
    book := lib["book"].(map[string]any)

    fmt.Println("Title:", book["#text"]) // "The Little Prince"
    fmt.Println("ID:", book["@id"])      // "1" (Attributes use '@' prefix)
}
```

### 2. Handling JSON Arrays (ForceArray)
XML is ambiguous regarding arrays (a single child vs. a list of one child). Use `ForceArray` to ensure specific tags are always treated as slices `[]any`.

```go
// <library><book>One</book></library>
m, _ := xml.MapXML(r, xml.ForceArray("book"))

// Now 'book' is guaranteed to be []any, even if there is only one book.
books := m["library"].(map[string]any)["book"].([]any)
```

### 3. Namespaces
Simplify keys mapped from XML with namespaces by registering aliases.

```go
// <root xmlns:h="http://w3.org/html"><h:table>Data</h:table></root>
m, _ := xml.MapXML(r, xml.RegisterNamespace("html", "http://w3.org/html"))

// Access as "html:table" instead of the full URL
val, _ := xml.Query(m, "root/html:table/#text")
```

### 4. Hooks & Type Inference
Automatically convert strings to Go native types or apply custom logic.

```go
// <log><date>2025-12-31</date><count>99</count></log>

// 1. Custom Hook for Dates
dateHook := func(s string) any {
    t, _ := time.Parse("2006-01-02", s)
    return t
}

m, _ := xml.MapXML(r, 
    xml.WithValueHook("date", dateHook),
    xml.EnableExperimental(), // Automatically infers "99" as int
)

dateVal, _ := xml.Query(m, "log/date") // Returns time.Time
```

### 5. Start Streaming (Large Files)
For huge datasets, avoid loading everything into memory.

#### Streaming Decoder (Generics)
Use `Stream[T]` to iterate element by element with strong typing for the specific nodes you need.

```go
type Order struct {
    ID    int     `xml:"id"`
    Total float64 `xml:"total"`
}

func main() {
    file, _ := os.Open("huge_orders.xml")
    defer file.Close()

    // Stream <Order> elements one by one
    stream := xml.NewStream[Order](file, "Order")

    for order := range stream.Iter() {
        fmt.Printf("Processing Order %d: $%.2f\n", order.ID, order.Total)
    }

    // Or with Context for cancellation/timeout:
    // ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    // defer cancel()
    // for order := range stream.IterWithContext(ctx) { ... }
}
```

#### Streaming Encoder
Write XML directly to an `io.Writer` (like `http.ResponseWriter` or `os.File`) efficiently.

```go
data := map[string]any{
    "feed": map[string]any{
        "@version": "2.0",
        "title":    "Tech Blog",
    },
}

// Writes directly to stdout with indentation
xml.NewEncoder(os.Stdout, xml.WithPrettyPrint()).Encode(data)
```

### 6. Validation Rules
Validate dynamic data against business rules without structs.

```go
rules := []xml.Rule{
    {Path: "user/age",  Type: "int",    Min: 18},
    {Path: "user/role", Type: "string", Enum: []string{"admin", "user"}},
    {Path: "user/email", Type: "string", Regex: `^.+@.+\..+$`},
}

errors := xml.Validate(data, rules)
if len(errors) > 0 {
    fmt.Println("Validation failed:", errors)
}
```

### 7. Legacy Charsets (ISO-8859-1 / Windows-1252)
The parser automatically handles UTF-8. For legacy systems (banking, government) sending ISO-8859-1 or Windows-1252, use `EnableLegacyCharsets()`.

```go
// Header says encoding="ISO-8859-1"
m, err := xml.MapXML(r, xml.EnableLegacyCharsets())
// The parser automatically uses the correct charset reader
```

## 🛠 CLI Tool
You can use the `main.go` as a standalone CLI tool to query XML files from the terminal.

```bash
# Query a value from an XML file
cat data.xml | go run main.go query "users/user[0]/name"
```

## ⚙️ Implementation Details

### Architecture
The library is designed as a **Single-File** solution (conceptually) to minimize dependency hell, though organized internally.
1.  **Parser Core**: Implements a stack-based state machine processing `xml.Token`. It normalizes XML quirks into a consistent JSON-like map structure.
2.  **Type Inference**: If enabled (`EnableExperimental`), it automatically detects numbers and booleans (e.g., "123" becomes `int(123)` instead of string).

### Data Structure Mapping
The internal representation follows these conventions to map XML to `map[string]any`:

*   **Elements**: Become dictionary keys (`<tag>` -> `"tag"`).
*   **Attributes**: Become keys prefixed with `@` (`id="1"` -> `"@id": "1"`).
*   **Text Content**: Stored in the special key `"#text"`.
*   **Comments**: Stored in `"#comments"` (list of strings).
*   **CDATA**: Stored in `"#cdata"`.
