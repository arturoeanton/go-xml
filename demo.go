package main

import (
	"crypto/sha512"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/arturoeanton/go-xml/xml"
)

// ============================================================================
// DEMO REGISTRY
// ============================================================================

// Maps the name given to `demo [name]` to its function
var demoRegistry = map[string]func(){
	// v0.1 - Basics
	"basic": demo_v1_BasicParsing,
	"array": demo_v1_ForceArray,

	// v1.0 - Robustness
	"html": demo_v1_HtmlLenient,

	// v2.0 - Namespaces and Queries
	"ns":    demo_v2_Namespaces,
	"query": demo_v2_QueryAdvanced,

	// v2.1 - Hooks and CDATA
	"hooks": demo_v2_HooksAndTypes,
	"cdata": demo_v2_MarshalCDATA,

	// v1.0 - Enterprise (Streaming & Validation)
	"stream_r": demo_v3_StreamingDecoder,
	"stream_w": demo_v3_StreamingEncoder,
	"validate": demo_v3_Validation,

	// v3.0 - Utilities & Legacy
	"legacy": demo_v3_LegacyCharsets,
	"json":   demo_v3_JSONConversion,

	"soap":  demo_soap,
	"soap2": demo_soap2,

	"dian":  demo_dian,
	"dian2": demo_dian_ubl,
}

// RunDemos: the orchestrator called from main()
func RunDemos(arg string) {
	fmt.Println("========================================")
	fmt.Println("   r2/xml - Historical Demo Gallery")
	fmt.Println("========================================")

	if arg == "all" || arg == "" {
		// Run ALL demos in logical order (not random map order)
		runSequence := []string{
			"basic", "array", "html",
			"ns", "query", "hooks",
			"cdata", "validate",
			"stream_r", "stream_w",
			"legacy", "json",
		}

		for _, name := range runSequence {
			printHeader(name)
			demoRegistry[name]()
			time.Sleep(300 * time.Millisecond)
		}
	} else {
		// Run a single demo
		if fn, exists := demoRegistry[arg]; exists {
			printHeader(arg)
			fn()
		} else {
			fmt.Printf("❌ Demo '%s' not found.\nAvailable demos: %v\n", arg, getDemoKeys())
		}
	}
}

func printHeader(name string) {
	fmt.Printf("\n>>> Running Demo: [%s] <<<\n", strings.ToUpper(name))
	fmt.Println(strings.Repeat("-", 40))
}

func getDemoKeys() []string {
	keys := []string{}
	for k := range demoRegistry {
		keys = append(keys, k)
	}
	return keys
}

// ============================================================================
// DEMOS v0.1 - FUNDAMENTALS
// ============================================================================

func demo_v1_BasicParsing() {
	fmt.Println("Goal: Read simple XML without structs.")

	xmlData := `<library><book id="1">The Little Prince</book></library>`

	// 1. Parse
	m, err := xml.MapXML(strings.NewReader(xmlData))
	if err != nil {
		panic(err)
	}

	title, _ := xml.Query(m, "library/book/#text")
	id, _ := xml.Query(m, "library/book/@id")

	fmt.Printf("Resulting Map: %+v\n", m)
	fmt.Printf("Title: %s (ID: %s)\n", title, id)
}

func demo_v1_ForceArray() {
	fmt.Println("Goal: Resolve JSON ambiguity (Single vs Array).")

	// Tricky case: there is only one book, but the frontend expects a list
	xmlData := `<library><book>Only One</book></library>`

	m, _ := xml.MapXML(strings.NewReader(xmlData), xml.ForceArray("book"))

	// Verify it is []any
	lib := m.Get("library").(*xml.OrderedMap)
	books := lib.Get("book").([]any) // If the cast fails, ForceArray failed

	fmt.Printf("Type of 'book': %T (Length: %d)\n", books, len(books))
}

// ============================================================================
// DEMOS v1.0 - ROBUSTNESS (HTML)
// ============================================================================

func demo_v1_HtmlLenient() {
	fmt.Println("Goal: Read dirty HTML (unclosed tags).")

	// <br> and <meta> have no closing tag
	htmlData := `<html><body>Hello<br>World<br><meta charset="utf-8"></body></html>`

	m, err := xml.MapXML(strings.NewReader(htmlData), xml.EnableExperimental())
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Use Query to verify content was read past the <br> tags
	body, _ := xml.Query(m, "html/body/#text")
	fmt.Printf("Content read successfully: %v\n", body)
}

