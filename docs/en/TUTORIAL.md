# go-xml v2.0 - Comprehensive Tutorial

This tutorial covers the core concepts of `go-xml`, a library designed for **Ordered, Deterministic XML** processing in Go.

## 1. Why `OrderedMap`?
Standard Go `map[string]any` is random. This sucks for XML.
- **XSD Validation**: Requires strict order (`<User><Name><Age>` != `<User><Age><Name>`).
- **Signatures**: Calculate hash based on exact byte sequence.
- **Diffing**: Random order makes git diffs useless.

`go-xml` v2.0 introduces `*OrderedMap`. It behaves like a map but remembers insertion order.

## 2. Reading XML
Use `MapXML` to get an `*OrderedMap`.

```go
xmlStr := `<Product id="p1"><Name>Box</Name><Price>10</Price></Product>`
m, _ := xml.MapXML(strings.NewReader(xmlStr))

// Access Attributes
fmt.Println(m.String("Product/@id")) // "p1"

// Access Elements
fmt.Println(m.String("Product/Name")) // "Box"
```

## 3. Writing XML (Fluent API)
Create deeply nested structures in one line.

```go
m := xml.NewMap()

// Auto-creates hierarchy: Envelope -> Body -> Login
m.Set("Envelope/Body/Login/User", "admin")
m.Set("Envelope/Body/Login/Pass", "secret")
m.Set("Envelope/Body/Login/@type", "secure")

// Encode with indentation
s, _ := xml.Marshal(m, xml.WithPrettyPrint())
fmt.Println(s)
```

**Output (Deterministic):**
```xml
<Envelope>
  <Body>
    <Login type="secure">
      <User>admin</User>
      <Pass>secret</Pass>
    </Login>
  </Body>
</Envelope>
```

## 4. Arrays & Lists
XML is ambiguous: `<Item>` could be a single object or a list of one.
Use `ForceArray` to resolve this.

```go
m, _ := xml.MapXML(r, xml.ForceArray("Item"))

// Now "Item" is ALWAYS a list []*OrderedMap
items := m.List("Order/Item")
for _, item := range items {
    fmt.Println(item.String("Name"))
}
```

## 5. CLI Power Tools
You can use the library as a binary tool `r2xml`.

**Pretty Print:**
```bash
go run main.go fmt ugly.xml
```

**Convert to CSV:**
```bash
go run main.go csv data.xml --path="Order/Item" > items.csv
```

**JSON Conversion:**
```bash
go run main.go json data.xml
```
