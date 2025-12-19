# go-xml v2.0 - Tutorial Completo

Este tutorial cubre los conceptos clave de `go-xml`, una librería diseñada para procesamiento XML **Ordenado y Determinista** en Go.

## 1. ¿Por qué `OrderedMap`?
El `map[string]any` estándar de Go es aleatorio. Esto es terrible para XML.
- **Validación XSD**: Requiere orden estricto (`<User><Name><Age>` != `<User><Age><Name>`).
- **Firmas/Hash**: Cambiar el orden rompe la firma digital.
- **Diferencia (Diffs)**: Orden aleatorio hace que los diffs de git sean inútiles.

`go-xml` v2.0 introduce `*OrderedMap`. Se comporta como un mapa pero recuerda el orden de inserción.

## 2. Leyendo XML
Usa `MapXML` para obtener un `*OrderedMap`.

```go
xmlStr := `<Product id="p1"><Name>Caja</Name><Price>10</Price></Product>`
m, _ := xml.MapXML(strings.NewReader(xmlStr))

// Acceder Atributos
fmt.Println(m.String("Product/@id")) // "p1"

// Acceder Elementos
fmt.Println(m.String("Product/Name")) // "Caja"
```

## 3. Escribiendo XML (API Fluida)
Crea estructuras profundamente anidadas en una línea.

```go
m := xml.NewMap()

// Auto-crea jerarquía: Envelope -> Body -> Login
m.Set("Envelope/Body/Login/User", "admin")
m.Set("Envelope/Body/Login/Pass", "secret")
m.Set("Envelope/Body/Login/@type", "secure")

// Encode con indentación
s, _ := xml.Marshal(m, xml.WithPrettyPrint())
fmt.Println(s)
```

**Salida (Determinista):**
```xml
<Envelope>
  <Body>
    <Login type="secure">
      <User>admin</User>
      <Pass>secret</Pass>
    </Login>
  </Body>
</Envelope>
```

## 4. Arrays y Listas
XML es ambiguo: `<Item>` puede ser un objeto único o una lista de uno.
Usa `ForceArray` para resolver esto.

```go
m, _ := xml.MapXML(r, xml.ForceArray("Item"))

// Ahora "Item" es SIEMPRE una lista []*OrderedMap
items := m.List("Order/Item")
for _, item := range items {
    fmt.Println(item.String("Name"))
}
```

## 5. Herramientas CLI
Puedes usar la librería como una herramienta binaria `r2xml`.

**Pretty Print (Formatear):**
```bash
go run main.go fmt feo.xml
```

**Convertir a CSV:**
```bash
go run main.go csv data.xml --path="Order/Item" > items.csv
```

**Conversión JSON:**
```bash
go run main.go json data.xml
```
