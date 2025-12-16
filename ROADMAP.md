# go-xml Roadmap

This document details missing functionalities and planned improvements for the project, ordered by impact and complexity.

## 🚀 High Priority (High Impact / Low-Medium Complexity)

### 1. JSON Export Utility (`xml.ToJSON`)
**Impact: High** | **Complexity: Low**

Many users parse XML solely to convert it to JSON for other services.
- **Need**: A helper method `xml.ToJSON(r io.Reader) ([]byte, error)` that pipelines the parsing and Map-to-JSON marshalling in one optimized step.

### 2. Improved Error Reporting
**Impact: High** | **Complexity: Low**

Validation and parsing errors are generic ("parsing error").
- **Need**: Expose line and column numbers where the error occurred in the `xml.Error` type, essential for debugging large or malformed files.

### 3. Wildcard Support in Query
**Impact: High** | **Complexity: Medium**

Navigating dynamic lists where keys are unknown is currently difficult (requires manual iteration).
- **Need**: Support `*` wildcard in `Query` paths, e.g., `invoice/items/*/sku` to get all SKUs regardless of the wrapper tag.

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

### 6. Full XPath 1.0 Support
**Impact: Medium** | **Complexity: High**

Current `Query` system is sufficient for 90% of cases. Full XPath 1.0 implies supporting axes (`following-sibling`, `ancestor`) and functions (`count()`, `contains()`).
- **Need**: Wait for user demand before implementing a full engine.

### 7. XSD Validation (Schema)
**Impact: Medium** | **Complexity: Very High**

Validating against a standard XSD file is the industry gold standard but extremely complex to implement.
- **Need**: Evaluate C wrappers (libxml2) if strict validation is critical.

### 8. Hybrid Support (Marshal/Unmarshal)
**Impact: Low** | **Complexity: Medium**

Allow using `MapXML` as an intermediate step to then decode into a standard Go struct.
