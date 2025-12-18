package xml

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

	// Verify error message content
	wantErr := "SOAP Fault 500: [soap:Client] Invalid ID"
	if err.Error() != wantErr {
		t.Errorf("Error = %q, want %q", err.Error(), wantErr)
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
