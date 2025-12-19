package xml

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSoapClient_PayloadOrder(t *testing.T) {
	// Mock Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		bodyStr := string(bodyBytes)

		// Verification: CUIT must appear BEFORE Concepto
		cuitIdx := strings.Index(bodyStr, "<CUIT>20123456789</CUIT>")
		conIdx := strings.Index(bodyStr, "<Concepto>1</Concepto>") // assuming simple serialization

		if cuitIdx == -1 {
			t.Error("CUIT not found in request")
		}
		if conIdx == -1 {
			t.Error("Concepto not found in request")
		}

		if cuitIdx > conIdx {
			t.Errorf("Order violation! CUIT (pos %d) should be before Concepto (pos %d)", cuitIdx, conIdx)
		}

		// Mock Response
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(`<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body><Response>OK</Response></soap:Body></soap:Envelope>`))
	}))
	defer ts.Close()

	client := NewSoapClient(ts.URL, "http://test.com")

	// Create Ordered Payload
	payload := NewMap()
	payload.Put("CUIT", "20123456789")
	payload.Put("Concepto", "1")

	// Call
	_, err := client.Call("Process", payload)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
}

// Helper for looking into the body manually if needed or just string matching
func TestSoapClient_WSSecurity(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		bodyStr := string(bodyBytes)

		if !strings.Contains(bodyStr, "wsse:Security") {
			t.Error("Missing WS-Security header")
		}
		if !strings.Contains(bodyStr, "wsse:Username") {
			t.Error("Missing Username")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<root><ok/></root>`))
	}))
	defer ts.Close()

	client := NewSoapClient(ts.URL, "ns", WithWSSecurity("user", "pass"))
	payload := NewMap()
	client.Call("Test", payload)
}
