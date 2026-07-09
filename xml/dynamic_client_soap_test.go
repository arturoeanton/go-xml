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
		// Validate that the SOAPAction is quoted (recent fix)
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

		// --- FIX HERE ---
		// We no longer use the "m:" prefix; we use the clean tag with the default namespace.
		if !strings.Contains(body, "<GetUser") {
			t.Error("Expected <GetUser> tag in request body")
		}
		// Validate that the namespace is injected correctly into the action node
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
	// Note: Depending on how your parser handles namespaces in the response,
	// the query sometimes needs adjusting. With standard MapXML this should work:
	name, err := Query(resp, "soap:Envelope/soap:Body/GetUserResponse/User/Name")
	if err != nil {
		// Fallback: Sometimes the parser simplifies keys
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
	if want := "SOAP fault [soap:Client]: Invalid ID"; fault.Error() != want {
		t.Errorf("fault.Error() = %q, want %q", fault.Error(), want)
	}
}

func TestSoapFault_Error_IncludesActor(t *testing.T) {
	fault := &SoapFault{Code: "soap:Server", Message: "boom", Actor: "http://example.org/actor"}
	want := "SOAP fault [soap:Server] (actor=http://example.org/actor): boom"
	if fault.Error() != want {
		t.Errorf("Error() = %q, want %q", fault.Error(), want)
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

func TestSoapClient_InsecureSkipVerify(t *testing.T) {
	// httptest.NewTLSServer serves a self-signed cert that no default trust
	// store will accept — the point of this test.
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<soap:Envelope><soap:Body><ok/></soap:Body></soap:Envelope>`)
	}))
	defer ts.Close()

	secure := NewSoapClient(ts.URL, "http://ns")
	if _, err := secure.Call("Action", nil); err == nil {
		t.Fatal("expected a certificate validation error without WithInsecureSkipVerify, got nil")
	}

	insecure := NewSoapClient(ts.URL, "http://ns", WithInsecureSkipVerify())
	if _, err := insecure.Call("Action", nil); err != nil {
		t.Fatalf("expected WithInsecureSkipVerify to accept the self-signed cert, got: %v", err)
	}
}

func TestSoapClient_CallOperation_UsesExactSOAPAction(t *testing.T) {
	wsdl, err := ParseWSDL(strings.NewReader(testWSDL))
	if err != nil {
		t.Fatalf("ParseWSDL error: %v", err)
	}

	var gotSOAPAction string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSOAPAction = r.Header.Get("SOAPAction")
		fmt.Fprint(w, `<soap:Envelope><soap:Body><ok/></soap:Body></soap:Envelope>`)
	}))
	defer ts.Close()

	client := NewSoapClient(ts.URL, "http://ns")
	if _, err := client.CallOperation(wsdl, "GetTemperature", nil); err != nil {
		t.Fatalf("CallOperation error: %v", err)
	}

	want := `"http://example.org/weather/GetTemperature"`
	if gotSOAPAction != want {
		t.Errorf("SOAPAction = %q, want the WSDL's exact value %q (not the guessed namespace/action one)", gotSOAPAction, want)
	}
}

func TestSoapClient_CallOperation_EmptySOAPAction(t *testing.T) {
	wsdl, err := ParseWSDL(strings.NewReader(testWSDL))
	if err != nil {
		t.Fatalf("ParseWSDL error: %v", err)
	}

	var gotSOAPAction string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// r.Header.Get returns "" both if the header is absent AND if its
		// value is empty — but we set the value to the literal 2-character
		// string `""` (quote-quote), which IS what we're asserting on here.
		gotSOAPAction = r.Header.Get("SOAPAction")
		fmt.Fprint(w, `<soap:Envelope><soap:Body><ok/></soap:Body></soap:Envelope>`)
	}))
	defer ts.Close()

	client := NewSoapClient(ts.URL, "http://ns")
	if _, err := client.CallOperation(wsdl, "GetStatus", nil); err != nil {
		t.Fatalf("CallOperation error: %v", err)
	}

	if gotSOAPAction != `""` {
		t.Errorf("SOAPAction = %q, want literal empty quotes %q (never a bare/omitted header)", gotSOAPAction, `""`)
	}
}

func TestSoapClient_CallOperation_UnknownAction(t *testing.T) {
	wsdl, err := ParseWSDL(strings.NewReader(testWSDL))
	if err != nil {
		t.Fatalf("ParseWSDL error: %v", err)
	}

	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer ts.Close()

	client := NewSoapClient(ts.URL, "http://ns")
	if _, err := client.CallOperation(wsdl, "NoSuchAction", nil); err == nil {
		t.Fatal("expected an error for an action not present in the WSDL, got nil")
	}
	if called {
		t.Error("CallOperation should validate the action before making any HTTP request")
	}
}

func TestNewSoapClientFromWSDL(t *testing.T) {
	wsdl, err := ParseWSDL(strings.NewReader(testWSDL))
	if err != nil {
		t.Fatalf("ParseWSDL error: %v", err)
	}

	client, err := NewSoapClientFromWSDL(wsdl)
	if err != nil {
		t.Fatalf("NewSoapClientFromWSDL error: %v", err)
	}

	if client.EndpointURL != "http://weather.example.org/soap11" {
		t.Errorf("EndpointURL = %q, want the WSDL's first SOAP port", client.EndpointURL)
	}
	if client.Namespace != "http://example.org/weather" {
		t.Errorf("Namespace = %q, want the WSDL's target namespace", client.Namespace)
	}
	if client.Version != Soap11 {
		t.Errorf("Version = %v, want Soap11 (matching the first port)", client.Version)
	}
}
