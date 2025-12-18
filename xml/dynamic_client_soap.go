package xml

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SoapClient is a dynamic client for consuming SOAP services without structs.
type SoapClient struct {
	EndpointURL    string
	Namespace      string
	HttpClient     *http.Client
	SoapActionBase string // Optional: Overrides Namespace for SOAPAction header generation
}

// ClientOption allows configuring the SoapClient (e.g., custom timeout).
type ClientOption func(*SoapClient)

// WithTimeout sets a custom timeout for the HTTP client.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *SoapClient) {
		c.HttpClient.Timeout = timeout
	}
}

// WithSoapActionBase sets a custom base URL for the SOAPAction header.
// Useful when the SOAPAction URL differs from the XML Namespace.
func WithSoapActionBase(base string) ClientOption {
	return func(c *SoapClient) {
		c.SoapActionBase = base
	}
}

// NewSoapClient creates a new instance of SoapClient.
// endpoint: The full URL of the SOAP service.
// namespace: The XML namespace of the service specific method (usually "http://tempuri.org/" or similar).
func NewSoapClient(endpoint, namespace string, opts ...ClientOption) *SoapClient {
	client := &SoapClient{
		EndpointURL: endpoint,
		Namespace:   namespace,
		HttpClient: &http.Client{
			Timeout: 30 * time.Second, // Default timeout
		},
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

// Call executes a SOAP Action.
// action: The name of the method to call (e.g., "GetUser").
// payload: The content map to be placed inside the method key.
// Returns a map representing the parsed SOAP response body.
func (c *SoapClient) Call(action string, payload map[string]any) (map[string]any, error) {
	// 1. Build the SOAP Envelope
	envelope := map[string]any{
		"soap:Envelope": map[string]any{
			"@xmlns:soap": "http://schemas.xmlsoap.org/soap/envelope/",
			"@xmlns:m":    c.Namespace,
			"soap:Body": map[string]any{
				"m:" + action: payload,
			},
		},
	}

	// 2. Encode to XML
	var buf bytes.Buffer
	enc := NewEncoder(&buf) // Assumes NewEncoder exists in package xml
	if err := enc.Encode(envelope); err != nil {
		return nil, fmt.Errorf("failed to encode SOAP request: %w", err)
	}

	// 3. Create HTTP Request
	req, err := http.NewRequest("POST", c.EndpointURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	// Some SOAP servers require the action in quotes, some without.
	// We'll append the namespace if strict SOAP 1.1? Or just the action.
	// Usually it is "namespace/action" or just "action".
	// Let's use the provided action combined with namespace if typical,
	// but user asked for simple "m:action" mapping.
	// For Headers, strictly "SOAPAction" often needs namespace + action.
	// Let's try constructing it sanely:
	soapActionHeader := action
	// Determine base for SOAPAction
	base := c.Namespace
	if c.SoapActionBase != "" {
		base = c.SoapActionBase
	}

	if base != "" && !strings.Contains(action, "/") {
		// e.g. "http://tempuri.org/Action"
		trimNs := strings.TrimSuffix(base, "/")
		soapActionHeader = fmt.Sprintf("%s/%s", trimNs, action)
	}
	req.Header.Set("SOAPAction", soapActionHeader)

	// 4. Send Request
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("soap call network error: %w", err)
	}
	defer resp.Body.Close()

	// 5. Parse Response
	// Even if status != 200, we might receive a SOAP Fault XML.
	respMap, err := MapXML(resp.Body)
	if err != nil {
		// If we can't parse the XML even on error, return distinct error
		return nil, fmt.Errorf("failed to parse response (status %d): %w", resp.StatusCode, err)
	}

	// 6. Handle HTTP Errors / SOAP Faults
	if resp.StatusCode != http.StatusOK {
		// Attempt to extract Fault string
		// Standard SOAP 1.1 Fault: Envelope/Body/Fault
		if fault, _ := Query(respMap, "Envelope/Body/Fault"); fault != nil {
			if fMap, ok := fault.(map[string]any); ok {
				faultCode := fMap["faultcode"]
				faultString := fMap["faultstring"]
				return nil, fmt.Errorf("SOAP Fault %d: [%v] %v", resp.StatusCode, faultCode, faultString)
			}
		}
		return nil, fmt.Errorf("http error %d", resp.StatusCode)
	}

	return respMap, nil
}
