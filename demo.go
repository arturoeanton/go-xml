package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/arturoeanton/go-xml/xml"
)

// ============================================================================
// REGISTRO DE DEMOS
// ============================================================================

// Mapa que vincula el nombre del flag --demo [nombre] con la función
var demoRegistry = map[string]func(){
	// v0.1 - Lo Básico
	"basic": demo_v1_BasicParsing,
	"array": demo_v1_ForceArray,

	// v1.0 - Robustez
	"html": demo_v1_HtmlLenient,

	// v2.0 - Namespaces y Querys
	"ns":    demo_v2_Namespaces,
	"query": demo_v2_QueryAdvanced,

	// v2.1 - Hooks y CDATA
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
}

// RunDemos: El orquestador que llama el main()
func RunDemos(arg string) {
	fmt.Println("========================================")
	fmt.Println("   r2/xml - Galería de Demos Histórica")
	fmt.Println("========================================")

	if arg == "all" || arg == "" {
		// Ejecutar TODAS en orden lógico (no por mapa aleatorio)
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
		// Ejecutar UNA específica
		if fn, exists := demoRegistry[arg]; exists {
			printHeader(arg)
			fn()
		} else {
			fmt.Printf("❌ Demo '%s' no encontrada.\nDemos disponibles: %v\n", arg, getDemoKeys())
		}
	}
}