// ============================================================================
// DEMOS v2.0 - NAMESPACES & QUERY
// ============================================================================

func demo_v2_Namespaces() {
	fmt.Println("Goal: Clean long URLs out of the keys.")

	xmlData := `<root xmlns:h="http://w3.org/html"><h:table>Data</h:table></root>`

	m, _ := xml.MapXML(strings.NewReader(xmlData),
		xml.RegisterNamespace("html", "http://w3.org/html"),
	)

	// Now we can access it as "html:table" instead of the full URL
	tableVal, _ := xml.Query(m, "root/html:table/#text")
	fmt.Printf("Value with clean Namespace: %v\n", tableVal)
}

func demo_v2_QueryAdvanced() {
	fmt.Println("Goal: Deep, iterative search (QueryAll).")

	xmlData := `
	<store>
		<section><item>A</item><item>B</item></section>
		<section><item>C</item></section>
	</store>`

	m, _ := xml.MapXML(strings.NewReader(xmlData), xml.ForceArray("section", "item"))

	// QueryAll flattens intermediate arrays (section -> item)
	items, _ := xml.QueryAll(m, "store/section/item/#text")

	fmt.Printf("Items found (3 expected): %v\n", items)
}

// ============================================================================
// DEMOS v2.1 - HOOKS & MARSHAL PRO
// ============================================================================

func demo_v2_HooksAndTypes() {
	fmt.Println("Goal: Transform strings into Go types (Time/Int) on the fly.")

	xmlData := `<log><date>2025-12-31</date><count>99</count></log>`

	// Hook for dates
	dateHook := func(s string) any {
		t, _ := time.Parse("2006-01-02", s)
		return t
	}

	m, _ := xml.MapXML(strings.NewReader(xmlData),
		xml.WithValueHook("date", dateHook),
		xml.EnableExperimental(), // Infers "99" -> int
	)

	dateVal, _ := xml.Query(m, "log/date")
	countVal, _ := xml.Query(m, "log/count")

	fmt.Printf("Date Type: %T, Value: %v\n", dateVal, dateVal)
	fmt.Printf("Count Type: %T, Value: %v\n", countVal, countVal)
}

func demo_v2_MarshalCDATA() {
	fmt.Println("Goal: Generate XML with CDATA and Comments.")

	data := map[string]any{
		"msg": map[string]any{
			"#comments": []string{" This is raw HTML "},
			"#cdata":    "<b>Bold</b>",
		},
	}

	// Use the Encoder (same logic as Marshal v2)
	fmt.Println("Generated XML:")
	xml.NewEncoder(os.Stdout, xml.WithPrettyPrint()).Encode(data)
	fmt.Println()
}

// ============================================================================
// DEMOS v1.0 - ENTERPRISE (STREAMING & VALIDATION)
// ============================================================================

func demo_v3_Validation() {
	fmt.Println("Goal: Validate business rules (Min, Regex, Enum).")

	xmlData := `<user><age>17</age><role>hacker</role></user>`
	m, _ := xml.MapXML(strings.NewReader(xmlData), xml.EnableExperimental())

	rules := []xml.Rule{
		{Path: "user/age", Type: "int", Min: 18},                             // Fails
		{Path: "user/role", Type: "string", Enum: []string{"admin", "user"}}, // Fails
	}

	errs := xml.Validate(m, rules)
	fmt.Println("Errors found (Expected):")
	for _, e := range errs {
		fmt.Printf(" - ❌ %s\n", e)
	}
}

func demo_v3_StreamingDecoder() {
	fmt.Println("Goal: Read huge files (Generics) without loading them into RAM.")

	xmlData := `
	<orders>
		<Order><id>101</id><total>50.5</total></Order>
		<Order><id>102</id><total>100.0</total></Order>
	</orders>`

	// Define a partial struct
	type Order struct {
		ID    int     `xml:"id"`
		Total float64 `xml:"total"`
	}

	stream := xml.NewStream[Order](strings.NewReader(xmlData), "Order")

	fmt.Println("Iterating Stream:")
	for o := range stream.Iter() {
		fmt.Printf(" -> Order %d: $%.2f\n", o.ID, o.Total)
	}
}

