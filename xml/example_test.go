package xml

import (
	"fmt"
	"os"
	"strings"
)

// Este ejemplo aparecerá en la documentación de Go como un caso de uso real.
func ExampleMapXML() {
	xmlData := `<user><name>Arturo</name><role>Admin</role></user>`

	// 1. Convertir XML a Mapa
	m, _ := MapXML(strings.NewReader(xmlData))

	// 2. Consultar valor
	// CORRECCIÓN: Como <name> no tiene atributos, el parser lo simplifica directo al valor.
	// No hace falta poner "/#text".
	name, _ := Query(m, "user/name")
	role, _ := Query(m, "user/role")

	fmt.Printf("User: %s, Role: %s\n", name, role)

	// Output:
	// User: Arturo, Role: Admin
}

func ExampleForceArray() {
	// Caso donde hay un solo item, pero queremos tratarlo como lista
	xmlData := `<list><item>Uno</item></list>`

	m, _ := MapXML(strings.NewReader(xmlData), ForceArray("item"))

	// CORRECCIÓN: Igual aquí, buscamos el item directo, no su #text interno
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
		fmt.Println("Configuración Válida")
	}

	// Output:
	// Configuración Válida
}

func ExampleNewEncoder() {
	data := map[string]any{
		"response": map[string]any{
			"-status": "ok",
			"message": "Hello World",
		},
	}

	// Escribe XML limpio a la consola
	NewEncoder(os.Stdout).Encode(data)

	// Output:
	// <response status="ok"><message>Hello World</message></response>
}
