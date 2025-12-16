# go-xml: El Parser XML Empresarial de Archivo Único
> Deja de escribir structs para XML dinámico.

`go-xml` (v1.0) es un parser y serializador de XML robusto y sin esquema para Go. Está diseñado para manejar XML complejo, dinámico o "sucio" sin necesidad de definir structs rígidos de Go para cada etiqueta. Ofrece una solución completa que va desde mapas dinámicos en memoria hasta streaming de alto rendimiento para archivos de gigabytes.

## 🚀 Características Principales

*   **Parseo Sin Esquema**: el contenido se parsea en `map[string]any`, permitiéndote recorrer estructuras XML desconocidas dinámicamente.
*   **Soporte de Streaming**:
    *   **Decoder**: Procesa archivos de varios gigabytes con uso constante de memoria usando `Stream[T]` Genérico.
    *   **Encoder**: Escribe XML directamente a un `io.Writer` para procesamiento eficiente en tuberías.
*   **Robusto y Permisivo**: Capaz de leer HTML/XML "sopa" (tags sin cerrar como `<br>`, `<meta>`) con modo permisivo. Usa `EnableExperimental()` para activar el Modo Soup (sanitización automática de `<script>`, normalización a minúsculas y soporte de tags HTML vacíos).
*   **Consultas Avanzadas**: Utilidades de consulta profunda estilo XPath (ej: `users/user[0]/name`).
*   **Motor de Validación**: Define reglas de negocio para tus datos (Regex, Rango, Enum, Tipo).
*   **Atributos como Datos**: Los atributos son tratados como ciudadanos de primera clase, accesibles mediante una convención de prefijo `@`.
*   **Namespaces Helpers**: Registra alias para trabajar con claves cortas en lugar de URLs completas.
*   **Hooks de Valor**: Define lógica personalizada para transformar strings en tipos nativos de Go (Fechas, Enums, etc.) durante el parseo.
*   **CLI Integrado**: Incluye una herramienta de terminal para consultas rápidas de XML.
*   **Charsets Legados**: Soporte nativo para ISO-8859-1 y Windows-1252 mediante `EnableLegacyCharsets()`.

## 📦 Instalación

```bash
go get github.com/arturoeanton/go-xml
```

## 📖 Guía de Uso

### 1. Parseo Básico (MapXML)
La función principal `MapXML` lee datos en un mapa dinámico.

```go
package main

import (
    "fmt"
    "strings"
    "github.com/arturoeanton/go-xml/xml"
)

func main() {
    xmlData := `<library><book id="1">El Principito</book></library>`

    // Parsear sin definir structs
    m, err := xml.MapXML(strings.NewReader(xmlData))
    if err != nil {
        panic(err)
    }

    // Acceso manual a los datos
    lib := m["library"].(map[string]any)
    book := lib["book"].(map[string]any)

    fmt.Println("Título:", book["#text"]) // "El Principito"
    fmt.Println("ID:", book["@id"])       // "1" (Los atributos usan prefijo '@')
}
```

### 2. Manejo de Arrays JSON (ForceArray)
XML es ambiguo respecto a los arrays (un hijo único vs. una lista de un hijo). Usa `ForceArray` para asegurar que etiquetas específicas sean siempre tratadas como slices `[]any`.

```go
// <library><book>Uno</book></library>
m, _ := xml.MapXML(r, xml.ForceArray("book"))

// Ahora 'book' está garantizado de ser []any, incluso si solo hay un libro.
books := m["library"].(map[string]any)["book"].([]any)
```

### 3. Namespaces
Simplifica las claves mapeadas desde XML con namespaces registrando alias.

```go
// <root xmlns:h="http://w3.org/html"><h:table>Datos</h:table></root>
m, _ := xml.MapXML(r, xml.RegisterNamespace("html", "http://w3.org/html"))

// Acceder como "html:table" en lugar de la URL completa
val, _ := xml.Query(m, "root/html:table/#text")
```

### 4. Hooks e Inferencia de Tipos
Convierte automáticamente strings a tipos nativos de Go o aplica lógica personalizada.

