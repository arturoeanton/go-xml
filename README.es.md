# go-xml: El Parser XML Determinista Empresarial (v2.0)
> **Actualización v2.0**: Ahora impulsado por `OrderedMap` para 100% de determinismo y preservación de orden.

`go-xml` es un parser y serializador XML robusto y sin esquemas para Go. A diferencia de los parsers estándar que fuerzan a definir structs o pierden el orden de los elementos usando mapas estándar, `go-xml` preserva la estructura exacta, atributos y orden usando un `OrderedMap` personalizado.

Está diseñado para **Integración Empresarial** (Bancos, Gobierno, SOAP) donde el orden importa (ej: validación XSD, firmas SOAP).

## 🚀 Características Principales

*   **Parseo Determinista**: Parsea XML en `*OrderedMap`, preservando el orden de inserción de elementos y atributos.
*   **API OrderedMap**: API fluida para acceso y manipulación profunda (`m.Set("Body/Auth", val)`).
*   **Soporte de Streaming**:
    *   **Decoder**: Procesa archivos de multi-gigabytes con uso de memoria constante usando `Stream[T]` Genérico.
    *   **Encoder**: Escribe XML directamente a `io.Writer` desde `*OrderedMap` (cumplimiento estricto XSD).
*   **Robusto y Permisivo**: "Modo Sopa" para HTML sucio (`<br>`, `<meta>`, tags sin cerrar).
*   **Consultas Avanzadas**: Consultas profundas tipo XPath (`Query(m, "users/user[0]/name")`).
*   **Motor de Validación**: Define reglas de negocio (Regex, Rango, Enum, Tipo).
*   **Herramienta CLI (r2xml)**: Navaja suiza integrada para XML (Format, JSON, CSV, SOAP).
*   **Cliente SOAP Dinámico**: Llama servicios SOAP 1.1 o 1.2 sin generar código. Soporta **mTLS**, **WS-Security**, errores `SoapFault` tipados y reintentos configurables.
*   **Descubrimiento de WSDL**: Apuntá `ParseWSDL` a un `.wsdl` y obtené operaciones validadas — `soapAction`, endpoint y versión SOAP exactos, en vez de adivinarlos a mano.
*   **Firma Digital**: Firmas **XML-DSig** y **XAdES-BES** con **Canonicalización XML Exclusiva** real (la variante que las firmas enveloped realmente necesitan), más `Verify()` para comprobar una firma producida de punta a punta en vez de confiar ciegamente.

## 📦 Instalación

```bash
go get github.com/arturoeanton/go-xml
```

## 📖 Guía de Uso

### 1. Parseo Básico (OrderedMap)
La función principal `MapXML` retorna un `*OrderedMap`.

```go
package main

import (
    "fmt"
    "strings"
    "github.com/arturoeanton/go-xml/xml" 
)

func main() {
    xmlData := `<library><book id="1">El Principito</book></library>`

    // 1. Parsear a OrderedMap
    m, _ := xml.MapXML(strings.NewReader(xmlData))

    // 2. Acceso Tipado Seguro (Sin pánicos)
    title := m.String("library/book/#text") // "El Principito"
    id := m.String("library/book/@id")      // "1" (Atributos usan '@')

    fmt.Printf("Libro: %s (ID: %s)\n", title, id)
}
```

### 2. Creación y Modificación de XML
Crear estructuras XML es fluido y legible.

```go
m := xml.NewMap()

// Setters Fluidos Profundos
m.Set("Order/ID", "1001")
m.Set("Order/Customer/Name", "Alice")
m.Set("Order/Customer/@id", "C55") // Atributo

// Serializar (Salida Determinista)
s, _ := xml.Marshal(m, xml.WithPrettyPrint())
fmt.Println(s)
// Salida:
// <Order>
//   <ID>1001</ID>
//   <Customer id="C55">
//     <Name>Alice</Name>
//   </Customer>
// </Order>
```

### 3. Herramienta CLI (r2xml)
La librería actúa como una herramienta CLI independiente.

```bash
# Pretty Print / Formatear
go run main.go fmt sucio.xml

# Convertir a JSON
go run main.go json data.xml

# Convertir Lista a CSV (Aplanar)
go run main.go csv pedidos.xml --path="orders/order" > reporte.csv

# Consultar (XPath-lite)
go run main.go query data.xml "users/user[id=1]/name"

# Ejecutar Request SOAP desde Config
go run main.go soap request.json
```

### 4. Cliente SOAP Dinámico (con mTLS)
Consume servicios SOAP dinámicamente.

