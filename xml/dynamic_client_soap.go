package xml

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Authentication types
const (
	AuthNone       = ""
	AuthBasic      = "Basic"
	AuthBearer     = "Bearer"
	AuthWSSecurity = "WSSecurity"
)

// SoapVersion selects the envelope/Content-Type used by Call.
type SoapVersion int

const (
	Soap11 SoapVersion = iota // http://schemas.xmlsoap.org/soap/envelope/, text/xml + SOAPAction header
	Soap12                    // http://www.w3.org/2003/05/soap-envelope, application/soap+xml with action= in Content-Type
)

const (
	soap11EnvelopeNS = "http://schemas.xmlsoap.org/soap/envelope/"
	soap12EnvelopeNS = "http://www.w3.org/2003/05/soap-envelope"
)

// SoapClient allows dynamic calls to SOAP services without structs.
type SoapClient struct {
	EndpointURL    string
	Namespace      string
	HttpClient     *http.Client
	SoapActionBase string
	Headers        map[string]string
	Version        SoapVersion

	AuthType     string
	AuthUsername string
	AuthPassword string
	AuthToken    string

	// --- mTLS Config ---
	CertFile string
	KeyFile  string
	Insecure bool // Skip Verify

	// --- Retry ---
	RetryAttempts int           // 0 or 1 = no retries
	RetryBackoff  time.Duration // fixed wait between attempts
}

// --- mTLS Options ---

// WithClientCertificate enables mTLS using PEM files (.crt and .key).
func WithClientCertificate(certFile, keyFile string) ClientOption {
	return func(s *SoapClient) {
		s.CertFile = certFile
		s.KeyFile = keyFile
	}
}

// WithInsecureSkipVerify skips validation of the server certificate.
// Useful for development (self-signed).
func WithInsecureSkipVerify() ClientOption {
	return func(s *SoapClient) { s.Insecure = true }
}

// ClientOption for functional configuration.
type ClientOption func(*SoapClient)

func WithTimeout(d time.Duration) ClientOption {
	return func(s *SoapClient) { s.HttpClient.Timeout = d }
}

func WithHeader(key, value string) ClientOption {
	return func(s *SoapClient) { s.Headers[key] = value }
}

func WithSoapActionBase(base string) ClientOption {
	return func(s *SoapClient) { s.SoapActionBase = base }
}

// WithSOAPVersion selects SOAP 1.1 (default) or SOAP 1.2.
func WithSOAPVersion(v SoapVersion) ClientOption {
	return func(s *SoapClient) { s.Version = v }
}

// WithRetry retries the call up to `attempts` times (with a fixed `backoff`
// wait between attempts) when the error is a transport error (connection,
// timeout, DNS). An HTTP response that was already received — including a
// SOAP Fault delivered with status 500 — is never retried: retrying does not
// change the outcome of a business error.
func WithRetry(attempts int, backoff time.Duration) ClientOption {
	return func(s *SoapClient) {
		s.RetryAttempts = attempts
		s.RetryBackoff = backoff
	}
}

// --- Auth Options ---

func WithBasicAuth(user, pass string) ClientOption {
	return func(s *SoapClient) {
		s.AuthType = AuthBasic
		s.AuthUsername = user
		s.AuthPassword = pass
	}
}

func WithBearerToken(token string) ClientOption {
	return func(s *SoapClient) {
		s.AuthType = AuthBearer
		s.AuthToken = token
	}
}

func WithWSSecurity(user, pass string) ClientOption {
	return func(s *SoapClient) {
		s.AuthType = AuthWSSecurity
		s.AuthUsername = user
		s.AuthPassword = pass
	}
}

// NewSoapClient creates a new client.
func NewSoapClient(endpoint, namespace string, opts ...ClientOption) *SoapClient {
	client := &SoapClient{
		EndpointURL: endpoint,
		Namespace:   namespace,
		HttpClient:  &http.Client{Timeout: 30 * time.Second},
		Headers:     make(map[string]string),
		AuthType:    AuthNone,
	}
	for _, opt := range opts {
		opt(client)
	}

	// === mTLS LOGIC ===
	tlsConfig := &tls.Config{}
	hasTlsConfig := false

	// 1. Load Client Certificate (PEM)
	if client.CertFile != "" && client.KeyFile != "" {
		// We use our wrapper function from cert.go
		cert, err := LoadCert(client.CertFile, client.KeyFile)
		if err != nil {
			// Critical warning, but no panic (so we don't take down entire apps)
			fmt.Printf("❌ CRITICAL: Failed to load mTLS certificates: %v\n", err)
		} else {
			tlsConfig.Certificates = []tls.Certificate{cert}
			hasTlsConfig = true
		}
	}

	// 2. Insecure Skip Verify
	if client.Insecure {
		tlsConfig.InsecureSkipVerify = true
		hasTlsConfig = true
	}

	// 3. Apply Transport
	if hasTlsConfig {
		// We would clone the default transport to keep Proxy settings (if any),
		// but since we're creating a new one, we use the basic Transport + TLS
		client.HttpClient.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	}

	return client
}

