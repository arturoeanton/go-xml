# Roadmap

## ✅ v1.0 (Completed)
- [x] Basic parsing to `map[string]any`.
- [x] Streaming Decoder/Encoder.
- [x] XPath-lite Query engine.

## ✅ v2.0 (Stable Release)
- [x] **OrderedMap Implementation**: Deterministic order for keys and attributes.
- [x] **Strict Serializer**: Output matches input exactly (Roundtrip 100%).
- [x] **CLI Tools**: `fmt`, `csv`, `json`, `soap` commands.
- [x] **mTLS Support**: Client Certificates for SOAP.
- [x] **CSV Export**: Flatten XML lists to CSV.
- [x] **Digital Signatures**: XML-DSig and XAdES-BES signing with real Exclusive XML Canonicalization, plus `Signer.Verify` (`xml.Signer`).
- [x] **SOAP 1.2 Support**: `WithSOAPVersion`, typed `SoapFault`, `WithRetry`.

## 🚀 v2.x (Future / In Progress)
- [ ] **WSDL Parser**: Dynamic inspection of WSDL to validate actions.
- [ ] **Parallel Processing**: Streaming chunks in parallel goroutines.