```go
// <log><date>2025-12-31</date><count>99</count></log>

// 1. Hook Personalizado para Fechas
dateHook := func(s string) any {
    t, _ := time.Parse("2006-01-02", s)
    return t
}

m, _ := xml.MapXML(r, 
    xml.WithValueHook("date", dateHook),
    xml.EnableExperimental(), // Infiere automáticamente "99" como int
)

dateVal, _ := xml.Query(m, "log/date") // Devuelve time.Time
```

### 5. Streaming (Archivos Grandes)
Para conjuntos de datos enormes, evita cargar todo en memoria.

#### Streaming Decoder (Generics)
Usa `Stream[T]` para iterar elemento por elemento con tipado fuerte para los nodos específicos que necesitas.

```go
type Order struct {
    ID    int     `xml:"id"`
    Total float64 `xml:"total"`
}

func main() {
    file, _ := os.Open("huge_orders.xml")
    defer file.Close()

    // Stream de elementos <Order> uno por uno
    stream := xml.NewStream[Order](file, "Order")

    for order := range stream.Iter() {
        fmt.Printf("Procesando Orden %d: $%.2f\n", order.ID, order.Total)
    }

    // O con Context para cancelación/timeout:
    // ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    // defer cancel()
    // for order := range stream.IterWithContext(ctx) { ... }
}
```

#### Streaming Encoder
Escribe XML directamente a un `io.Writer` (como `http.ResponseWriter` u `os.File`) eficientemente.

```go
data := map[string]any{
    "feed": map[string]any{
        "@version": "2.0",
        "title":    "Blog Tech",
    },
}

// Escribe directamente a stdout con indentación
xml.NewEncoder(os.Stdout, xml.WithPrettyPrint()).Encode(data)
```

### 6. Reglas de Validación
Valida datos dinámicos contra reglas de negocio sin structs.

```go
rules := []xml.Rule{
    {Path: "user/age",  Type: "int",    Min: 18},
    {Path: "user/role", Type: "string", Enum: []string{"admin", "user"}},
    {Path: "user/email", Type: "string", Regex: `^.+@.+\..+$`},
}

errors := xml.Validate(data, rules)
if len(errors) > 0 {
    fmt.Println("Validación fallida:", errors)
}
```

### 7. Exportación JSON en una línea
Convierte XML directamente a JSON en un solo paso (útil para APIs).

```go
jsonBytes, _ := xml.ToJSON(r)
fmt.Println(string(jsonBytes))
```

### 8. Soporte de Charsets Legados (ISO-8859-1 / Windows-1252)
El parser maneja UTF-8 automáticamente. Para sistemas legados (bancos, gobierno) que envían ISO-8859-1 o Windows-1252, usa `EnableLegacyCharsets()`.

```go
// El header dice encoding="ISO-8859-1"
m, err := xml.MapXML(r, xml.EnableLegacyCharsets())
// El parser inyecta automáticamente el lector de charsets correcto
```

## 🛠 Herramienta CLI
Puedes usar `main.go` como una herramienta CLI independiente para consultar archivos XML desde la terminal.

```bash
# Consultar un valor de un archivo XML
cat data.xml | go run main.go query "users/user[0]/name"
```

## ⚙️ Detalles de Implementación

### Arquitectura
La librería está diseñada como una solución de **Archivo Único** (conceptualmente) para minimizar el infierno de dependencias, aunque organizada internamente.
1.  **Núcleo del Parser**: Implementa una máquina de estados basada en pila que procesa `xml.Token`. Normaliza las peculiaridades de XML en una estructura de mapa consistente tipo JSON.
2.  **Inferencia de Tipos**: Si se habilita (`EnableExperimental`), detecta automáticamente números y booleanos (ej: "123" se convierte en `int(123)` en lugar de string).

### Mapeo de Estructura de Datos
La representación interna sigue estas convenciones para mapear XML a `map[string]any`:

*   **Elementos**: Se convierten en claves del diccionario (`<tag>` -> `"tag"`).
*   **Atributos**: Se convierten en claves con prefijo `@` (`id="1"` -> `"@id": "1"`).
*   **Contenido de Texto**: Se almacena en la clave especial `"#text"`.
*   **Comentarios**: Se almacenan en `"#comments"` (lista de strings).
*   **CDATA**: Se almacena en `"#cdata"`.
