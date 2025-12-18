# go-xml Advanced Querying Tutorial

`go-xml` provides a powerful Query Engine inspired by XPath but optimized for Go's `map[string]any` structure. This tutorial covers advanced querying techniques, including Wildcards, XPath-Lite features, and Custom Functions.

## 1. Wildcards (`*`)

Use `*` to iterate over all "child" nodes of a map, excluding attributes (`@...`) and metadata keys (`#...`). This is perfect for dynamic maps where keys are IDs or unknown names.

### Scenario: Dynamic Item Keys (Invoice)
```xml
<invoice>
    <items>
        <box_001> <sku>A1</sku> </box_001>
        <bag_002> <sku>B2</sku> </bag_002>
    </items>
</invoice>
```

**Query**:
```go
// Get ALL SKUs, regardless of the container key (box_001, bag_002)
skus, _ := xml.QueryAll(m, "invoice/items/*/sku")
// Output: ["A1", "B2"]
```

---

## 2. XPath-Lite Features

We implement the "80/20" rule of XPath: the most useful features without the overhead of a full engine.

### A. Deep Search (`//`)
Recursively searches the entire data structure for keys matching the name.

**Query**:
```go
// Find ALL "error" nodes anywhere in the document
errs, _ := xml.QueryAll(m, "//error")
```

### B. Filter Operators
Support for standard comparison operators: `=`, `!=`, `>`, `<`, `>=`, `<=`.

**Query**:
```go
// Find books cheaper than $10
cheapBooks, _ := xml.QueryAll(m, "library/book[price<10]/title")

// Find active users (status is not inactive)
activeUsers, _ := xml.QueryAll(m, "users/user[status!=inactive]")
```

### C. Filter Functions
Built-in string functions inside filters:
-   `contains(key, 'value')`
-   `starts-with(key, 'value')`

**Query**:
```go
// Find users with gmail addresses
gmailUsers, _ := xml.QueryAll(m, "users/user[contains(email, '@gmail.com')]")
```

### D. Aggregation (`#count`)
Returns the number of children in a list or map.

**Query**:
```go
// Count number of books
count, _ := xml.Query(m, "library/book/#count")
```

---

## 3. Custom Query Functions (`func:...`)

If standard tools aren't enough, you can register custom Go functions to filter keys.

### Step 1: Register your Function
```go
import (
    "strings"
    "github.com/arturoeanton/go-xml/xml"
)

func init() {
    // Register a function that returns true for keys starting with "iphone"
    xml.RegisterQueryFunction("isIphone", func(key string) bool {
        return strings.HasPrefix(key, "iphone")
    })
}
```

### Step 2: Use in Query
Use the `func:name` syntax in your path.

**Query**:
```go
// Select only keys passing "isIphone", then get their "model"
models, _ := xml.QueryAll(m, "products/func:isIphone/model")
```

### Built-in Functions
We provide 15 built-in utility functions out of the box (see `xml/features_query.go`):
-   `isNumeric`: Keys like "123".
-   `isUUID`: Keys like "550e8400...".
-   `isSnakeCase`, `isCamelCase`: For structural validation.
-   `hasDigits`, `isAlpha`, etc.

**Example**:
```go
// Iterate only over numeric keys (ignoring metadata or string keys)
ids, _ := xml.QueryAll(m, "data/func:isNumeric/id")
```
