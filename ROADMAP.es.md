# Roadmap de go-xml

Este documento detalla las funcionalidades faltantes y mejoras planificadas para el proyecto, ordenadas por impacto y complejidad.

## 🚀 Alta Prioridad (Alto Impacto / Complejidad Baja-Media)



### 3. Soporte de Wildcards en Query [COMPLETADO]
**Impacto: COMPLETADO** | **Complejidad: COMPLETADO**

*Implementado en v1.1*: Soporta comodín `*` en rutas `Query`, ej: `invoice/items/*/sku`.

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

### 6. Soporte XPath 1.0 Completo [PARCIALMENTE COMPLETADO]
**Impacto: Medio** | **Complejidad: Alta**

*Actualización v1.1*: Implementado "XPath-Lite" cubriendo casos de uso comunes:
- Búsqueda Profunda (`//nodo`).
- Operadores (`>`, `<`, `!=`) dentro de filtros.
- Funciones (`contains()`, `starts-with()`).
- Agregación (`#count`).
- Wildcards (`*`).
- Registro de Funciones Personalizadas (`items/func:miFunc/id`).

Ejes completos de XPath como `following-sibling` se posponen hasta nueva demanda.

### 7. Validación contra XSD (Schema)
**Impacto: Medio** | **Complejidad: Muy Alta**

Validar contra un archivo XSD estándar es el estándar de oro pero extremadamente complejo de implementar.
- **Necesidad**: Evaluar wrappers de C (libxml2) si la validación estricta es crítica.

### 8. Soporte Híbrido (Marshal/Unmarshal)
**Impacto: Bajo** | **Complejidad: Media**

Permitir usar `MapXML` como un paso intermedio para luego decodificar en un struct estándar de Go.
