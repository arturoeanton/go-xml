package xml

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSoapClient_Success(t *testing.T) {
	// Mock Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Validate Headers
		if r.Header.Get("Content-Type") != "text/xml; charset=utf-8" {
			t.Errorf("Expected Content-Type text/xml, got %s", r.Header.Get("Content-Type"))
		}
		// Validar que el SOAPAction tenga comillas (Fix reciente)
		soapAction := r.Header.Get("SOAPAction")
		if !strings.Contains(soapAction, "\"") {
			t.Errorf("Expected quoted SOAPAction header, got %s", soapAction)
		}

		// 2. Validate Body
		bodyBytes, _ := io.ReadAll(r.Body)
		body := string(bodyBytes)

		if !strings.Contains(body, "soap:Envelope") {
			t.Error("Expected SOAP Envelope in request body")
		}

		// --- CORRECCIÓN AQUÍ ---
		// Ya no usamos el prefijo "m:", usamos el tag limpio con el namespace por defecto.
		if !strings.Contains(body, "<GetUser") {
			t.Error("Expected <GetUser> tag in request body")
		}
		// Validamos que se inyecte el namespace correctamente en el nodo de acción
		if !strings.Contains(body, `xmlns="http://example.org/myservice"`) {
			t.Errorf("Expected namespace in action node. Got body: %s", body)
		}

		// 3. Return Success XML
		respXML := `
        <soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
            <soap:Body>
                <GetUserResponse>
                    <User>
                        <ID>123</ID>
                        <Name>Alice</Name>
                    </User>
                </GetUserResponse>
            </soap:Body>
        </soap:Envelope>`
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, respXML)
	}))
	defer ts.Close()

	// Client
	client := NewSoapClient(ts.URL, "http://example.org/myservice")

	// Call
	payload := map[string]any{
		"ID": 123,
	}

	resp, err := client.Call("GetUser", payload)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	// Verify Response Parsing
	// Nota: Dependiendo de cómo tu parser maneje namespaces en la respuesta,
	// a veces es necesario ajustar la query. Si tienes MapXML estándar, esto debería funcionar:
	name, err := Query(resp, "soap:Envelope/soap:Body/GetUserResponse/User/Name")
	if err != nil {
		// Fallback: A veces el parser simplifica keys
		name, err = Query(resp, "Envelope/Body/GetUserResponse/User/Name")
	}

	if err != nil {
		t.Fatalf("Query response failed: %v", err)
	}
	if name != "Alice" {
		t.Errorf("Expected name Alice, got %v", name)
	}
}

func TestSoapClient_Fault(t *testing.T) {
	// Mock Server Error
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respXML := `
		<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
			<soap:Body>
				<soap:Fault>
					<faultcode>soap:Client</faultcode>
					<faultstring>Invalid ID</faultstring>
				</soap:Fault>
			</soap:Body>
		</soap:Envelope>`
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, respXML)
	}))
	defer ts.Close()

	client := NewSoapClient(ts.URL, "http://example.org/myservice")
	payload := map[string]any{"ID": -1}

	_, err := client.Call("GetUser", payload)
	if err == nil {
		t.Fatal("Expected error for SOAP Fault, got nil")
	}

	var fault *SoapFault
	if !errors.As(err, &fault) {
		t.Fatalf("expected a *SoapFault, got %T: %v", err, err)
	}
	if fault.Code != "soap:Client" {
		t.Errorf("fault.Code = %q, want %q", fault.Code, "soap:Client")
	}
	if fault.Message != "Invalid ID" {
		t.Errorf("fault.Message = %q, want %q", fault.Message, "Invalid ID")
	}
}

func TestSoapClient_Auth(t *testing.T) {
	// 1. Basic Auth
	t.Run("BasicAuth", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
			if auth != expected {
				t.Errorf("Expected basic auth header %q, got %q", expected, auth)
			}
			fmt.Fprint(w, `<soap:Envelope><soap:Body></soap:Body></soap:Envelope>`)
		}))
		defer ts.Close()

		client := NewSoapClient(ts.URL, "http://ns", WithBasicAuth("user", "pass"))
		client.Call("Action", nil)
	})

	// 2. Bearer Token
	t.Run("BearerToken", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			expected := "Bearer mytoken123"
			if auth != expected {
				t.Errorf("Expected bearer header %q, got %q", expected, auth)
			}
			fmt.Fprint(w, `<soap:Envelope><soap:Body></soap:Body></soap:Envelope>`)
		}))
		defer ts.Close()

		client := NewSoapClient(ts.URL, "http://ns", WithBearerToken("mytoken123"))
		client.Call("Action", nil)
	})

	// 3. WS-Security
	t.Run("WSSecurity", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			sBody := string(body)

			// Simple checks for XML structure presence
			checks := []string{
				"<soap:Header>",
				"wsse:Security",
				"wsse:Username>admin</wsse:Username>",
				"wsse:Password",
				"secret123",
			}
			for _, check := range checks {
				if !strings.Contains(sBody, check) {
					t.Errorf("Expected WS-Security XML to contain %q, but it didn't. Body:\n%s", check, sBody)
				}
			}
			fmt.Fprint(w, `<soap:Envelope><soap:Body></soap:Body></soap:Envelope>`)
		}))
		defer ts.Close()

		client := NewSoapClient(ts.URL, "http://ns", WithWSSecurity("admin", "secret123"))
		client.Call("Action", nil)
	})
}

