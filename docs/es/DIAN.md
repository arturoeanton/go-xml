# Tutorial de Integración: DIAN Colombia (Factura Electrónica)

Este tutorial explica cómo generar una **Factura Electrónica UBL 2.1** válida para la DIAN (Colombia) utilizando `go-xml`.

El proceso para la DIAN es complejo debido a tres requisitos estrictos:
1.  **CUFE** (Código Único de Factura Electrónica): Un hash SHA-384 de campos específicos.
2.  **Firma XAdES-BES**: Firma digital avanzada sobre el XML canonicalizado.
3.  **Orden Estricto**: La estructura UBL debe seguir el orden XSD exacto.

## Prerrequisitos
*   Certificado Digital (`.crt` o `.pem`) y Clave Privada (`.key`).
*   Clave Técnica del portal de habilitación DIAN (para calcular CUFE).
*   Rangos de numeración autorizados.

---

## 1. Estructura del Proyecto
Para este ejemplo, asumimos que tienes dos helpers:
*   `xml/singer.go`: Lógica para generar firmas XAdES.
*   `xml/dian_utils.go`: Lógica para calcular el CUFE.

## 2. Flujo Completo (Paso a Paso)

### A. Configuración y Certificados
Primero cargamos los certificados que se usarán para firmar.

```go
crt, _ := os.ReadFile("certificado.crt")
key, _ := os.ReadFile("privada.key")
signer, _ := xml.NewSigner(crt, key)
```

### B. Cálculo del CUFE
El CUFE es la "huella digital" de la factura. Debe calcularse **antes** de generar el XML, ya que el valor del CUFE se incluye dentro del XML.

La fórmula es concatenar valores específicos y aplicar SHA-384.

```go
// Datos de la factura
numFac := "SETP-99000001"
issueDate := "2025-12-19"
issueTime := "12:00:00-05:00" // IMPORTANTE: Incluir offset horario (-05:00)
valTotal := "1000.00"
valImp := "0.00"
valPagar := "1000.00"
nitEmisor := "800197268"
nitReceptor := "222222222222"
claveTecnica := "fc8eac422eba16..." // Del portal DIAN
ambiente := "2" // 1=Prod, 2=Pruebas

// Helper (Ver xml/dian_utils.go)
cufe := xml.CalculateCUFE(
    numFac, issueDate, issueTime, valTotal,
    "01", valImp,        // Impuesto 1 (IVA)
    "04", "0.00",        // Impuesto 2 (Consumo)
    valPagar,
    nitEmisor, nitReceptor,
    claveTecnica, ambiente,
)
```

### C. Construcción del XML (UBL 2.1)
Usamos `xml.NewMap()` para construir la estructura. El orden de inserción en `OrdereMap` garantiza que el XML final sea válido.

```go
invoiceData := xml.NewMap()

// 1. Namespaces UBL 2.1
invoiceData.Set("@xmlns", "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2")
invoiceData.Set("@xmlns:cac", "urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2")
invoiceData.Set("@xmlns:cbc", "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2")
// ... otros namespaces (ds, ext, xades) ...

// 2. Cabecera
invoiceData.Set("cbc:UBLVersionID", "UBL 2.1")
invoiceData.Set("cbc:ID", numFac)
invoiceData.Set("cbc:IssueDate", issueDate)
invoiceData.Set("cbc:IssueTime", issueTime)
invoiceData.Set("cbc:InvoiceTypeCode", "01")

// 3. Insertar CUFE (Elemento + Atributo)
invoiceData.Set("cbc:UUID", map[string]interface{}{
    "#text":       cufe,
    "@schemeName": "CUFE-SHA384",
})

// 4. Emisor y Receptor (PartyTaxScheme, etc.)
// ... Ver código completo en demo.go ...

// 5. Totales
totals := xml.NewMap()
totals.Set("cbc:LineExtensionAmount", map[string]interface{}{"#text": valTotal, "@currencyID": "COP"})
// ...
invoiceData.Set("cac:LegalMonetaryTotal", totals)
```

### D. Firma Digital (XAdES)
DIAN requiere que la firma esté dentro de una extensión UBL (`ext:UBLExtensions`).

1.  **Serializar** el contenido actual para tener los bytes a firmar.
2.  **Firmar** los bytes.
3.  **Inyectar** la firma en la estructura.

```go
// 1. Generar payload temporal para firmar
preRoot := xml.NewMap()
preRoot.Set("Invoice", invoiceData)
xmlBytesToSign, _ := xml.Marshal(preRoot)

// 2. Crear Firma XAdES
sig, _ := signer.CreateXadesSignature([]byte(xmlBytesToSign))

// 3. Estructura de Extensión UBL
extensionContent := xml.NewMap()
extensionContent.Set("ds:Signature", sig) // La firma va aquí

ublExtension := xml.NewMap()
ublExtension.Set("ext:ExtensionContent", extensionContent)

ublExtensions := xml.NewMap()
ublExtensions.Set("ext:UBLExtension", ublExtension)

// 4. Insertar Extensión en la Factura (Debe ser el primer hijo)
// Reconstruimos el mapa para asegurar que UBLExtensions vaya PRIMERO
finalInvoice := xml.NewMap()

// Copiar atributos (@)
for _, k := range invoiceData.Keys() {
    if k[0] == '@' { finalInvoice.Set(k, invoiceData.Get(k)) }
}

// Poner Extensiones
finalInvoice.Set("ext:UBLExtensions", ublExtensions)

// Copiar resto del contenido
for _, k := range invoiceData.Keys() {
    if k[0] != '@' { finalInvoice.Set(k, invoiceData.Get(k)) }
}
```

### E. Resultado Final
Finalmente envolvemos todo en el tag raíz y guardamos.

```go
root := xml.NewMap()
root.Set("Invoice", finalInvoice)

finalXML, _ := xml.Marshal(root)
os.WriteFile("factura_dian.xml", []byte(finalXML), 0644)
```

## Validaciones Comunes
*   **Regla 90**: El orden de los tags es crítico. Si recibes errores de esquema, verifica contra el XSD.
*   **Hora**: `IssueTime` debe incluir la zona horaria (ej: `-05:00`).
*   **CUFE**: Si el CUFE calculado no coincide con el que la DIAN recalcula allá, la factura será rechazada ("Regla: Validez del CUFE"). Verifica que los decimales y fechas sean idénticos en el hash y en el XML.

## Código Fuente de Ayuda
Revisar el archivo `demo.go` (función `demo_dian_ubl`) en el repositorio para ver el ensamblaje completo de los campos.