func demo_v3_StreamingEncoder() {
	fmt.Println("Goal: Write XML straight to an io.Writer with Root attributes.")

	// Data with attributes on the ROOT
	data := map[string]any{
		"feed": map[string]any{
			"@lang":    "en-US", // Feature: Root Attribute
			"@version": "2.0",   // Feature: Root Attribute
			"title":    "Tech Blog",
		},
	}

	// Write straight to Stdout (io.Writer) without an intermediate string
	encoder := xml.NewEncoder(os.Stdout, xml.WithPrettyPrint())

	fmt.Println("Generating XML straight to the console:")
	if err := encoder.Encode(data); err != nil {
		fmt.Println("Error encoding:", err)
	}
	fmt.Println() // Trailing newline
}

// ============================================================================
// DEMOS v3.0 - UTILITIES & LEGACY
// ============================================================================

func demo_v3_LegacyCharsets() {
	fmt.Println("Goal: Parse XML encoded in ISO-8859-1 (Latin1).")

	// "café" in ISO-8859-1 is: 0x63, 0x61, 0x66, 0xE9
	// Reading it as UTF-8 without a CharsetReader would fail or produce garbage.
	isoData := []byte{
		'<', 'd', 'a', 't', 'a', '>',
		'c', 'a', 'f', 0xE9,
		'<', '/', 'd', 'a', 't', 'a', '>',
	}

	fmt.Println("Input (Bytes):", isoData)

	reader := strings.NewReader(string(isoData)) // Note: string(bytes) preserves the raw bytes in Go

	m, err := xml.MapXML(reader, xml.EnableLegacyCharsets())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Resulting Map: %v\n", m)
	// Expected output: café (UTF-8) if the console supports it, or the correct bytes.
}

func demo_v3_JSONConversion() {
	fmt.Println("Goal: Convert XML to clean JSON (no metadata).")

	xmlData := `<user id="42"><name>Alice</name><active>true</active></user>`
	reader := strings.NewReader(xmlData)

	jsonBytes, err := xml.ToJSON(reader)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("XML Input: %s\n", xmlData)
	fmt.Printf("JSON Output: %s\n", string(jsonBytes))
}

func demo_soap() {
	// 1. Configure the Client
	// Real public service (DataFlex Web Service for Country Info)
	// WSDL: http://webservices.oorsprong.org/websamples.countryinfo/CountryInfoService.wso?WSDL

	// NOTE: This service is strict about URLs.
	endpoint := "http://webservices.oorsprong.org/websamples.countryinfo/CountryInfoService.wso"

	// The exact Namespace defined in its WSDL for the body
	namespace := "http://www.oorsprong.org/websamples.countryinfo"

	// Optional: configure the base SOAPAction header (though this client derives it when standard)
	client := xml.NewSoapClient(endpoint, namespace)

	fmt.Println("=== Demo: Dynamic SOAP Client (Real Service) ===")

	// ---------------------------------------------------------
	// Call 1: ListOfContinentsByName
	// ---------------------------------------------------------
	fmt.Println("\n1. Calling ListOfContinentsByName...")

	// This service takes no parameters for this call
	resp, err := client.Call("ListOfContinentsByName", nil)
	if err != nil {
		log.Fatalf("Error calling ListOfContinentsByName: %v", err)
	}

	// Dynamic Parsing
	// The real service returns:
	// <m:ListOfContinentsByNameResponse>
	//   <m:ListOfContinentsByNameResult>
	//     <m:tContinent>
	//       <m:sCode>AF</m:sCode>
	//       <m:sName>Africa</m:sName>
	//     </m:tContinent>
	//     ...
	continents, _ := xml.QueryAll(resp, "//tContinent")
	fmt.Printf("Found %d continents:\n", len(continents))

	for _, c := range continents {
		// Just use Query directly on 'c' (which works for both map and OrderedMap)
		code, _ := xml.Query(c, "sCode")
		name, _ := xml.Query(c, "sName")
		fmt.Printf(" - %v: %v\n", code, name)
	}

	// ---------------------------------------------------------
	// Call 2: FullCountryInfo (a call that takes parameters)
	// ---------------------------------------------------------
	fmt.Println("\n2. Calling FullCountryInfo (Code: AR)...")

	payload := map[string]any{
		"sCountryISOCode": "AR", // Argentina
	}

	resp2, err := client.Call("FullCountryInfo", payload)
	if err != nil {
		log.Printf("Error calling FullCountryInfo: %v", err)
	} else {
		// The response is nested inside FullCountryInfoResult
		name, _ := xml.Query(resp2, "//sName")
		capital, _ := xml.Query(resp2, "//sCapitalCity")
		currency, _ := xml.Query(resp2, "//sCurrencyISOCode")
		flag, _ := xml.Query(resp2, "//sCountryFlag")

		fmt.Printf("Country: %v\n", name)
		fmt.Printf("Capital: %v\n", capital)
		fmt.Printf("Currency: %v\n", currency)
		fmt.Printf("Flag URL: %v\n", flag)
	}
}

