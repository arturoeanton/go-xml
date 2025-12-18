# Tutorial de Consultas Avanzadas en go-xml

`go-xml` ofrece un Motor de Consultas inspirado en XPath pero optimizado para la estructura `map[string]any` en Go. Este tutorial cubre técnicas avanzadas, incluyendo Wildcards, funcionalidades XPath-Lite y Funciones Personalizadas.

## 1. Wildcards (`*`)

Usa `*` para iterar sobre todos los nodos "hijos" de un mapa, excluyendo atributos (`@...`) y claves de metadatos (`#...`). Esto es perfecto para mapas dinámicos donde las claves son IDs o nombres desconocidos.

### Escenario: Claves Dinámicas (Factura)
```xml
<invoice>
    <items>
        <box_001> <sku>A1</sku> </box_001>
        <bag_002> <sku>B2</sku> </bag_002>
    </items>
</invoice>
```

**Consulta**:
```go
// Obtener TODOS los SKUs, sin importar la clave contenedora (box_001, bag_002)
skus, _ := xml.QueryAll(m, "invoice/items/*/sku")
// Salida: ["A1", "B2"]
```

---

## 2. Características XPath-Lite

Implementamos la regla "80/20" de XPath: las características más útiles sin la sobrecarga de un motor completo.

### A. Búsqueda Profunda (`//`)
Busca recursivamente en toda la estructura de datos claves que coincidan con el nombre.

**Consulta**:
```go
// Encontrar TODOS los nodos "error" en cualquier parte del documento
errs, _ := xml.QueryAll(m, "//error")
```

### B. Operadores de Filtro
Soporte para operadores de comparación estándar: `=`, `!=`, `>`, `<`, `>=`, `<=`.

**Consulta**:
```go
// Encontrar libros más baratos que $10
cheapBooks, _ := xml.QueryAll(m, "library/book[price<10]/title")

// Encontrar usuarios activos (estado no es inactivo)
activeUsers, _ := xml.QueryAll(m, "users/user[status!=inactive]")
```

### C. Funciones de Filtro
Funciones de string integradas dentro de los filtros:
-   `contains(key, 'value')`
-   `starts-with(key, 'value')`

**Consulta**:
```go
// Encontrar usuarios con email de gmail
gmailUsers, _ := xml.QueryAll(m, "users/user[contains(email, '@gmail.com')]")
```

### D. Agregación (`#count`)
Retorna la cantidad de hijos en una lista o mapa.

**Consulta**:
```go
// Contar número de libros
count, _ := xml.Query(m, "library/book/#count")
```

---

## 3. Funciones de Consulta Personalizadas (`func:...`)

Si las herramientas estándar no son suficientes, puedes registrar funciones Go personalizadas para filtrar claves.

### Paso 1: Registra tu Función
```go
import (
    "strings"
    "github.com/arturoeanton/go-xml/xml"
)

func init() {
    // Registrar una función que retorna true para claves que empiezan con "iphone"
    xml.RegisterQueryFunction("isIphone", func(key string) bool {
        return strings.HasPrefix(key, "iphone")
    })
}
```

### Paso 2: Usar en Consulta
Usa la sintaxis `func:nombre` en tu ruta.

**Consulta**:
```go
// Seleccionar solo claves que pasen "isIphone", luego obtener su "model"
models, _ := xml.QueryAll(m, "products/func:isIphone/model")
```

### Funciones Integradas
Proporcionamos 15 funciones de utilidad listas para usar (ver `xml/features_query.go`):
-   `isNumeric`: Claves como "123".
-   `isUUID`: Claves como "550e8400...".
-   `isSnakeCase`, `isCamelCase`: Para validación estructural.
-   `hasDigits`, `isAlpha`, etc.

**Ejemplo**:
```go
// Iterar solo sobre claves numéricas (ignorando metadatos o claves de texto)
ids, _ := xml.QueryAll(m, "data/func:isNumeric/id")
```