```go
// 1. Configurar
client := xml.NewSoapClient(
    "https://secure-bank.com/service", 
    "http://tempuri.org/",
    xml.WithClientCertificate("client.crt", "client.key"), // Soporte mTLS
    xml.WithBasicAuth("user", "pass"),
)

// 2. Llamar (El Payload respeta el orden de claves)
payload := xml.NewMap()
payload.Put("FromAccount", "123")
payload.Put("ToAccount", "456")
payload.Put("Amount", 100.50)

resp, err := client.Call("TransferFunds", payload)

// 3. Verificar Resultado
status := resp.String("Envelope/Body/TransferResponse/Status")
```

### 5. Características Avanzadas

#### Manejo de Arrays
Usa `ForceArray` para asegurar que tags específicos sean siempre listas.
```go
m, _ := xml.MapXML(r, xml.ForceArray("item"))
items := m.List("order/item") // Siempre []*OrderedMap
```

#### Mutación de Nodos
Refactoriza tus datos fácilmente.
```go
m.Rename("legacy_key", "new_key")
m.Move("temp/data", "final/destination")
```

#### Streaming (Archivos Grandes)
Procesa GBs de datos con memoria constante.
```go
stream := xml.NewStream[Order](file, "Order")
for order := range stream.Iter() {
    process(order)
}
```

### 6. Firmas Digitales (XML-DSig / XAdES-BES)
La firma usa **Canonicalización XML Exclusiva** real (`http://www.w3.org/2001/10/xml-exc-c14n#`) — la variante que XML-DSig enveloped, WS-Security y XAdES-BES (incluida la facturación electrónica DIAN de Colombia) realmente requieren en la práctica. `Verify` permite confirmar que una firma producida es válida en vez de confiar ciegamente.

```go
signer, _ := xml.NewSigner(certPEM, keyPEM)

// 1. Serializar el documento ANTES de que exista la firma — eso es lo que se referencia.
docBytes, _ := xml.Marshal(doc)

// 2. Firmar (XAdES-BES; usar CreateSignature para XML-DSig simple)
sig, _ := signer.CreateXadesSignature([]byte(docBytes))

// 3. Embeber la firma y re-serializar
invoiceNode.Set("ds:Signature", sig)
finalXML, _ := xml.Marshal(doc)

// 4. Verificar (digest + firma RSA) — detecta tanto manipulación como regresiones
if err := signer.Verify([]byte(finalXML)); err != nil {
    log.Fatal("firma inválida: ", err)
}
```

> **¿Tenés un certificado `.p12`/`.pfx`?** `NewSigner` recibe PEM, siguiendo el diseño sin dependencias externas de esta librería (sin `golang.org/x/crypto/pkcs12` ni similares). Convertilo una vez con OpenSSL:
> ```bash
> openssl pkcs12 -in cert.p12 -out cert.pem -clcerts -nokeys
> openssl pkcs12 -in cert.p12 -out key.pem -nocerts -nodes
> ```

### 7. Descubrimiento de WSDL (validar antes de llamar)
`ParseWSDL` lee un archivo WSDL 1.1 y lo resuelve en operaciones invocables — el `soapAction`, endpoint y versión SOAP exactos, en vez de la convención adivinada `namespace/action` de `SoapClient.Call` (frecuentemente incorrecta: muchos servicios reales usan `soapAction` vacío o una URN sin relación).

```go
f, _ := os.Open("service.wsdl")
wsdl, _ := xml.ParseWSDL(f)

for _, op := range wsdl.Operations() {
    fmt.Println(op.Name, op.SOAPAction, op.Endpoint)
}

client, _ := xml.NewSoapClientFromWSDL(wsdl) // endpoint/namespace/versión desde el WSDL
resp, err := client.CallOperation(wsdl, "GetTemperature", payload) // error si la acción no existe
```

O desde la CLI:
```bash
r2xml wsdl service.wsdl                                    # listar operaciones descubiertas
r2xml call --wsdl=service.wsdl --action=GetTemperature --data="city=Bogota"
```

**Alcance**: solo WSDL 1.1, un solo archivo (sin `wsdl:import`/`xsd:import`), sin modelado de tipos XSD — las partes de mensaje exponen su nombre de elemento/tipo resuelto, no un árbol de esquema completo. Cubre el caso común (descubrir y validar servicios SOAP reales) sin el peso de un generador de código completo.

## ⚙️ Arquitectura: OrderedMap

En v2.0, reemplazamos `map[string]any` con `*OrderedMap`.
- **¿Por qué?** El mapa nativo de Go aleatoriza el orden de iteración. XML (XSD) y SOAP a menudo requieren un orden estricto de elementos.
- **¿Cómo?** `OrderedMap` mantiene un slice de claves `[]string` junto con el mapa.
- **Beneficio**: Leer XML -> Modificar -> Escribir XML = **Estructura Idéntica**.

## 📄 Licencia
MIT