func printHeader(name string) {
	fmt.Printf("\n>>> Ejecutando Demo: [%s] <<<\n", strings.ToUpper(name))
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
// DEMOS v0.1 - FUNDAMENTOS
// ============================================================================

func demo_v1_BasicParsing() {
	fmt.Println("Objetivo: Leer XML simple sin Structs.")

	xmlData := `<library><book id="1">El Principito</book></library>`

	// 1. Parseo
	m, err := xml.MapXML(strings.NewReader(xmlData))
	if err != nil {
		panic(err)
	}

	title, _ := xml.Query(m, "library/book/#text")
	id, _ := xml.Query(m, "library/book/@id")

	fmt.Printf("Mapa Resultante: %+v\n", m)
	fmt.Printf("Título: %s (ID: %s)\n", title, id)
}

func demo_v1_ForceArray() {
	fmt.Println("Objetivo: Resolver ambigüedad JSON (Single vs Array).")

	// Caso dificil: Solo hay un libro, pero el frontend espera una lista
	xmlData := `<library><book>Solo Uno</book></library>`

	m, _ := xml.MapXML(strings.NewReader(xmlData), xml.ForceArray("book"))

	// Verificamos que sea []any
	lib := m["library"].(map[string]any)
	books := lib["book"].([]any) // Si falla el cast, ForceArray falló

	fmt.Printf("Tipo de 'book': %T (Longitud: %d)\n", books, len(books))
}

// ============================================================================
// DEMOS v1.0 - ROBUSTEZ (HTML)
// ============================================================================

func demo_v1_HtmlLenient() {
	fmt.Println("Objetivo: Leer HTML sucio (tags sin cerrar).")

	// <br> y <meta> no tienen cierre
	htmlData := `<html><body>Hola<br>Mundo<br><meta charset="utf-8"></body></html>`

	m, err := xml.MapXML(strings.NewReader(htmlData), xml.EnableExperimental())
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Usamos Query para verificar que leyó después de los br
	body, _ := xml.Query(m, "html/body/#text")
	fmt.Printf("Contenido leído exitosamente: %v\n", body)
}

// ============================================================================
// DEMOS v2.0 - NAMESPACES & QUERY
// ============================================================================

func demo_v2_Namespaces() {
	fmt.Println("Objetivo: Limpiar URLs largas de los keys.")

	xmlData := `<root xmlns:h="http://w3.org/html"><h:table>Datos</h:table></root>`

	m, _ := xml.MapXML(strings.NewReader(xmlData),
		xml.RegisterNamespace("html", "http://w3.org/html"),
	)

	// Ahora podemos acceder como "html:table" en vez de la URL completa
	tableVal, _ := xml.Query(m, "root/html:table/#text")
	fmt.Printf("Valor con Namespace limpio: %v\n", tableVal)
}

func demo_v2_QueryAdvanced() {
	fmt.Println("Objetivo: Búsqueda profunda e iterativa (QueryAll).")

	xmlData := `
	<store>
		<section><item>A</item><item>B</item></section>
		<section><item>C</item></section>
	</store>`

	m, _ := xml.MapXML(strings.NewReader(xmlData), xml.ForceArray("section", "item"))

	// QueryAll aplana los arrays intermedios (section -> item)
	items, _ := xml.QueryAll(m, "store/section/item/#text")

	fmt.Printf("Items encontrados (3 esperados): %v\n", items)
}

// ============================================================================
// DEMOS v2.1 - HOOKS & MARSHAL PRO
// ============================================================================

func demo_v2_HooksAndTypes() {
	fmt.Println("Objetivo: Transformar strings a Tipos Go (Time/Int) al vuelo.")

	xmlData := `<log><date>2025-12-31</date><count>99</count></log>`

	// Hook para Fechas
	dateHook := func(s string) any {
		t, _ := time.Parse("2006-01-02", s)
		return t
	}

	m, _ := xml.MapXML(strings.NewReader(xmlData),
		xml.WithValueHook("date", dateHook),
		xml.EnableExperimental(), // Infiere "99" -> int
	)

	dateVal, _ := xml.Query(m, "log/date")
	countVal, _ := xml.Query(m, "log/count")

	fmt.Printf("Fecha Tipo: %T, Valor: %v\n", dateVal, dateVal)
	fmt.Printf("Count Tipo: %T, Valor: %v\n", countVal, countVal)
}

func demo_v2_MarshalCDATA() {
	fmt.Println("Objetivo: Generar XML con CDATA y Comentarios.")

	data := map[string]any{
		"msg": map[string]any{
			"#comments": []string{" Esto es HTML raw "},
			"#cdata":    "<b>Negrita</b>",
		},
	}

	// Usamos el Encoder (que usa la misma lógica que Marshal v2)
	fmt.Println("XML Generado:")
	xml.NewEncoder(os.Stdout, xml.WithPrettyPrint()).Encode(data)
	fmt.Println()
}

// ============================================================================
// DEMOS v1.0 - ENTERPRISE (STREAMING & VALIDATION)
// ============================================================================

func demo_v3_Validation() {
	fmt.Println("Objetivo: Validar reglas de negocio (Min, Regex, Enum).")

	xmlData := `<user><age>17</age><role>hacker</role></user>`
	m, _ := xml.MapXML(strings.NewReader(xmlData), xml.EnableExperimental())

	rules := []xml.Rule{
		{Path: "user/age", Type: "int", Min: 18},                             // Falla
		{Path: "user/role", Type: "string", Enum: []string{"admin", "user"}}, // Falla
	}

	errs := xml.Validate(m, rules)
	fmt.Println("Errores encontrados (Esperados):")
	for _, e := range errs {
		fmt.Printf(" - ❌ %s\n", e)
	}
}

func demo_v3_StreamingDecoder() {
	fmt.Println("Objetivo: Leer archivos gigantes (Generics) sin cargar RAM.")

	xmlData := `
	<orders>
		<Order><id>101</id><total>50.5</total></Order>
		<Order><id>102</id><total>100.0</total></Order>
	</orders>`

	// Definimos struct parcial
	type Order struct {
		ID    int     `xml:"id"`
		Total float64 `xml:"total"`
	}

	stream := xml.NewStream[Order](strings.NewReader(xmlData), "Order")

	fmt.Println("Iterando Stream:")
	for o := range stream.Iter() {
		fmt.Printf(" -> Orden %d: $%.2f\n", o.ID, o.Total)
	}
}

func demo_v3_StreamingEncoder() {
	fmt.Println("Objetivo: Escribir XML directo a io.Writer con Atributos Root.")

	// Datos con atributos en el ROOT
	data := map[string]any{
		"feed": map[string]any{
			"@lang":    "es-AR", // Feature: Root Attribute
			"@version": "2.0",   // Feature: Root Attribute
			"title":    "Blog Tech",
		},
	}

	// Escribir directo a Stdout (io.Writer) sin crear string intermedio
	encoder := xml.NewEncoder(os.Stdout, xml.WithPrettyPrint())

	fmt.Println("Generando XML directo a consola:")
	if err := encoder.Encode(data); err != nil {
		fmt.Println("Error encoding:", err)
	}
	fmt.Println() // Salto de línea final
}

// ============================================================================
// DEMOS v3.0 - UTILITIES & LEGACY
// ============================================================================

func demo_v3_LegacyCharsets() {
	fmt.Println("Objetivo: Parsear XML codificado en ISO-8859-1 (Latin1).")

	// "café" en ISO-8859-1 es: 0x63, 0x61, 0x66, 0xE9
	// Si lo leyéramos como UTF-8 sin CharsetReader, fallaría o daría basura.
	isoData := []byte{
		'<', 'd', 'a', 't', 'a', '>',
		'c', 'a', 'f', 0xE9,
		'<', '/', 'd', 'a', 't', 'a', '>',
	}

	fmt.Println("Input (Bytes):", isoData)

	reader := strings.NewReader(string(isoData)) // Nota: string(bytes) preserva los bytes tal cual en Go

	m, err := xml.MapXML(reader, xml.EnableLegacyCharsets())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Mapa Resultante: %v\n", m)
	// Output esperado: cafÃ© (UTF-8) si la consola lo soporta, o los bytes correctos.
}

func demo_v3_JSONConversion() {
	fmt.Println("Objetivo: Convertir XML a JSON limpio (sin metadatos).")

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

	// NOTA: Este servicio es estricto con las URLs.
	endpoint := "http://webservices.oorsprong.org/websamples.countryinfo/CountryInfoService.wso"

	// El Namespace exacto definido en su WSDL para el body
	namespace := "http://www.oorsprong.org/websamples.countryinfo"

	// Opcional: Configurar el Header SOAPAction base (aunque este cliente lo deduce si es estándar)
	client := xml.NewSoapClient(endpoint, namespace)

	fmt.Println("=== Demo: Dynamic SOAP Client (Real Service) ===")

	// ---------------------------------------------------------
	// Call 1: ListOfContinentsByName
	// ---------------------------------------------------------
	fmt.Println("\n1. Calling ListOfContinentsByName...")

	// Este servicio no requiere parámetros para esta llamada
	resp, err := client.Call("ListOfContinentsByName", nil)
	if err != nil {
		log.Fatalf("Error calling ListOfContinentsByName: %v", err)
	}

	// Dynamic Parsing
	// El servicio real devuelve:
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
		if cMap, ok := c.(map[string]any); ok {
			// Usamos xml.String helper si lo implementaste, o Query normal
			code, _ := xml.Query(cMap, "sCode")
			name, _ := xml.Query(cMap, "sName")
			fmt.Printf(" - %v: %v\n", code, name)
		}
	}

	// ---------------------------------------------------------
	// Call 2: FullCountryInfo (Probemos algo con parámetros)
	// ---------------------------------------------------------
	fmt.Println("\n2. Calling FullCountryInfo (Code: AR)...")

	payload := map[string]any{
		"sCountryISOCode": "AR", // Argentina
	}

	resp2, err := client.Call("FullCountryInfo", payload)
	if err != nil {
		log.Printf("Error calling FullCountryInfo: %v", err)
	} else {
		// La respuesta está anidada en FullCountryInfoResult
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

	// Construir Carga
	// El Envelope y Body se manejan automáticamente.
	// Solo provees el contenido dentro del tag de la Acción.
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
		fmt.Printf("Resultado %s: %v\n", method, result)
	}
}
