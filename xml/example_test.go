package xml

import (
	"fmt"
	"os"
	"strings"
)

// This example will appear in the Go documentation as a real-world use case.
func ExampleMapXML() {
	xmlData := `<user><name>Arthur</name><role>Admin</role></user>`

	// 1. Convert XML to Map
	m, _ := MapXML(strings.NewReader(xmlData))

	// 2. Query value
	// CORRECTION: Since <name> has no attributes, the parser simplifies it directly to the value.
	// No need to use "/#text".
	name, _ := Query(m, "user/name")
	role, _ := Query(m, "user/role")

	fmt.Printf("User: %s, Role: %s\n", name, role)

	// Output:
	// User: Arthur, Role: Admin
}

func ExampleForceArray() {
	// Case where there is only one item, but we want to treat it as a list
	xmlData := `<list><item>One</item></list>`

	m, _ := MapXML(strings.NewReader(xmlData), ForceArray("item"))

	// CORRECTION: Same here, we look for the item directly, not its internal #text
	items, _ := QueryAll(m, "list/item")
	fmt.Println("Items:", len(items))

	// Output:
	// Items: 1
}

func ExampleValidate() {
	xmlData := `<config><port>8080</port></config>`
	m, _ := MapXML(strings.NewReader(xmlData), EnableExperimental())

	rules := []Rule{
		{Path: "config/port", Type: "int", Min: 1024, Max: 65535},
	}

	errs := Validate(m, rules)
	if len(errs) == 0 {
		fmt.Println("Valid Configuration")
	}

	// Output:
	// Valid Configuration
}

func ExampleNewEncoder() {
	data := map[string]any{
		"response": map[string]any{
			"@status": "ok",
			"message": "Hello World",
		},
	}

	// Write clean XML to stdout
	NewEncoder(os.Stdout).Encode(data)

	// Output:
	// <response status="ok"><message>Hello World</message></response>
}

// ExampleSoupMode demonstrates how to scrape data from messy, real-world HTML.
// It uses "EnableExperimental" to handle void tags (like <br>, <img>), case insensitivity,
// and sanitizes <script> tags automatically.
func ExampleEnableExperimental() {
	htmlData := `
	<html>
		<body bgcolor="#FFF">
			<div class="product-list">
				<div id="p1" data-stock="yes">
					<h2>Gaming Mouse</h2>
					<span class="price">$50</span>
					<img src="mouse.jpg">
				</div>
			</div>
		</body>
	</html>`

	// 1. Enable Soup Mode (Lenient + Void Tags + Normalization)
	m, _ := MapXML(strings.NewReader(htmlData), EnableExperimental())

	// 2. Query using normalized paths (all lowercase) and attributes
	name, _ := Get[string](m, "html/body/div/div/h2/#text")
	price, _ := Get[string](m, "html/body/div/div/span/#text")
	stock, _ := Get[string](m, "html/body/div/div/@data-stock")

	fmt.Printf("Product: %s, Price: %s, Stock: %s\n", name, price, stock)

	// Output:
	// Product: Gaming Mouse, Price: $50, Stock: yes
}

// ExampleStream demonstrates how to process huge XML files efficiently.
// Instead of loading the entire file into RAM, it iterates element by element
// using Go Generics to map data directly to a struct.
func ExampleNewStream() {
	xmlData := `
	<orders>
		<Order><Id>101</Id><Total>500</Total></Order>
		<Order><Id>102</Id><Total>1200</Total></Order>
		<Order><Id>103</Id><Total>300</Total></Order>
	</orders>`

	type Order struct {
		Id    int `xml:"Id"`
		Total int `xml:"Total"`
	}

	// 1. Create a Stream for "Order" tags
	stream := NewStream[Order](strings.NewReader(xmlData), "Order")

	// 2. Iterate efficiently
	totalRevenue := 0
	for order := range stream.Iter() {
		if order.Total > 1000 {
			fmt.Printf("High Value Order: %d\n", order.Id)
		}
		totalRevenue += order.Total
	}
	fmt.Printf("Total Revenue: %d\n", totalRevenue)

	// Output:
	// High Value Order: 102
	// Total Revenue: 2000
}

