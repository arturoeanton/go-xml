# go-xml Roadmap

This document details missing functionalities and planned improvements for the project, ordered by impact and complexity.

## 🚀 High Priority (High Impact / Low-Medium Complexity)



### 3. Wildcard Support in Query [COMPLETED]
**Impact: COMPLETED** | **Complexity: COMPLETED**

*Implemented in v1.1*: Supports `*` wildcard in `Query` paths, e.g., `invoice/items/*/sku`.

## 🔮 Medium Priority (Strategic Features)

### 4. Raw Node Extraction (Canonicalization)
**Impact: Medium/High** | **Complexity: Medium**

Enterprise users (banking, crypto) often need the *unaltered* source string of a specific node to verify digital signatures (HMAC/RSA).
- **Need**: Mechanism to extract the raw bytes of a node (e.g., `<signedInfo>...</signedInfo>`) during parsing.

### 5. Struct Generation (CLI)
**Impact: Low** | **Complexity: Medium**

Although the philosophy is "no structs", sometimes migration or interoperability requires them.
- **Need**: A CLI command (`go run main.go gen-struct data.xml`) that infers and generates Go struct code based on a sample XML.

## 🧊 Low Priority / Future (High Complexity / Niche)

### 6. Full XPath 1.0 Support [PARTIALLY COMPLETED]
**Impact: Medium** | **Complexity: High**

*Update v1.1*: Implemented "XPath-Lite" covering common use-cases:
- Deep Search (`//node`)
- Operators (`>`, `<`, `!=`) inside filters.
- Functions (`contains()`, `starts-with()`).
- Aggregation (`#count`).
- Wildcards (`*`).
- Custom Functions registry (`items/func:myFunc/id`).

Full XPath axes (`following-sibling`, `ancestor`) are deferred until demanded.

### 7. XSD Validation (Schema)
**Impact: Medium** | **Complexity: Very High**

Validating against a standard XSD file is the industry gold standard but extremely complex to implement.
- **Need**: Evaluate C wrappers (libxml2) if strict validation is critical.

### 8. Hybrid Support (Marshal/Unmarshal)
**Impact: Low** | **Complexity: Medium**

Allow using `MapXML` as an intermediate step to then decode into a standard Go struct.
