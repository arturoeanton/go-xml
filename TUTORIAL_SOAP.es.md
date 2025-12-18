# Tutorial: Cliente SOAP Dinámico

Este tutorial te guía en el uso de `SoapClient` para interactuar con servicios web SOAP 1.1 sin definir structs de Go para cada petición y respuesta.

## 1. Concepto

Los clientes SOAP tradicionales en Go requieren generar miles de líneas de código desde archivos WSDL. `go-xml` toma un enfoque dinámico:

1.  **Entrada Map**: Construyes el cuerpo de la petición usando `map[string]any`.
2.  **Salida Map**: La respuesta se parsea en un mapa agnóstico al método.
3.  **Consultas XPath**: Extraes datos usando `Query`.

## 2. Configuración

```go
package main

import (
    "fmt"
    "github.com/arturoeanton/go-xml/xml"
)

func main() {
    // Inicializar cliente estándar
    client := xml.NewSoapClient(
        "http://www.dneonline.com/calculator.asmx", // Endpoint
        "http://tempuri.org/",                      // Namespace
    )
}
```

## 3. Realizar una Llamada (Ejemplo Suma)

Para llamar a la operación `Add` que espera parámetros `intA` e `intB`:

```go
func main() {
    client := xml.NewSoapClient("http://www.dneonline.com/calculator.asmx", "http://tempuri.org/")

    // Construir Carga
    // El Envelope y Body se manejan automáticamente.
    // Solo provees el contenido dentro del tag de la Acción.
    payload := map[string]any{
        "intA": 10,
        "intB": 20,
    }

    // "Add" se convierte en "<m:Add>...</m:Add>"
    resp, err := client.Call("Add", payload)
    if err != nil {
        panic(err)
    }

    // Estructura de Respuesta:
    // <Envelope>
    //   <Body>
    //     <AddResponse>
    //       <AddResult>30</AddResult>
    //     </AddResponse>
    // ...
    
    result, _ := xml.Query(resp, "Envelope/Body/AddResponse/AddResult")
    fmt.Printf("Resultado: %v\n", result) 
}
```

## 4. Autenticación

### Basic Auth
```go
client := xml.NewSoapClient(url, ns, xml.WithBasicAuth("user", "pass"))
```

### WS-Security
Añade el header estándar `wsse:Security` para el perfil UsernameToken.

```go
client := xml.NewSoapClient(url, ns, xml.WithWSSecurity("user", "secret"))
```

## 5. Manejo de Errores (SOAP Faults)

Si el servidor retorna un SOAP Fault (ej: 500 Internal Server Error con cuerpo XML), `Call` retorna un error conteniendo el string del fallo.

```go
resp, err := client.Call("Divide", map[string]any{"intA": 10, "intB": 0})
if err != nil {
    fmt.Println("Error SOAP:", err) 
    // Output: SOAP Fault 500: [soap:Client] System.Web.Services.Protocols.SoapException: ...
}
```