// ExampleQuery_filtering demonstrates the advanced filtering capabilities.
// You can search for nodes based on their attributes or child values using [key=value].
func ExampleQuery_filtering() {
	xmlData := `
	<users>
		<user id="1" type="admin"><name>Alice</name></user>
		<user id="2" type="guest"><name>Bob</name></user>
		<user id="3" type="admin"><name>Charlie</name></user>
	</users>`

	m, _ := MapXML(strings.NewReader(xmlData), ForceArray("user"))

	// 1. Find the name of the user with id=2
	guestName, _ := Query(m, "users/user[id=2]/name")

	// 2. Find the name of the user with type=admin (First match)
	// Note: Use "@" for attributes in filters if needed, though simple keys work too depending on parsing.
	adminName, _ := Query(m, "users/user[@type=admin]/name")

	fmt.Printf("Guest: %s, Admin: %s\n", guestName, adminName)

	// Output:
	// Guest: Bob, Admin: Alice
}

// ExampleModifyAndSave shows how to read XML, modify values dynamically,
// and write it back to a clean XML format.
func ExampleSet() {
	xmlData := `<config><debug>false</debug><timeout>30</timeout></config>`
	m, _ := MapXML(strings.NewReader(xmlData))

	// 1. Update existing values
	Set(m, "config/debug", "true")

	// 2. Add new values (automatically creates map keys if parent exists)
	Set(m, "config/server/port", "8080")

	// 3. Write back to XML
	NewEncoder(os.Stdout).Encode(m)

	// Output:
	// <config><debug>true</debug><server><port>8080</port></server><timeout>30</timeout></config>
}

// ExampleTextExtraction shows how to extract deep text content from a structure,
// similar to the jQuery .text() method. Useful for search indexing or summarizing.
func ExampleText() {
	xmlData := `
	<article>
		<h1>Breaking News</h1>
		<content>
			<p>The <b>stock market</b> reached record highs today.</p>
			<p>Details at 11.</p>
		</content>
	</article>`

	m, _ := MapXML(strings.NewReader(xmlData))

	// 1. Extract all text from the <content> tag, ignoring structure
	contentNode, _ := Query(m, "article/content")
	fullText := Text(contentNode)

	fmt.Println(fullText)

	// Output:
	// The stock market reached record highs today.Details at 11.
}

func ExampleEnableLegacyCharsets() {
	// IMPORTANTE: Construimos el XML simulando que viene de un archivo antiguo.
	// Usamos escapes hexadecimales para forzar bytes ISO-8859-1 reales:
	// \xF3 es 'ó' en ISO-8859-1 (en UTF-8 sería \xC3\xB3)
	// \xF1 es 'ñ' en ISO-8859-1 (en UTF-8 sería \xC3\xB1)
	xmlData := "<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?>\n" +
		"<transacci\xF3n>\n" +
		"    <descripci\xF3n>Pago de expensas a\xF1o 2023</descripci\xF3n>\n" +
		"    <moneda>ARS</moneda>\n" +
		"</transacci\xF3n>"

	// El parser detectará el encoding en el header y usará nuestro CharsetReader
	// para transformar esos bytes \xF3 y \xF1 a UTF-8 válido internamente.
	m, err := MapXML(strings.NewReader(xmlData), EnableLegacyCharsets())
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Buscamos usando nombres de tags normales (en UTF-8/Go nativo)
	// Nótese que aquí SÍ escribimos 'ó' normal, porque el mapa 'm' ya está en UTF-8.
	desc, _ := Query(m, "transacción/descripción")
	fmt.Println(desc)

	// Output:
	// Pago de expensas año 2023
}
