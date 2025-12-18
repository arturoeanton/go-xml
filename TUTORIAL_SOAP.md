# Tutorial: Dynamic SOAP Client

This tutorial guides you through using the `SoapClient` to interact with SOAP 1.1 web services without defining Go structs for every request and response.

## 1. Concept

Traditional SOAP clients in Go require generating thousands of lines of code from WSDL files. `go-xml` takes a dynamic approach:

1.  **Map Input**: You build the request body using `map[string]any`.
2.  **Map Output**: The response is parsed into a method-agnostic map.
3.  **XPath Querying**: You extract data using path queries.

## 2. Setup

```go
package main

import (
	"fmt"
	"github.com/arturoeanton/go-xml/xml"
)

func main() {
    // Initialize standard client
    client := xml.NewSoapClient(
        "http://www.dneonline.com/calculator.asmx", // Endpoint
        "http://tempuri.org/",                      // Namespace
    )
}
```

## 3. Making a Call (Add Example)

To call the `Add` operation which expects parameters `intA` and `intB`:

```go
func main() {
    client := xml.NewSoapClient("http://www.dneonline.com/calculator.asmx", "http://tempuri.org/")

    // Construct Load
    // SOAP Envelope and Body are handled automatically.
    // You only provide the content inside the Action tag.
    payload := map[string]any{
        "intA": 10,
        "intB": 20,
    }

    // "Add" becomes "<m:Add>...</m:Add>" automatically.
    resp, err := client.Call("Add", payload)
    if err != nil {
        panic(err)
    }
    
    // Response Data Extraction
    // Option A: Full Path
    // result, _ := xml.Query(resp, "Envelope/Body/AddResponse/AddResult")
    
    // Option B: Deep Search (Easier)
    result, _ := xml.Query(resp, "//AddResult")
    fmt.Printf("Result: %v\n", result) 
}
```

## 4. Advanced Options

### Custom Headers
You can inject custom HTTP headers (e.g., for tracing or anti-bot measures).

```go
client := xml.NewSoapClient(url, ns, 
    xml.WithHeader("X-Correlation-ID", "12345"),
    xml.WithHeader("User-Agent", "MySoapClient/1.0"),
)
```

### Timeouts
Set a custom timeout for the underlying HTTP client.

```go
client := xml.NewSoapClient(url, ns, xml.WithTimeout(10 * time.Second))
```

## 5. Authentication

### Basic Auth
```go
client := xml.NewSoapClient(url, ns, xml.WithBasicAuth("user", "pass"))
```

### WS-Security
Adds the standard `wsse:Security` header for UsernameToken profile.

```go
client := xml.NewSoapClient(url, ns, xml.WithWSSecurity("user", "secret"))
```

## 6. Handling Errors (SOAP Faults)

If the server returns a SOAP Fault (e.g., 500 Internal Server Error with XML body), `Call` returns an error containing the fault string.

```go
resp, err := client.Call("Divide", map[string]any{"intA": 10, "intB": 0})
if err != nil {
    fmt.Println("SOAP Error:", err) 
    // Output: SOAP Fault 500: [soap:Client] System.Web.Services.Protocols.SoapException: ...
}
```