func demo_soap2() {
	client := xml.NewSoapClient("http://www.dneonline.com/calculator.asmx", "http://tempuri.org/")

	// Build the payload
	// The Envelope and Body are handled automatically.
	// You only provide the content inside the Action tag.
	payload := map[string]any{
		"intA": 60,
		"intB": 20,
	}

	methods := []string{"Add", "Subtract", "Multiply", "Divide"}

	for _, method := range methods {
		resp, err := client.Call(method, payload)
		if err != nil {
			panic(err)
		}
		result, _ := xml.Query(resp, "//"+method+"Result")
		fmt.Printf("%s result: %v\n", method, result)
	}
}

func demo_dian() {
	fmt.Println("   -> Loading certificates...")

	// 1. Check that the files exist
	certPath := "certificado.crt"
	keyPath := "privada.key"
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		fmt.Printf("❌ Error: '%s' is missing. Copy it from the previous demo.\n", certPath)
		return
	}

	crt, _ := os.ReadFile(certPath)
	key, _ := os.ReadFile(keyPath)
	signer, err := xml.NewSigner(crt, key)
	if err != nil {
		fmt.Printf("❌ Error parsing keys: %v\n", err)
		return
	}
	fmt.Println("   -> Keys loaded successfully.")

	// 2. Build the invoice CONTENT (the inner data)
	fmt.Println("   -> Generating XML...")

	innerInvoice := xml.NewMap() // This will be the content of <Invoice>
	innerInvoice.Set("@xmlns", "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2")
	innerInvoice.Set("@xmlns:ds", "http://www.w3.org/2000/09/xmldsig#") // Signature namespace

	innerInvoice.Set("ID", "SETT-100")
	innerInvoice.Set("IssueDate", "2025-12-19")
	innerInvoice.Set("InvoiceTypeCode", "01")

	innerInvoice.Set("LegalMonetaryTotal/LineExtensionAmount", 1000.00)
	innerInvoice.Set("LegalMonetaryTotal/PayableAmount", 1000.00)

	// 3. Create the ROOT wrapper (to satisfy the single-root-element rule)
	doc := xml.NewMap()
	doc.Set("Invoice", innerInvoice)

	// 4. Serialize to get the bytes to sign
	invoiceBytes, err := xml.Marshal(doc)
	if err != nil {
		fmt.Printf("❌ Error generating base XML: %v\n", err)
		return
	}

	// 5. Generate the signature
	fmt.Println("   -> Computing Digital Signature (SHA256 + RSA)...")
	signatureMap, err := signer.CreateXadesSignature([]byte(invoiceBytes))
	if err != nil {
		fmt.Printf("❌ Error signing: %v\n", err)
		return
	}

	// 6. Inject the signature into the XML
	// It goes inside 'innerInvoice', not 'doc'
	innerInvoice.Set("ds:Signature", signatureMap)

	// 7. Final result
	finalXML, _ := xml.Marshal(doc)

	fmt.Println("\n✅ XML SIGNED SUCCESSFULLY (DIAN READY):")
	fmt.Println("--------------------------------------------------")
	fmt.Println(finalXML)
	fmt.Println("--------------------------------------------------")

	if err := signer.Verify([]byte(finalXML)); err != nil {
		fmt.Printf("❌ Verify failed: %v\n", err)
		return
	}
	fmt.Println("✅ Verify: the signature is valid (digest + RSA verified).")
}

