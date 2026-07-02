package xml

import (
	"strings"
	"testing"
)

// testWSDL is hand-authored, representative of a typical ASMX/.NET-generated
// WSDL: one service exposing the same two operations over both a SOAP 1.1
// and a SOAP 1.2 port (a very common real-world pattern), plus a third,
// non-SOAP http: port mixed in to confirm it's skipped rather than
// misdetected. GetTemperature has an explicit soapAction; GetStatus has an
// empty one (also common, and must round-trip as "" not be dropped).
const testWSDL = `<?xml version="1.0" encoding="UTF-8"?>
<definitions name="WeatherService"
	targetNamespace="http://example.org/weather"
	xmlns:tns="http://example.org/weather"
	xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/"
	xmlns:soap12="http://schemas.xmlsoap.org/wsdl/soap12/"
	xmlns:http="http://schemas.xmlsoap.org/wsdl/http/"
	xmlns="http://schemas.xmlsoap.org/wsdl/">

	<message name="GetTemperatureRequest">
		<part name="city" type="xsd:string"/>
	</message>
	<message name="GetTemperatureResponse">
		<part name="temperature" type="xsd:float"/>
	</message>
	<message name="GetStatusRequest">
		<part name="parameters" element="tns:GetStatusElement"/>
	</message>
	<message name="GetStatusResponse">
		<part name="parameters" element="tns:GetStatusResponseElement"/>
	</message>

	<portType name="WeatherPortType">
		<operation name="GetTemperature">
			<input message="tns:GetTemperatureRequest"/>
			<output message="tns:GetTemperatureResponse"/>
		</operation>
		<operation name="GetStatus">
			<input message="tns:GetStatusRequest"/>
			<output message="tns:GetStatusResponse"/>
		</operation>
	</portType>

	<binding name="WeatherSoap11Binding" type="tns:WeatherPortType">
		<soap:binding style="document" transport="http://schemas.xmlsoap.org/soap/http"/>
		<operation name="GetTemperature">
			<soap:operation soapAction="http://example.org/weather/GetTemperature"/>
			<input><soap:body use="literal"/></input>
			<output><soap:body use="literal"/></output>
		</operation>
		<operation name="GetStatus">
			<soap:operation soapAction=""/>
			<input><soap:body use="literal"/></input>
			<output><soap:body use="literal"/></output>
		</operation>
	</binding>

	<binding name="WeatherSoap12Binding" type="tns:WeatherPortType">
		<soap12:binding style="document" transport="http://schemas.xmlsoap.org/soap/http"/>
		<operation name="GetTemperature">
			<soap12:operation soapAction="http://example.org/weather/GetTemperature"/>
			<input><soap12:body use="literal"/></input>
			<output><soap12:body use="literal"/></output>
		</operation>
		<operation name="GetStatus">
			<soap12:operation soapAction=""/>
			<input><soap12:body use="literal"/></input>
			<output><soap12:body use="literal"/></output>
		</operation>
	</binding>

	<binding name="WeatherHttpBinding" type="tns:WeatherPortType">
		<http:binding verb="GET"/>
	</binding>

	<service name="WeatherService">
		<port name="WeatherSoap11" binding="tns:WeatherSoap11Binding">
			<soap:address location="http://weather.example.org/soap11"/>
		</port>
		<port name="WeatherSoap12" binding="tns:WeatherSoap12Binding">
			<soap12:address location="http://weather.example.org/soap12"/>
		</port>
		<port name="WeatherHttp" binding="tns:WeatherHttpBinding">
			<http:address location="http://weather.example.org/http"/>
		</port>
	</service>
</definitions>`

func mustParseTestWSDL(t *testing.T) *WSDL {
	t.Helper()
	w, err := ParseWSDL(strings.NewReader(testWSDL))
	if err != nil {
		t.Fatalf("ParseWSDL error: %v", err)
	}
	return w
}

func TestParseWSDL_OperationCount(t *testing.T) {
	w := mustParseTestWSDL(t)
	ops := w.Operations()

	// 2 operations x 2 SOAP ports (11 and 12) = 4. The http: port
	// contributes zero — it must be skipped, not misdetected.
	if len(ops) != 4 {
		t.Fatalf("expected 4 operations (2 ops x 2 SOAP ports), got %d: %+v", len(ops), ops)
	}
}

