# Roadmap (Hoja de Ruta)

## ✅ v1.0 (Completado)
- [x] Parseo básico a `map[string]any`.
- [x] Streaming Decoder/Encoder.
- [x] Motor de consultas XPath-lite.

## ✅ v2.0 (Versión Estable)
- [x] **Implementación OrderedMap**: Orden determinista para claves y atributos.
- [x] **Serializador Estricto**: La salida coincide exactamente con la entrada (Roundtrip 100%).
- [x] **Herramientas CLI**: Comandos `fmt`, `csv`, `json`, `soap`.
- [x] **Soporte mTLS**: Certificados de Cliente para SOAP.
- [x] **Exportación CSV**: Aplanar listas XML a CSV.
- [x] **Firma Digital**: XML-DSig y XAdES-BES con Canonicalización XML Exclusiva real, más `Signer.Verify` (`xml.Signer`).
- [x] **Soporte SOAP 1.2**: `WithSOAPVersion`, `SoapFault` tipado, `WithRetry`.

## 🚀 v2.x (Futuro / En Progreso)
- [ ] **Parser WSDL**: Inspección dinámica de WSDL para validar acciones.
- [ ] **Procesamiento Paralelo**: Streaming de chunks en goroutines paralelas.