func demo_dian_ubl() {
	fmt.Println(">>> Generating DIAN Electronic Invoice (UBL 2.1) with CUFE <<<")

	// 1. Load certificates
	crt, err := os.ReadFile("certificado.crt")
	if err != nil {
		fmt.Println("❌ Error loading certificado.crt:", err)
		return
	}
	key, err := os.ReadFile("privada.key")
	if err != nil {
		fmt.Println("❌ Error loading privada.key:", err)
		return
	}

	// Signing wrapper
	signer, _ := xml.NewSigner(crt, key)

	// ===============================================================
	// VARIABLE DATA
	// ===============================================================
	invoiceNumber := "SETP-99000001"
	supplierNIT := "800197268"
	customerNIT := "222222222222"
	totalAmount := "1000.00"
	taxAmount := "0.00"
	payableAmount := "1000.00"
	environmentType := "2" // Testing
	technicalKey := "fc8eac422eba16e22ffd8c6f94b3940a6e681623"

	now := time.Now()
	issueDate := now.Format("2006-01-02")
	// TIME FIX: concatenate the fixed offset for Colombia (-05:00)
	// so Go does not interpret the trailing 05 as repeated "seconds".
	issueTime := now.Format("15:04:05") + "-05:00"

	// ===============================================================
	// CUFE COMPUTATION
	// ===============================================================
	fmt.Println("   -> Computing CUFE...")
	cufe := CalculateCUFE(
		invoiceNumber, issueDate, issueTime, totalAmount,
		"01", taxAmount,
		"04", "0.00",
		payableAmount,
		supplierNIT, customerNIT,
		technicalKey, environmentType,
	)
	fmt.Printf("      Generated CUFE: %s...\n", cufe[:15])

	// ===============================================================
	// INVOICE CONTENT CONSTRUCTION
	// ===============================================================
	invoiceData := xml.NewMap()

	// A. NAMESPACES
	invoiceData.Set("@xmlns", "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2")
	invoiceData.Set("@xmlns:cac", "urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2")
	invoiceData.Set("@xmlns:cbc", "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2")
	invoiceData.Set("@xmlns:ds", "http://www.w3.org/2000/09/xmldsig#")
	invoiceData.Set("@xmlns:ext", "urn:oasis:names:specification:ubl:schema:xsd:CommonExtensionComponents-2")
	invoiceData.Set("@xmlns:xades", "http://uri.etsi.org/01903/v1.3.2#")
	invoiceData.Set("@xmlns:xades141", "http://uri.etsi.org/01903/v1.4.1#")
	invoiceData.Set("@xmlns:xsi", "http://www.w3.org/2001/XMLSchema-instance")
	invoiceData.Set("@xsi:schemaLocation", "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2 http://docs.oasis-open.org/ubl/os-UBL-2.1/xsd/maindoc/UBL-Invoice-2.1.xsd")

	// C. HEADER DATA
	invoiceData.Set("cbc:UBLVersionID", "UBL 2.1")
	invoiceData.Set("cbc:CustomizationID", "10")
	invoiceData.Set("cbc:ProfileID", "DIAN 2.1: Factura Electrónica de Venta")
	invoiceData.Set("cbc:ProfileExecutionID", environmentType)
	invoiceData.Set("cbc:ID", invoiceNumber)

	// FIX: CUFE (value + attribute)
	invoiceData.Set("cbc:UUID", map[string]interface{}{
		"#text":       cufe,
		"@schemeName": "CUFE-SHA384",
	})

	invoiceData.Set("cbc:IssueDate", issueDate)
	invoiceData.Set("cbc:IssueTime", issueTime)
	invoiceData.Set("cbc:InvoiceTypeCode", "01")
	invoiceData.Set("cbc:Note", "Invoice created with go-xml")
	invoiceData.Set("cbc:DocumentCurrencyCode", "COP")

	// D. SUPPLIER
	supplier := xml.NewMap()
	supplier.Set("cbc:AdditionalAccountID", "1")
	party := xml.NewMap()
	scheme := xml.NewMap()
	scheme.Set("cbc:RegistrationName", "Mi Empresa S.A.S")
	scheme.Set("cbc:CompanyID", supplierNIT)
	scheme.Set("@schemeID", "31")
	scheme.Set("@schemeName", "31")
	party.Set("cac:PartyTaxScheme", scheme)
	supplier.Set("cac:Party", party)
	invoiceData.Set("cac:AccountingSupplierParty", supplier)

	// E. CUSTOMER
	customer := xml.NewMap()
	customer.Set("cbc:AdditionalAccountID", "1")
	cParty := xml.NewMap()
	cScheme := xml.NewMap()
	cScheme.Set("cbc:RegistrationName", "Cliente Final")
	cScheme.Set("cbc:CompanyID", customerNIT)
	cScheme.Set("@schemeID", "13")
	cParty.Set("cac:PartyTaxScheme", cScheme)
	customer.Set("cac:Party", cParty)
	invoiceData.Set("cac:AccountingCustomerParty", customer)

	// F. TOTALS (FIX: use maps for value and currency)
	totals := xml.NewMap()

	// Helper to build monetary amounts
	copAmount := func(val string) map[string]interface{} {
		return map[string]interface{}{
			"#text":       val,
			"@currencyID": "COP",
		}
	}

	totals.Set("cbc:LineExtensionAmount", copAmount(totalAmount))
	totals.Set("cbc:TaxExclusiveAmount", copAmount(taxAmount))
	totals.Set("cbc:TaxInclusiveAmount", copAmount(payableAmount))
	totals.Set("cbc:PayableAmount", copAmount(payableAmount))
	invoiceData.Set("cac:LegalMonetaryTotal", totals)

	// G. LINES
	line := xml.NewMap()
	line.Set("cbc:ID", "1")

	// FIX: quantity with unit
	line.Set("cbc:InvoicedQuantity", map[string]interface{}{
		"#text":     "1",
		"@unitCode": "EA",
	})

	line.Set("cbc:LineExtensionAmount", copAmount(totalAmount))

	item := xml.NewMap()
	item.Set("cbc:Description", "Go Software Services")
	line.Set("cac:Item", item)

	price := xml.NewMap()
	price.Set("cbc:PriceAmount", copAmount(totalAmount))
	line.Set("cac:Price", price)

	invoiceData.Set("cac:InvoiceLine", line)

	// ===============================================================
	// SIGNING PROCESS
	// ===============================================================
	fmt.Println("   -> Generating base XML for signing...")

	// Build a temporary "pre-root" just so the signer has context
	preRoot := xml.NewMap()
	preRoot.Set("Invoice", invoiceData)
	xmlBytesToSign, _ := xml.Marshal(preRoot)

	fmt.Println("   -> Computing XAdES Signature...")
	sig, err := signer.CreateXadesSignature([]byte(xmlBytesToSign))
	if err != nil {
		fmt.Printf("❌ Error signing: %v\n", err)
		return
	}

	// ===============================================================
	// SIGNATURE INJECTION AND FINAL RECONSTRUCTION
	// ===============================================================

	// Extension structure
	extensionContent := xml.NewMap()
	extensionContent.Set("ds:Signature", sig)
	ublExtension := xml.NewMap()
	ublExtension.Set("ext:ExtensionContent", extensionContent)
	ublExtensions := xml.NewMap()
	ublExtensions.Set("ext:UBLExtension", ublExtension)

	// Build the ordered content
	finalInvoiceContent := xml.NewMap()

	// 1. Namespaces (@ attributes)
	for _, key := range invoiceData.Keys() {
		if len(key) > 0 && key[:1] == "@" {
			val := invoiceData.Get(key)
			finalInvoiceContent.Set(key, val)
		}
	}

	// 2. Extension with the signature (must come first inside Invoice)
	finalInvoiceContent.Set("ext:UBLExtensions", ublExtensions)

	// 3. Rest of the body
	for _, key := range invoiceData.Keys() {
		if len(key) > 0 && key[:1] != "@" {
			val := invoiceData.Get(key)
			finalInvoiceContent.Set(key, val)
		}
	}

	// -------------------------------------------------------------
	// ⚡️ KEY STEP: WRAP IN "Invoice"
	// -------------------------------------------------------------
	root := xml.NewMap()
	root.Set("Invoice", finalInvoiceContent)

	finalXML, err := xml.Marshal(root)
	if err != nil {
		fmt.Printf("❌ Error marshalling XML: %v\n", err)
		return
	}

	fmt.Println("\n✅ XML SIGNED SUCCESSFULLY (DIAN READY):")

	err = os.WriteFile("factura_dian_cufe.xml", []byte(finalXML), 0644)
	if err != nil {
		fmt.Printf("❌ Error saving file: %v\n", err)
		return
	}

	fmt.Println("✅ Final invoice generated: factura_dian_cufe.xml")
}

// CUFE helper
func CalculateCUFE(NumFac, FecFac, HorFac, ValFac, CodImp1, ValImp1, CodImp2, ValImp2, ValTot, NitEmi, NumAdq, ClaveTec, TipoAmb string) string {
	raw := fmt.Sprintf("%s%s%s%s%s%s%s%s%s%s%s%s%s",
		NumFac, FecFac, HorFac, ValFac,
		CodImp1, ValImp1,
		CodImp2, ValImp2,
		ValTot,
		NitEmi, NumAdq,
		ClaveTec,
		TipoAmb)
	hash := sha512.Sum384([]byte(raw))
	return fmt.Sprintf("%x", hash)
}
