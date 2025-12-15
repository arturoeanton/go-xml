# go-xml Roadmap

This document details missing functionalities and planned improvements for the project, ordered by impact and complexity.

## 🚀 High Priority

### 1. Charset Support (ISO-8859-1, Windows-1252)
**Impact: High** | **Complexity: Medium**

Currently, the parser assumes UTF-8 by default. Many legacy systems (banking, government) send XML in encodings such as ISO-8859-1.
- **Need**: Implement `CharsetReader` in the decoder configuration to automatically transform input to UTF-8.

### 2. Context-Aware Streaming
**Impact: High** | **Complexity: Low**

Current streaming (`Stream[T]`) runs in a separate goroutine until the file ends. There is no way to cancel the process if the HTTP request is cut off or times out.
- **Need**: Add support for `context.Context` in `NewStream` or `Iter`, allowing the goroutine to be cancelled and resources released immediately.

### 3. Improved Error Reporting
**Impact: Medium** | **Complexity: Low**

Validation and parsing errors are generic.
- **Need**: Expose line and column numbers where the error occurred, especially useful for large or malformed files.

## 🔮 Medium Priority

### 4. Full XPath 1.0 Support
**Impact: Medium/High** | **Complexity: High**

The current Query system (`users/user[0]/name`) is powerful but limited. It does not support complex axes (`following-sibling`, `ancestor`) or XPath functions (`count()`, `contains()`).
- **Need**: Evaluate whether to implement a real XPath engine or continue extending the current mini-language.

### 5. Struct Generation (CLI)
**Impact: Low** | **Complexity: Medium**

Although the philosophy is "no structs", sometimes migration or interoperability with systems that use them is needed.
- **Need**: A CLI command (`go run main.go gen-struct data.xml`) that infers and generates Go struct code based on a sample XML.

## 🧊 Low Priority / Future

### 6. XSD Validation (Schema)
**Impact: Medium** | **Complexity: Very High**

Validating against a standard XSD file is extremely complex to implement from scratch, but it is the industry gold standard.
- **Need**: Integrate partial support or C wrappers for strict validation if required by the user.

### 7. Hybrid Support (Marshal/Unmarshal)
**Impact: Low** | **Complexity: Medium**

Allow using `MapXML` as an intermediate step to then decode into a standard Go struct, for users who want the best of both worlds.
