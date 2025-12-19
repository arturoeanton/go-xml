# Tutorial Cliente SOAP (v2.0)

`go-xml` incluye un potente cliente SOAP 1.1 dinámico que maneja payloads usando `OrderedMap` para asegurar cumplimiento estricto XML.

## Características
- **Dinámico**: No requiere generación de structs (wsdl2go).
- **Seguro**: Soporta Basic Auth, WS-Security y **mTLS**.
- **Determinismo**: Los payloads respetan el orden definido.

## Ejemplo: Llamada Bancaria Segura

### 1. Configuración (Código)

```go
client := xml.NewSoapClient(
    "https://api.banco-seguro.com/soap",   // Endpoint
    "http://tempuri.org/services",         // Namespace
    
    // Stack de Seguridad
    xml.WithClientCertificate("cert.pem", "key.pem"), // Mutual TLS
    xml.WithWSSecurity("user", "password"),           // Header WS-Security
    xml.WithHeader("User-Agent", "Go-XML-Client"),
)
```

### 2. Preparar Payload (Orden Estricto)

```go
payload := xml.NewMap()

// ¡El orden importa en SOAP!
payload.Set("Auth/Token", "ABC-123")
payload.Set("Transaction/ID", 999)
payload.Set("Transaction/Amount", 500.00)
```

### 3. Ejecutar
```go
resp, err := client.Call("ProcessPayment", payload)
if err != nil {
    log.Fatal(err) // Parsea automáticamente SOAP Faults
}

fmt.Println("Estado:", resp.String("Envelope/Body/ProcessPaymentResponse/Status"))
```

---

## Ejecución CLI (Sin Código)
Puedes verificar endpoints SOAP usando un archivo de configuración JSON.

**request.json**:
```json
{
  "endpoint": "https://api.banco.com/ws",
  "namespace": "http://banco.com/",
  "action": "Login",
  "auth": {
    "type": "wsse",
    "user": "admin",
    "pass": "secret"
  },
  "cert_file": "client.crt",
  "key_file": "client.key",
  "payload": {
    "User": "admin"
  }
}
```

**Ejecutar:**
```bash
go run main.go soap request.json
```
