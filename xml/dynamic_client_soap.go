package xml

import (
	"bytes"
	"crypto/tls"
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

	AuthType     string
	AuthUsername string
	AuthPassword string
	AuthToken    string

	// --- mTLS Config (NUEVO) ---
	CertFile string
	KeyFile  string
	Insecure bool // Skip Verify
}

// --- Nuevas Opciones mTLS ---

// WithClientCertificate habilita mTLS usando archivos PEM (.crt y .key).
func WithClientCertificate(certFile, keyFile string) ClientOption {
	return func(s *SoapClient) {
		s.CertFile = certFile
		s.KeyFile = keyFile
	}
}

// WithInsecureSkipVerify salta la validación del certificado del servidor.
// Útil para desarrollo (Self-signed).
func WithInsecureSkipVerify() ClientOption {
	return func(s *SoapClient) { s.Insecure = true }
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
		Headers:     make(map[string]string),
		AuthType:    AuthNone,
	}
	for _, opt := range opts {
		opt(client)
	}

	// === LÓGICA mTLS ===
	tlsConfig := &tls.Config{}
	hasTlsConfig := false

	// 1. Cargar Certificado Cliente (PEM)
	if client.CertFile != "" && client.KeyFile != "" {
		// Usamos nuestra función wrapper de cert.go
		cert, err := LoadCert(client.CertFile, client.KeyFile)
		if err != nil {
			// Advertencia crítica, pero no panic (para no tumbar apps enteras)
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

	// 3. Aplicar Transporte
	if hasTlsConfig {
		// Clonamos el transporte por defecto para no perder configuraciones de Proxy (si hubiera)
		// Pero como estamos creando uno nuevo, usamos el Transport básico + TLS
		client.HttpClient.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	}

	return client
}

// Call ejecuta una acción SOAP.
// payload puede ser *OrderedMap (respeta orden) o map[string]any (ordena alfabéticamente).
func (c *SoapClient) Call(action string, payload any) (*OrderedMap, error) {
	// 1. Preparar el Payload
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

	// 2. Construir Envelope Base
	envelopeMap := NewMap()
	envelopeMap.Put("@xmlns:soap", "http://schemas.xmlsoap.org/soap/envelope/")

	// 3. Inyectar WS-Security (Si aplica) - Headers van ANTES del Body
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

	// 6. Create Request
	req, err := http.NewRequest("POST", c.EndpointURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Headers Base
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("User-Agent", "r2-xml-client/2.0")

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

	// SOAPAction Logic
	base := c.Namespace
	if c.SoapActionBase != "" {
		base = c.SoapActionBase
	}
	cleanBase := strings.TrimSuffix(base, "/")
	cleanAction := strings.TrimPrefix(action, "/")
	soapActionHeader := fmt.Sprintf("\"%s/%s\"", cleanBase, cleanAction)
	req.Header.Set("SOAPAction", soapActionHeader)

	// 7. Send
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("soap call network error: %w", err)
	}
	defer resp.Body.Close()

	// 8. Parse
	respMap, err := MapXML(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response (status %d): %w", resp.StatusCode, err)
	}

	// 9. Fault Handling
	if resp.StatusCode != http.StatusOK {
		// Intentamos navegar al Fault. La estructura suele ser Envelope/Body/Fault
		// Pero al usar MapXML sin unwrapper manual, la root key "soap:Envelope" encapsula todo.
		// MapXML retorna el objeto root directamente? NO. Revisitando MapXML:
		// "stack := []*node{{tagName: "", data: root}}" -> devuelve root.
		// El root tiene una key "soap:Envelope".
		// Query debería ser "soap:Envelope/soap:Body/soap:Fault" (con nombres reales devueltos por MapXML).
		// MapXML no limpia namespaces automáticamente en claves, solo en valores hooks.
		// Las claves retienen "wsse:..." si no hay alias?
		// "tagName := resolveName(...)". Si no hay alias, usa local? No.
		// "resolveName": if match -> alias:local. Else -> local.
		// Espera, xml.Name es {Space, Local}.
		// resolveName: "return name.Local" if no match.
		// Entonces las claves son LOCAL NAMES. "Envelope", "Body", "Fault".
		// EXCEPTO si registramos namespaces. En default config map is empty.
		// Entonces claves son "Envelope", "Body", "Fault".

		fault, _ := Query(respMap, "Envelope/Body/Fault")
		if fault != nil {
			if fMap, ok := fault.(*OrderedMap); ok {
				return nil, fmt.Errorf("SOAP Fault %d: [%v] %v", resp.StatusCode, fMap.Get("faultcode"), fMap.Get("faultstring"))
			}
		}
		return nil, fmt.Errorf("http error %d", resp.StatusCode)
	}

	return respMap, nil
}
