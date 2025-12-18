package xml

import (
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
		if r.Header.Get("SOAPAction") == "" {
			t.Error("Expected SOAPAction header")
		}

		// 2. Validate Body (Optional: Check if payload is correct)
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "soap:Envelope") {
			t.Error("Expected SOAP Envelope in request body")
		}
		if !strings.Contains(string(body), "m:GetUser") { // Check action key
			t.Error("Expected m:GetUser action in request body")
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

	// Verify Response Parsing (using Query)
	name, err := Query(resp, "Envelope/Body/GetUserResponse/User/Name")
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