func TestSoapClient_Soap12(t *testing.T) {
	var gotContentType, gotSOAPAction string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotSOAPAction = r.Header.Get("SOAPAction")
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "http://www.w3.org/2003/05/soap-envelope") {
			t.Errorf("expected SOAP 1.2 envelope namespace in request body, got: %s", body)
		}
		fmt.Fprint(w, `<soap:Envelope><soap:Body><ok/></soap:Body></soap:Envelope>`)
	}))
	defer ts.Close()

	client := NewSoapClient(ts.URL, "http://example.org/svc", WithSOAPVersion(Soap12))
	if _, err := client.Call("DoThing", nil); err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	if !strings.HasPrefix(gotContentType, "application/soap+xml") {
		t.Errorf("Content-Type = %q, want application/soap+xml prefix", gotContentType)
	}
	if !strings.Contains(gotContentType, `action="http://example.org/svc/DoThing"`) {
		t.Errorf("Content-Type missing action parameter: %q", gotContentType)
	}
	if gotSOAPAction != "" {
		t.Errorf("SOAP 1.2 requests should not set a separate SOAPAction header, got %q", gotSOAPAction)
	}
}

// hijackAndClose simulates a transient transport-level failure (as opposed
// to a valid HTTP response like a Fault): it grabs the raw connection and
// closes it without writing anything, which surfaces to the client as a
// network error, not an HTTP status.
func hijackAndClose(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	hj, ok := w.(http.Hijacker)
	if !ok {
		t.Fatal("ResponseWriter does not support hijacking")
	}
	conn, _, err := hj.Hijack()
	if err != nil {
		t.Fatalf("hijack failed: %v", err)
	}
	conn.Close()
}

func TestSoapClient_WithRetry_RecoversFromTransientNetworkError(t *testing.T) {
	var callCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&callCount, 1) == 1 {
			hijackAndClose(t, w)
			return
		}
		fmt.Fprint(w, `<soap:Envelope><soap:Body><ok/></soap:Body></soap:Envelope>`)
	}))
	defer ts.Close()

	client := NewSoapClient(ts.URL, "http://ns", WithRetry(3, 5*time.Millisecond))
	if _, err := client.Call("Action", nil); err != nil {
		t.Fatalf("expected retry to recover from a transient network error, got: %v", err)
	}
	if got := atomic.LoadInt32(&callCount); got != 2 {
		t.Errorf("expected exactly 2 attempts (1 failure + 1 success), got %d", got)
	}
}

func TestSoapClient_WithRetry_FailsAfterExhaustingAttempts(t *testing.T) {
	var callCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		hijackAndClose(t, w)
	}))
	defer ts.Close()

	client := NewSoapClient(ts.URL, "http://ns", WithRetry(3, 1*time.Millisecond))
	if _, err := client.Call("Action", nil); err == nil {
		t.Fatal("expected error after exhausting retry attempts, got nil")
	}
	if got := atomic.LoadInt32(&callCount); got != 3 {
		t.Errorf("expected exactly 3 attempts, got %d", got)
	}
}

func TestSoapClient_WithRetry_DoesNotRetryOnFault(t *testing.T) {
	var callCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `<soap:Envelope><soap:Body><soap:Fault><faultcode>soap:Client</faultcode><faultstring>bad request</faultstring></soap:Fault></soap:Body></soap:Envelope>`)
	}))
	defer ts.Close()

	client := NewSoapClient(ts.URL, "http://ns", WithRetry(3, 1*time.Millisecond))
	_, err := client.Call("Action", nil)
	if err == nil {
		t.Fatal("expected a fault error, got nil")
	}
	if callCount != 1 {
		t.Errorf("expected exactly 1 call (faults are not retried), got %d", callCount)
	}
}