// SoapFault represents a typed <soap:Fault> (SOAP 1.1 faultcode/
// faultstring or SOAP 1.2 Code/Reason), instead of a generic error —
// it allows using errors.As to inspect Code/Message/Detail.
type SoapFault struct {
	Code    string
	Message string
	Actor   string
	Detail  *OrderedMap
}

func (f *SoapFault) Error() string {
	if f.Actor != "" {
		return fmt.Sprintf("SOAP fault [%s] (actor=%s): %s", f.Code, f.Actor, f.Message)
	}
	return fmt.Sprintf("SOAP fault [%s]: %s", f.Code, f.Message)
}

// extractSoapFault looks for a soap:Fault in the parsed response, supporting
// both the SOAP 1.1 form (faultcode/faultstring/faultactor/detail) and the
// SOAP 1.2 form (Code/Value, Reason/Text, Detail). Returns nil if there is
// no recognizable Fault.
func extractSoapFault(respMap *OrderedMap) *SoapFault {
	fault, _ := Query(respMap, "Envelope/Body/Fault")
	fMap, ok := fault.(*OrderedMap)
	if !ok {
		return nil
	}

	if code := fMap.String("faultcode"); code != "" {
		sf := &SoapFault{Code: code, Message: fMap.String("faultstring"), Actor: fMap.String("faultactor")}
		if d, ok := fMap.Get("detail").(*OrderedMap); ok {
			sf.Detail = d
		}
		return sf
	}

	codeNode, _ := fMap.Get("Code").(*OrderedMap)
	reasonNode, _ := fMap.Get("Reason").(*OrderedMap)
	if codeNode == nil && reasonNode == nil {
		return nil
	}
	sf := &SoapFault{}
	if codeNode != nil {
		sf.Code = codeNode.String("Value")
	}
	if reasonNode != nil {
		sf.Message = reasonNode.String("Text")
	}
	if d, ok := fMap.Get("Detail").(*OrderedMap); ok {
		sf.Detail = d
	}
	return sf
}

// buildEnvelope constructs the soap:Envelope (payload, WS-Security header,
// body) for action/payload and returns its encoded bytes. Shared by Call and
// CallOperation.
func (c *SoapClient) buildEnvelope(action string, payload any) ([]byte, error) {
	// 1. Prepare the Payload
	actionNode := NewMap()
	actionNode.Put("@xmlns", c.Namespace)

	// Ingest Payload
	if payload != nil {
		if om, ok := payload.(*OrderedMap); ok {
			om.ForEach(func(k string, v any) bool {
				actionNode.Put(k, v)
				return true
			})
		} else if m, ok := payload.(map[string]any); ok {
			keys := sortedKeys(m)
			for _, k := range keys {
				actionNode.Put(k, m[k])
			}
		} else {
			return nil, fmt.Errorf("unsupported payload type: %T", payload)
		}
	}

	// 2. Build Base Envelope
	envelopeNS := soap11EnvelopeNS
	if c.Version == Soap12 {
		envelopeNS = soap12EnvelopeNS
	}
	envelopeMap := NewMap()
	envelopeMap.Put("@xmlns:soap", envelopeNS)

	// 3. Inject WS-Security (if applicable) - Headers go BEFORE the Body
	if c.AuthType == AuthWSSecurity {
		security := NewMap()
		security.Put("@xmlns:wsse", "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd")

		usernameToken := NewMap()
		usernameToken.Put("wsse:Username", c.AuthUsername)

		password := NewMap()
		password.Put("@Type", "http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordText")
		password.Put("#text", c.AuthPassword)
		usernameToken.Put("wsse:Password", password)

		security.Put("wsse:UsernameToken", usernameToken)

		header := NewMap()
		header.Put("wsse:Security", security)
		envelopeMap.Put("soap:Header", header)
	}

	// 4. Body
	body := NewMap()
	body.Put(action, actionNode)
	envelopeMap.Put("soap:Body", body)

	envelope := NewMap()
	envelope.Put("soap:Envelope", envelopeMap)

	// 5. Encode
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(envelope); err != nil {
		return nil, fmt.Errorf("failed to encode SOAP request: %w", err)
	}
	return buf.Bytes(), nil
}

