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

// SoapVersion selecciona el envelope/Content-Type usado por Call.
type SoapVersion int

const (
	Soap11 SoapVersion = iota // http://schemas.xmlsoap.org/soap/envelope/, text/xml + header SOAPAction
	Soap12                    // http://www.w3.org/2003/05/soap-envelope, application/soap+xml con action= en Content-Type
)

const (
	soap11EnvelopeNS = "http://schemas.xmlsoap.org/soap/envelope/"
	soap12EnvelopeNS = "http://www.w3.org/2003/05/soap-envelope"
)

// SoapClient permite llamadas dinámicas a servicios SOAP sin structs.
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
	RetryAttempts int           // 0 o 1 = sin reintentos
	RetryBackoff  time.Duration // espera fija entre intentos
}

// --- Opciones mTLS ---

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

// WithSOAPVersion selecciona SOAP 1.1 (default) o SOAP 1.2.
func WithSOAPVersion(v SoapVersion) ClientOption {
	return func(s *SoapClient) { s.Version = v }
}

// WithRetry reintenta la llamada hasta `attempts` veces (con espera fija
// `backoff` entre intentos) cuando el error es de transporte (conexión,
// timeout, DNS). Una respuesta HTTP ya recibida — incluyendo un SOAP Fault
// entregado con status 500 — nunca se reintenta: reintentar no cambia el
// resultado de un error de negocio.
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

// SoapFault representa un <soap:Fault> tipado (SOAP 1.1 faultcode/
// faultstring o SOAP 1.2 Code/Reason), en vez de un error genérico —
// permite usar errors.As para inspeccionar Code/Message/Detail.
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

// extractSoapFault busca un soap:Fault en la respuesta parseada, soportando
// tanto la forma SOAP 1.1 (faultcode/faultstring/faultactor/detail) como la
// forma SOAP 1.2 (Code/Value, Reason/Text, Detail). Devuelve nil si no hay
// Fault reconocible.
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
	envelopeNS := soap11EnvelopeNS
	if c.Version == Soap12 {
		envelopeNS = soap12EnvelopeNS
	}
	envelopeMap := NewMap()
	envelopeMap.Put("@xmlns:soap", envelopeNS)

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

// Call ejecuta una acción SOAP.
// payload puede ser *OrderedMap (respeta orden) o map[string]any (ordena alfabéticamente).
// SOAPAction se reconstruye como "namespace/action" (o SoapActionBase/action) —
// una convención que no coincide con el soapAction real de muchos servicios.
// Si tenés el WSDL, usá CallOperation para el valor exacto.
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

// CallOperation ejecuta action usando el soapAction, endpoint y versión SOAP
// exactos declarados en w (en vez de la convención adivinada de Call).
// Devuelve error si action no existe en el WSDL.
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

// NewSoapClientFromWSDL construye un SoapClient usando el primer endpoint
// SOAP declarado en w (ver WSDL.Endpoint) — namespace y versión incluidos.
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
