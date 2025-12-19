# SOAP Client Tutorial (v2.0)

`go-xml` includes a powerful dynamic SOAP 1.1 client that handles payloads using `OrderedMap` to ensure strict XML compliance.

## Features
- **Dynamic**: No struct generation required.
- **Secure**: Supports Basic Auth, WS-Security (Check/Sign), and **mTLS**.
- **Determinism**: Payloads respect the order you define.

## Example: Secure Banking Call

### 1. Configuration (Code)

```go
client := xml.NewSoapClient(
    "https://api.secure-bank.com/soap",    // Endpoint
    "http://tempuri.org/services",         // Namespace
    
    // Security Stack
    xml.WithClientCertificate("cert.pem", "key.pem"), // Mutual TLS
    xml.WithWSSecurity("user", "password"),           // WS-Security Header
    xml.WithHeader("User-Agent", "Go-XML-Client"),
)
```

### 2. Prepare Payload (Strict Order)

```go
payload := xml.NewMap()

// Order matters for SOAP!
payload.Set("Auth/Token", "ABC-123")
payload.Set("Transaction/ID", 999)
payload.Set("Transaction/Amount", 500.00)
```

### 3. Execute
```go
resp, err := client.Call("ProcessPayment", payload)
if err != nil {
    log.Fatal(err) // Automatically parses SOAP Faults
}

fmt.Println("Status:", resp.String("Envelope/Body/ProcessPaymentResponse/Status"))
```

---

## CLI Execution (No Code)
You can verify SOAP endpoints using a JSON config file.

**request.json**:
```json
{
  "endpoint": "https://api.bank.com/ws",
  "namespace": "http://bank.com/",
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

**Run:**
```bash
go run main.go soap request.json
```