// doCall sends bodyBytes to the endpoint with the given exact soapAction
// (retrying on transport errors per WithRetry), parses the response, and
// surfaces a *SoapFault for non-2xx responses that carry one.
func (c *SoapClient) doCall(bodyBytes []byte, soapAction string) (*OrderedMap, error) {
	attempts := c.RetryAttempts
	if attempts < 1 {
		attempts = 1
	}

	var resp *http.Response
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if c.RetryBackoff > 0 {
				time.Sleep(c.RetryBackoff)
			}
		}

		req, err := http.NewRequest("POST", c.EndpointURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		if c.Version == Soap12 {
			req.Header.Set("Content-Type", fmt.Sprintf(`application/soap+xml; charset=utf-8; action="%s"`, soapAction))
		} else {
			req.Header.Set("Content-Type", "text/xml; charset=utf-8")
			req.Header.Set("SOAPAction", fmt.Sprintf("\"%s\"", soapAction))
		}
		req.Header.Set("User-Agent", "r2-xml-client/2.0")

		switch c.AuthType {
		case AuthBasic:
			req.SetBasicAuth(c.AuthUsername, c.AuthPassword)
		case AuthBearer:
			req.Header.Set("Authorization", "Bearer "+c.AuthToken)
		}
		for k, v := range c.Headers {
			req.Header.Set(k, v)
		}

		resp, lastErr = c.HttpClient.Do(req)
		if lastErr == nil {
			break
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("soap call network error: %w", lastErr)
	}
	defer resp.Body.Close()

	respMap, err := MapXML(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response (status %d): %w", resp.StatusCode, err)
	}

	if resp.StatusCode != http.StatusOK {
		if fault := extractSoapFault(respMap); fault != nil {
			return nil, fault
		}
		return nil, fmt.Errorf("http error %d", resp.StatusCode)
	}

	return respMap, nil
}

// Call executes a SOAP action.
// payload can be *OrderedMap (preserves order) or map[string]any (sorted alphabetically).
// SOAPAction is reconstructed as "namespace/action" (or SoapActionBase/action) —
// a convention that does not match the real soapAction of many services.
// If you have the WSDL, use CallOperation for the exact value.
func (c *SoapClient) Call(action string, payload any) (*OrderedMap, error) {
	bodyBytes, err := c.buildEnvelope(action, payload)
	if err != nil {
		return nil, err
	}

	base := c.Namespace
	if c.SoapActionBase != "" {
		base = c.SoapActionBase
	}
	cleanBase := strings.TrimSuffix(base, "/")
	cleanAction := strings.TrimPrefix(action, "/")
	soapAction := fmt.Sprintf("%s/%s", cleanBase, cleanAction)

	return c.doCall(bodyBytes, soapAction)
}

// CallOperation executes action using the exact soapAction, endpoint and
// SOAP version declared in w (instead of Call's guessed convention).
// Returns an error if action does not exist in the WSDL.
func (c *SoapClient) CallOperation(w *WSDL, action string, payload any) (*OrderedMap, error) {
	op, err := w.Operation(action)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := c.buildEnvelope(action, payload)
	if err != nil {
		return nil, err
	}

	return c.doCall(bodyBytes, op.SOAPAction)
}

// NewSoapClientFromWSDL builds a SoapClient using the first SOAP endpoint
// declared in w (see WSDL.Endpoint) — namespace and version included.
func NewSoapClientFromWSDL(w *WSDL, opts ...ClientOption) (*SoapClient, error) {
	endpoint, err := w.Endpoint()
	if err != nil {
		return nil, err
	}
	ops := w.Operations()
	namespace, version := ops[0].Namespace, ops[0].Version

	allOpts := append([]ClientOption{WithSOAPVersion(version)}, opts...)
	return NewSoapClient(endpoint, namespace, allOpts...), nil
}
