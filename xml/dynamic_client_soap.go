package xml

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Tipos de Autenticación
const (
	AuthNone       = ""
	AuthBasic      = "Basic"
	AuthBearer     = "Bearer"
	AuthWSSecurity = "WSSecurity"
)

// SoapClient permite llamadas dinámicas a servicios SOAP sin structs.
type SoapClient struct {
	EndpointURL    string
	Namespace      string
	HttpClient     *http.Client
	SoapActionBase string
	Headers        map[string]string

	// Configuración de Auth
	AuthType     string
	AuthUsername string
	AuthPassword string
	AuthToken    string
}

// ClientOption para configuración funcional.
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

// NewSoapClient crea un nuevo cliente.
func NewSoapClient(endpoint, namespace string, opts ...ClientOption) *SoapClient {
	client := &SoapClient{
		EndpointURL: endpoint,
		Namespace:   namespace,
		HttpClient:  &http.Client{Timeout: 30 * time.Second},
		Headers:     make(map[string]string), // IMPORTANTE: Inicializar mapa
		AuthType:    AuthNone,
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

// Call ejecuta una acción SOAP.
func (c *SoapClient) Call(action string, payload map[string]any) (*OrderedMap, error) {
	// 1. Preparar el Payload
	// Definimos el xmlns en el nodo de acción para que los hijos lo hereden.
	actionNode := NewMap()
	actionNode.Put("@xmlns", c.Namespace)

	// Convert payload map to OrderedMap for order preservation (if user passes map)
	// Ideally user should pass OrderedMap but interface says map[string]any.
	// We'll iterate sorted keys to be deterministic at least.
	keys := sortedKeys(payload)
	for _, k := range keys {
		actionNode.Put(k, payload[k])
	}

	// 2. Construir Envelope Base
	envelopeMap := NewMap()
	envelopeMap.Put("@xmlns:soap", "http://schemas.xmlsoap.org/soap/envelope/")

	body := NewMap()
	body.Put(action, actionNode)
	envelopeMap.Put("soap:Body", body)

	// 3. Inyectar WS-Security (Si aplica)
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

	envelope := NewMap()
	envelope.Put("soap:Envelope", envelopeMap)

	// 4. Encode
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	if err := enc.Encode(envelope); err != nil {
		return nil, fmt.Errorf("failed to encode SOAP request: %w", err)
	}

	// 5. Create Request
	req, err := http.NewRequest("POST", c.EndpointURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Headers Base
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("User-Agent", "r2-xml-client/1.0") // Anti-bloqueo

	// Auth Headers (HTTP Level)
	switch c.AuthType {
	case AuthBasic:
		req.SetBasicAuth(c.AuthUsername, c.AuthPassword)
	case AuthBearer:
		req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	}

	// Custom Headers
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}

	// 6. SOAPAction Logic (Estricto)
	base := c.Namespace
	if c.SoapActionBase != "" {
		base = c.SoapActionBase
	}

	// Limpieza de slashes dobles
	cleanBase := strings.TrimSuffix(base, "/")
	cleanAction := strings.TrimPrefix(action, "/")

	// El estándar requiere comillas dobles alrededor de la URL
	soapActionHeader := fmt.Sprintf("\"%s/%s\"", cleanBase, cleanAction)
	req.Header.Set("SOAPAction", soapActionHeader)

	// 7. Send
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("soap call network error: %w", err)
	}
	defer resp.Body.Close()

	// 8. Parse
	respMap, err := MapXML(resp.Body) // Returns *OrderedMap now
	if err != nil {
		return nil, fmt.Errorf("failed to parse response (status %d): %w", resp.StatusCode, err)
	}

	// 9. Fault Handling
	if resp.StatusCode != http.StatusOK {
		if fault, _ := Query(respMap, "Envelope/Body/Fault"); fault != nil {
			if fMap, ok := fault.(*OrderedMap); ok {
				return nil, fmt.Errorf("SOAP Fault %d: [%v] %v", resp.StatusCode, fMap.Get("faultcode"), fMap.Get("faultstring"))
			}
			// Fallback for legacy map (unlikely with MapXML change but good for safety)
			if fMap, ok := fault.(map[string]any); ok {
				return nil, fmt.Errorf("SOAP Fault %d: [%v] %v", resp.StatusCode, fMap["faultcode"], fMap["faultstring"])
			}
		}
		return nil, fmt.Errorf("http error %d", resp.StatusCode)
	}

	return respMap, nil
}
