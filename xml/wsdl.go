package xml

import (
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
)

// ============================================================================
// WSDL 1.1 PARSER (Discovery & Validation, not codegen)
//
// Scope, deliberately: WSDL 1.1 only (not 2.0), single file (no
// wsdl:import/xsd:import/xsd:include), no XSD type modeling (message parts
// expose only their resolved local element/type name), and QName
// cross-references between sections (binding/@type, port/@binding,
// input|output/@message, part/@element|@type) are resolved by LOCAL NAME
// ONLY against this document's own definitions — valid for the common
// single-targetNamespace case, documented as a known limitation otherwise.
//
// The one place namespace resolution is NOT simplified: telling apart a
// soap:, soap12: or non-SOAP (e.g. http:) binding/port. That's done with
// namespace-qualified encoding/xml struct tags below, because guessing that
// from local names alone would misdetect a non-SOAP port as SOAP 1.1.
// ============================================================================

// --- Raw wire structs (unexported: encoding/xml struct-tag unmarshal) -----

type wsdlAddress struct {
	Location string `xml:"location,attr"`
}

type wsdlSoapBindingInfo struct {
	Style string `xml:"style,attr"`
}

type wsdlSoapOperationInfo struct {
	SOAPAction string `xml:"soapAction,attr"`
}

type wsdlPart struct {
	Name    string `xml:"name,attr"`
	Element string `xml:"element,attr"`
	Type    string `xml:"type,attr"`
}

type wsdlMessage struct {
	Name  string     `xml:"name,attr"`
	Parts []wsdlPart `xml:"part"`
}

type wsdlMessageRef struct {
	Message string `xml:"message,attr"`
}

type wsdlPortTypeOperation struct {
	Name   string          `xml:"name,attr"`
	Input  *wsdlMessageRef `xml:"input"`
	Output *wsdlMessageRef `xml:"output"`
}

type wsdlPortType struct {
	Name       string                  `xml:"name,attr"`
	Operations []wsdlPortTypeOperation `xml:"operation"`
}

type wsdlBindingOperation struct {
	Name     string                 `xml:"name,attr"`
	SoapOp   *wsdlSoapOperationInfo `xml:"http://schemas.xmlsoap.org/wsdl/soap/ operation"`
	Soap12Op *wsdlSoapOperationInfo `xml:"http://schemas.xmlsoap.org/wsdl/soap12/ operation"`
}

type wsdlBinding struct {
	Name          string                 `xml:"name,attr"`
	Type          string                 `xml:"type,attr"`
	SoapBinding   *wsdlSoapBindingInfo   `xml:"http://schemas.xmlsoap.org/wsdl/soap/ binding"`
	Soap12Binding *wsdlSoapBindingInfo   `xml:"http://schemas.xmlsoap.org/wsdl/soap12/ binding"`
	Operations    []wsdlBindingOperation `xml:"operation"`
}

type wsdlPort struct {
	Name          string       `xml:"name,attr"`
	Binding       string       `xml:"binding,attr"`
	SoapAddress   *wsdlAddress `xml:"http://schemas.xmlsoap.org/wsdl/soap/ address"`
	Soap12Address *wsdlAddress `xml:"http://schemas.xmlsoap.org/wsdl/soap12/ address"`
}

type wsdlService struct {
	Name  string     `xml:"name,attr"`
	Ports []wsdlPort `xml:"port"`
}

type wsdlDefinitions struct {
	XMLName         xml.Name       `xml:"definitions"`
	TargetNamespace string         `xml:"targetNamespace,attr"`
	Messages        []wsdlMessage  `xml:"message"`
	PortTypes       []wsdlPortType `xml:"portType"`
	Bindings        []wsdlBinding  `xml:"binding"`
	Services        []wsdlService  `xml:"service"`
}

// --- Public, resolved API --------------------------------------------------

// WSDLPart is one <part> of a WSDL message: a name plus its resolved
// (local-name-only) element or type reference. Informational only — no XSD
// type tree is built.
type WSDLPart struct {
	Name    string
	Element string
	Type    string
}

// WSDLOperation is a fully cross-referenced, callable SOAP operation
// discovered from a WSDL: the exact soapAction and endpoint to use, instead
// of SoapClient.Call's guessed "namespace/action" convention.
type WSDLOperation struct {
	Name        string
	SOAPAction  string
	Endpoint    string
	Namespace   string
	Version     SoapVersion
	InputParts  []WSDLPart
	OutputParts []WSDLPart
}

// WSDL is a parsed, cross-referenced WSDL 1.1 document.
type WSDL struct {
	operations []WSDLOperation
}

