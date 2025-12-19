# go-xml: The Enterprise Deterministic XML Parser (v2.0)
> **v2.0 Update**: Now powered by `OrderedMap` for 100% determinism and order preservation.

`go-xml` is a robust, schemaless XML parser and serializer for Go. Unlike standard parsers that force you to define structs or lose element order using standard maps, `go-xml` preserves the exact document structure attributes, and order using a custom `OrderedMap`.

It is designed for **Enterprise Integration** (Banking, Government, SOAP) where order matters (e.g., XSD validation, SOAP signatures).

## 🚀 Key Features

*   **Deterministic Parsing**: Parses XML into `*OrderedMap`, preserving insertion order of elements and attributes.
*   **OrderedMap API**: Fluent API for deep access and manipulation (`m.Set("Body/Auth", val)`).
*   **Streaming Support**:
    *   **Decoder**: Process multi-gigabyte files with constant memory usage using Generic `Stream[T]`.
    *   **Encoder**: Write XML directly to `io.Writer` from `*OrderedMap` (strict XSD compliance).
*   **Robust & Lenient**: "Soup Mode" for dirty HTML (`<br>`, `<meta>`, unclosed tags).
*   **Advanced Querying**: XPath-like deep querying (`Query(m, "users/user[0]/name")`).
*   **Validation Engine**: Define business rules (Regex, Range, Enum, Type).
*   **CLI Tool (r2xml)**: Built-in Swiss Army Knife for XML (Format, JSON, CSV, SOAP).
*   **Dynamic SOAP Client**: Call SOAP 1.1 services without generating code. Supports **mTLS** and **WS-Security**.
*   **Digital Signatures**: Helper for **XML-DSig** and **XAdES-BES** signing.

## 📦 Installation

```bash
go get github.com/arturoeanton/go-xml
```

## 📖 Usage Guide

### 1. Basic Parsing (OrderedMap)
The core function `MapXML` returns an `*OrderedMap`.

```go
package main

import (
    "fmt"
    "strings"
    "github.com/arturoeanton/go-xml/xml" 
)

func main() {
    xmlData := `<library><book id="1">The Little Prince</book></library>`

    // 1. Parse into OrderedMap
    m, _ := xml.MapXML(strings.NewReader(xmlData))

    // 2. Safe Typed Access (No panics)
    title := m.String("library/book/#text") // "The Little Prince"
    id := m.String("library/book/@id")      // "1" (Attributes use '@')

    fmt.Printf("Book: %s (ID: %s)\n", title, id)
}
```

### 2. Creating & Modifying XML
Creating XML structures is fluent and readable.

```go
m := xml.NewMap()

// Deep Fluent Setters
m.Set("Order/ID", "1001")
m.Set("Order/Customer/Name", "Alice")
m.Set("Order/Customer/@id", "C55") // Attribute

// Serialize (Deterministic Output)
s, _ := xml.Marshal(m, xml.WithPrettyPrint())
fmt.Println(s)
// Output:
// <Order>
//   <ID>1001</ID>
//   <Customer id="C55">
//     <Name>Alice</Name>
//   </Customer>
// </Order>
```

### 3. CLI Tool (r2xml)
The library acts as a standalone CLI tool.

```bash
# Pretty Print / Format
go run main.go fmt dirty.xml

# Convert to JSON
go run main.go json data.xml

# Convert List to CSV (Flatten)
go run main.go csv orders.xml --path="orders/order" > report.csv

# Query (XPath-lite)
go run main.go query data.xml "users/user[id=1]/name"

# Execute SOAP Request from Config
go run main.go soap request.json
```

### 4. Dynamic SOAP Client (with mTLS)
Consume SOAP services dynamically.

```go
// 1. Configure
client := xml.NewSoapClient(
    "https://secure-bank.com/service", 
    "http://tempuri.org/",
    xml.WithClientCertificate("client.crt", "client.key"), // mTLS Support
    xml.WithBasicAuth("user", "pass"),
)

// 2. Call (Payload matches key order)
payload := xml.NewMap()
payload.Put("FromAccount", "123")
payload.Put("ToAccount", "456")
payload.Put("Amount", 100.50)

resp, err := client.Call("TransferFunds", payload)

// 3. Check Result
status := resp.String("Envelope/Body/TransferResponse/Status")
```

### 5. Advanced Features

#### Handling Arrays
Use `ForceArray` to ensure specific tags are always lists, even if single.
```go
m, _ := xml.MapXML(r, xml.ForceArray("item"))
items := m.List("order/item") // Always []*OrderedMap
```

#### Node Mutation
Refactor your data easily.
```go
m.Rename("legacy_key", "new_key")
m.Move("temp/data", "final/destination")
```

#### Streaming (Large Files)
Process GBs of data with constant memory.
```go
stream := xml.NewStream[Order](file, "Order")
for order := range stream.Iter() {
    process(order)
}
```

## ⚙️ Architecture: OrderedMap

In v2.0, we replaced `map[string]any` with `*OrderedMap`.
- **Why?** Go's native map randomizes iteration order. XML (XSD) and SOAP often require strict element ordering.
- **How?** `OrderedMap` maintains a slice of keys `[]string` alongside the map.
- **Benefit**: Read XML -> Modify -> Write XML = **Identical Structure**.

## 📄 License
MIT