func TestParseWSDL_SkipsNonSOAPPort(t *testing.T) {
	w := mustParseTestWSDL(t)
	for _, op := range w.Operations() {
		if op.Endpoint == "http://weather.example.org/http" {
			t.Errorf("non-SOAP http: port leaked into Operations(): %+v", op)
		}
	}
}

func TestParseWSDL_VersionDetection(t *testing.T) {
	w := mustParseTestWSDL(t)
	for _, op := range w.Operations() {
		switch op.Endpoint {
		case "http://weather.example.org/soap11":
			if op.Version != Soap11 {
				t.Errorf("operation %s on soap11 port: Version = %v, want Soap11", op.Name, op.Version)
			}
		case "http://weather.example.org/soap12":
			if op.Version != Soap12 {
				t.Errorf("operation %s on soap12 port: Version = %v, want Soap12", op.Name, op.Version)
			}
		default:
			t.Errorf("unexpected endpoint: %s", op.Endpoint)
		}
	}
}

func TestWSDL_Operation_Found(t *testing.T) {
	w := mustParseTestWSDL(t)

	// GetTemperature appears on both ports; Operation() picks the first
	// (document order = SOAP 1.1 port here).
	op, err := w.Operation("GetTemperature")
	if err != nil {
		t.Fatalf("Operation error: %v", err)
	}
	if op.SOAPAction != "http://example.org/weather/GetTemperature" {
		t.Errorf("SOAPAction = %q, want the exact WSDL value", op.SOAPAction)
	}
	if op.Namespace != "http://example.org/weather" {
		t.Errorf("Namespace = %q, want target namespace", op.Namespace)
	}
	if len(op.InputParts) != 1 || op.InputParts[0].Type != "string" {
		t.Errorf("InputParts = %+v, want 1 part with Type=string", op.InputParts)
	}
}

func TestWSDL_Operation_EmptySOAPAction(t *testing.T) {
	w := mustParseTestWSDL(t)
	op, err := w.Operation("GetStatus")
	if err != nil {
		t.Fatalf("Operation error: %v", err)
	}
	if op.SOAPAction != "" {
		t.Errorf("SOAPAction = %q, want empty string (explicit in WSDL)", op.SOAPAction)
	}
	if len(op.InputParts) != 1 || op.InputParts[0].Element != "GetStatusElement" {
		t.Errorf("InputParts = %+v, want 1 part with Element=GetStatusElement", op.InputParts)
	}
}

func TestWSDL_Operation_NotFound(t *testing.T) {
	w := mustParseTestWSDL(t)
	_, err := w.Operation("DoesNotExist")
	if err == nil {
		t.Fatal("expected error for unknown operation, got nil")
	}
	if !strings.Contains(err.Error(), "GetTemperature") || !strings.Contains(err.Error(), "GetStatus") {
		t.Errorf("error should list available operations, got: %v", err)
	}
}

func TestWSDL_Endpoint(t *testing.T) {
	w := mustParseTestWSDL(t)
	ep, err := w.Endpoint()
	if err != nil {
		t.Fatalf("Endpoint error: %v", err)
	}
	if ep != "http://weather.example.org/soap11" {
		t.Errorf("Endpoint() = %q, want the first SOAP port's address", ep)
	}
}

func TestWSDL_Endpoint_NoSOAPPort(t *testing.T) {
	noSoap := `<definitions xmlns="http://schemas.xmlsoap.org/wsdl/"
		xmlns:http="http://schemas.xmlsoap.org/wsdl/http/" targetNamespace="urn:x">
		<service name="S"><port name="P" binding="x:B"><http:address location="http://x/"/></port></service>
	</definitions>`
	w, err := ParseWSDL(strings.NewReader(noSoap))
	if err != nil {
		t.Fatalf("ParseWSDL error: %v", err)
	}
	if _, err := w.Endpoint(); err == nil {
		t.Fatal("expected error for a WSDL with no SOAP port, got nil")
	}
}