// ParseWSDL parses a WSDL 1.1 document and resolves its cross-references
// (binding -> portType -> message) into a flat list of callable operations.
func ParseWSDL(r io.Reader) (*WSDL, error) {
	var defs wsdlDefinitions
	if err := xml.NewDecoder(r).Decode(&defs); err != nil {
		return nil, fmt.Errorf("wsdl: parse error: %w", err)
	}

	messages := make(map[string]wsdlMessage, len(defs.Messages))
	for _, m := range defs.Messages {
		messages[m.Name] = m
	}
	portTypes := make(map[string]wsdlPortType, len(defs.PortTypes))
	for _, pt := range defs.PortTypes {
		portTypes[pt.Name] = pt
	}
	bindings := make(map[string]wsdlBinding, len(defs.Bindings))
	for _, b := range defs.Bindings {
		bindings[localName(b.Name)] = b
	}

	resolveParts := func(msgRef string) ([]WSDLPart, error) {
		if msgRef == "" {
			return nil, nil
		}
		msg, ok := messages[localName(msgRef)]
		if !ok {
			return nil, fmt.Errorf("message %q not found", msgRef)
		}
		parts := make([]WSDLPart, len(msg.Parts))
		for i, p := range msg.Parts {
			parts[i] = WSDLPart{Name: p.Name, Element: localName(p.Element), Type: localName(p.Type)}
		}
		return parts, nil
	}

	var ops []WSDLOperation
	for _, svc := range defs.Services {
		for _, port := range svc.Ports {
			var endpoint string
			var version SoapVersion
			switch {
			case port.SoapAddress != nil:
				endpoint, version = port.SoapAddress.Location, Soap11
			case port.Soap12Address != nil:
				endpoint, version = port.Soap12Address.Location, Soap12
			default:
				continue // non-SOAP port (e.g. http:address) — not an error, just not ours
			}

			binding, ok := bindings[localName(port.Binding)]
			if !ok {
				return nil, fmt.Errorf("wsdl: port %q references unknown binding %q", port.Name, port.Binding)
			}
			portType, ok := portTypes[localName(binding.Type)]
			if !ok {
				return nil, fmt.Errorf("wsdl: binding %q references unknown portType %q", binding.Name, binding.Type)
			}
			ptOpsByName := make(map[string]wsdlPortTypeOperation, len(portType.Operations))
			for _, o := range portType.Operations {
				ptOpsByName[o.Name] = o
			}

			for _, bindOp := range binding.Operations {
				ptOp, ok := ptOpsByName[bindOp.Name]
				if !ok {
					return nil, fmt.Errorf("wsdl: binding operation %q has no matching portType operation", bindOp.Name)
				}

				soapAction := ""
				if bindOp.SoapOp != nil {
					soapAction = bindOp.SoapOp.SOAPAction
				} else if bindOp.Soap12Op != nil {
					soapAction = bindOp.Soap12Op.SOAPAction
				}

				var inputMsg, outputMsg string
				if ptOp.Input != nil {
					inputMsg = ptOp.Input.Message
				}
				if ptOp.Output != nil {
					outputMsg = ptOp.Output.Message
				}
				inParts, err := resolveParts(inputMsg)
				if err != nil {
					return nil, fmt.Errorf("wsdl: operation %q input: %w", bindOp.Name, err)
				}
				outParts, err := resolveParts(outputMsg)
				if err != nil {
					return nil, fmt.Errorf("wsdl: operation %q output: %w", bindOp.Name, err)
				}

				ops = append(ops, WSDLOperation{
					Name:        bindOp.Name,
					SOAPAction:  soapAction,
					Endpoint:    endpoint,
					Namespace:   defs.TargetNamespace,
					Version:     version,
					InputParts:  inParts,
					OutputParts: outParts,
				})
			}
		}
	}

	return &WSDL{operations: ops}, nil
}

// localName strips a "prefix:" from a QName-shaped string, matching by local
// name only (see the scope note at the top of this file).
func localName(qname string) string {
	if i := strings.IndexByte(qname, ':'); i >= 0 {
		return qname[i+1:]
	}
	return qname
}

// Operations returns every discovered SOAP operation, in document order.
// Multiple services/ports for the same operation name (a common pattern:
// one port per SOAP version) all appear — see Operation for how the first
// match is picked.
func (w *WSDL) Operations() []WSDLOperation {
	out := make([]WSDLOperation, len(w.operations))
	copy(out, w.operations)
	return out
}

// Operation looks up a single operation by name. If multiple ports expose
// an operation with the same name (e.g. a SOAP 1.1 and a SOAP 1.2 port for
// the same service), the first one found is returned — deterministic
// (document order) but not configurable in this version.
func (w *WSDL) Operation(name string) (*WSDLOperation, error) {
	for i := range w.operations {
		if w.operations[i].Name == name {
			op := w.operations[i]
			return &op, nil
		}
	}
	names := make([]string, 0, len(w.operations))
	seen := map[string]bool{}
	for _, op := range w.operations {
		if !seen[op.Name] {
			seen[op.Name] = true
			names = append(names, op.Name)
		}
	}
	sort.Strings(names)
	return nil, fmt.Errorf("wsdl: operation %q not found; available: %s", name, strings.Join(names, ", "))
}

// Endpoint returns the first discovered SOAP endpoint. Returns an error if
// the WSDL has no SOAP port at all.
func (w *WSDL) Endpoint() (string, error) {
	if len(w.operations) == 0 {
		return "", fmt.Errorf("wsdl: no SOAP endpoint found")
	}
	return w.operations[0].Endpoint, nil
}
