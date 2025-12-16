# Roadmap de go-xml

Este documento detalla las funcionalidades faltantes y mejoras planificadas para el proyecto, ordenadas por impacto y complejidad.

## 🚀 Alta Prioridad



### 3. Mejora en Reporte de Errores
**Impacto: Medio** | **Complejidad: Baja**

Los errores de validación y parseo son genéricos.
- **Necesidad**: Exponer número de línea y columna donde ocurrió el error, especialmente útil para archivos grandes o mal formados.

## 🔮 Media Prioridad

### 4. XPath 1.0 Completo
**Impacto: Medio/Alto** | **Complejidad: Alta**

El sistema actual de Query (`users/user[0]/name`) es potente pero limitado. No soporta ejes complejos (`following-sibling`, `ancestor`) ni funciones XPath (`count()`, `contains()`).
- **Necesidad**: Evaluar si implementar un motor XPath real o seguir extendiendo el mini-lenguaje actual.

### 5. Generación de Structs (CLI)
**Impacto: Bajo** | **Complejidad: Media**

Aunque la filosofía es "no usar structs", a veces se necesita migrar o interoperar con sistemas que sí los usan.
- **Necesidad**: Un comando CLI (`go run main.go gen-struct data.xml`) que infiera y genere el código Go de los structs basándose en un XML de muestra.

## 🧊 Baja Prioridad / Futuro

### 6. Validación contra XSD (Schema)
**Impacto: Medio** | **Complejidad: Muy Alta**

Validar contra un archivo XSD estándar es extremadamente complejo de implementar desde cero, pero es el estándar de oro en la industria.
- **Necesidad**: Integrar soporte parcial o wrappers de C para validación estricta si el usuario lo requiere.

### 7. Soporte Híbrido (Marshal/Unmarshal)
**Impacto: Bajo** | **Complejidad: Media**

Permitir usar `MapXML` como un paso intermedio para luego decodificar en un struct estándar de Go, para usuarios que quieren lo mejor de los dos mundos.
