# Roadmap de go-xml

Este documento detalla las funcionalidades faltantes y mejoras planificadas para el proyecto, ordenadas por impacto y complejidad.

## 🚀 Alta Prioridad (Alto Impacto / Complejidad Baja-Media)


### 2. Mejora en Reporte de Errores
**Impacto: Alto** | **Complejidad: Baja**

Los errores de validación y parseo son genéricos ("parsing error").
- **Necesidad**: Exponer número de línea y columna donde ocurrió el error en el tipo `xml.Error`, esencial para depurar archivos grandes o mal formados.

### 3. Soporte de Wildcards en Query
**Impacto: Alto** | **Complejidad: Media**

Navegar listas dinámicas donde las claves son desconocidas es difícil actualmente (requiere iteración manual).
- **Necesidad**: Soportar el comodín `*` en rutas de `Query`, ej: `invoice/items/*/sku` para obtener todos los SKUs sin importar el tag contenedor.

## 🔮 Media Prioridad (Funcionalidades Estratégicas)

### 4. Extracción de Nodo Crudo (Canonicalización)
**Impacto: Medio/Alto** | **Complejidad: Media**

Usuarios empresariales (bancos, crypto) a menudo necesitan el string fuente *inalterado* de un nodo específico para verificar firmas digitales (HMAC/RSA).
- **Necesidad**: Mecanismo para extraer los bytes crudos de un nodo (ej: `<signedInfo>...</signedInfo>`) durante el parseo.

### 5. Generación de Structs (CLI)
**Impacto: Bajo** | **Complejidad: Media**

Aunque la filosofía es "no usar structs", a veces la migración o interoperabilidad los requiere.
- **Necesidad**: Un comando CLI (`go run main.go gen-struct data.xml`) que infiera y genere el código Go de los structs basándose en un XML de muestra.

## 🧊 Baja Prioridad / Futuro (Alta Complejidad / Nicho)

### 6. Soporte XPath 1.0 Completo
**Impacto: Medio** | **Complejidad: Alta**

El sistema actual de `Query` es suficiente para el 90% de los casos. XPath 1.0 completo implica soportar ejes (`following-sibling`, `ancestor`) y funciones (`count()`, `contains()`).
- **Necesidad**: Esperar demanda de usuarios antes de implementar un motor completo.

### 7. Validación contra XSD (Schema)
**Impacto: Medio** | **Complejidad: Muy Alta**

Validar contra un archivo XSD estándar es el estándar de oro pero extremadamente complejo de implementar.
- **Necesidad**: Evaluar wrappers de C (libxml2) si la validación estricta es crítica.

### 8. Soporte Híbrido (Marshal/Unmarshal)
**Impacto: Bajo** | **Complejidad: Media**

Permitir usar `MapXML` como un paso intermedio para luego decodificar en un struct estándar de Go.
