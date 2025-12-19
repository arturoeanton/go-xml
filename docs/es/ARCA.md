# Integración ARCA (AFIP) vía CLI

Este tutorial explica cómo interactuar con los Web Services de ARCA (Factura Electrónica) utilizando únicamente la **Línea de Comandos** (`r2xml`) y archivos de configuración JSON.

Esto es ideal para pruebas rápidas, debugging o integración con sistemas legacy que pueden generar un JSON y ejecutar un binario.

## Prerrequisitos
1. **Binario `r2xml`**: Compilado (`go build -o r2xml main.go`) o usando `go run main.go`.
2. **Certificados**: Tu certificado `.crt` y clave privada `.key` (para generar el CMS de login).
3. **Acceso**: Haber vinculado el certificado al servicio `wsfe` en el portal de AFIP.

---

## 1. Autenticación (WSAA)
Para obtener el `Token` y `Sign`, necesitamos llamar al servicio de Login (WSAA).

### Configuración (`login.json`)
Prepara un archivo JSON con la configuración del servicio y el Ticket de Acceso (TRA) ya firmado (CMS/PKCS#7) en base64.

> **Nota**: La generación del CMS firmado se hace externamente (openssl) o con herramientas propias. Aquí asumimos que ya tienes el CMS en base64.

```json
{
  "endpoint": "https://wsaahomo.afip.gov.ar/ws/services/LoginCms",
  "namespace": "",
  "action": "loginCms",
  "payload": { 
    "in0": "MIIG4QYJKoZIhvcNAQcCoIIG0jC...." <-- CMS en base64 firmado
  },
  "output": "xml" 
}
```

### Ejecución
```bash
go run main.go soap login.json > auth_response.json
```

**Respuesta Esperada:**
```json
{
  "Envelope": {
    "Body": {
      "loginCmsResponse": {
        "loginCmsReturn": "<?xml version=\"1.0\" ... <token>...</token><sign>...</sign> ..."
      }
    }
  }
}
```


### Helper Login Script 

```bash
#!/bin/bash

# --- CONFIGURACIÓN ---
CERT="certificado.crt"
KEY="privada.key"
URL="https://wsaahomo.afip.gov.ar/ws/services/LoginCms"
SERVICE="wsfe"
# ---------------------

echo "1. Generando TRA UTC..."
GEN_TIME=$(date -u -v-10M +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d "-10 minutes" +%Y-%m-%dT%H:%M:%SZ)
EXP_TIME=$(date -u -v+10M +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d "+10 minutes" +%Y-%m-%dT%H:%M:%SZ)

TRA_XML="<loginTicketRequest version=\"1.0\"><header><uniqueId>$(date +%s)</uniqueId><generationTime>${GEN_TIME}</generationTime><expirationTime>${EXP_TIME}</expirationTime></header><service>$SERVICE</service></loginTicketRequest>"
echo "$TRA_XML" > tra.xml

echo "2. Firmando TRA..."
openssl cms -sign -in tra.xml -signer $CERT -inkey $KEY -nodetach -outform DER | base64 | tr -d '\n' > cms.b64
CMS_CONTENT=$(cat cms.b64)

cat <<EOF > arca_login.json
{
  "endpoint": "$URL",
  "namespace": "",
  "action": "loginCms",
  "payload": { "in0": "$CMS_CONTENT" },
  "output": "xml" 
}
EOF

echo "3. Solicitando Credenciales a AFIP..."
# Guardamos la respuesta cruda


RAW_RESP=$(go run main.go soap arca_login.json)
# 1. Limpiamos las entidades HTML (&lt; -> <) para que grep funcione
# Tambien quitamos &#xA; que son saltos de linea feos
CLEAN_XML=$(echo "$RAW_RESP" | sed 's/&lt;/</g; s/&gt;/>/g; s/&#xA;//g')

# 2. Extraer Token y Sign (Ahora sí funciona porque los tags son <token> reales)
TOKEN=$(echo "$CLEAN_XML" | grep -o '<token>.*</token>' | sed 's/<token>//;s/<\/token>//')
SIGN=$(echo "$CLEAN_XML" | grep -o '<sign>.*</sign>' | sed 's/<sign>//;s/<\/sign>//')

# 3. Generar JSON limpio final
cat <<EOF > credenciales.json
{
  "Token": "$TOKEN",
  "Sign": "$SIGN"
}
EOF

echo "✅ ÉXITO! Credenciales extraídas:"
cat credenciales.json
```


---

## 2. Autorizar Factura (WSFEv1)
Una vez que tienes `Token` y `Sign`, puedes autorizar un comprobante.

### Configuración (`factura.json`)
Aquí definimos el payload exacto para `FECAESolicitar`. Gracias a la arquitectura de `go-xml`, el orden de los campos se respetará estrictamente (Auth -> FeCabReq -> FeDetReq), evitando errores de esquema.

```json
{
    "endpoint": "https://wswhomo.afip.gov.ar/wsfev1/service.asmx",
    "namespace": "http://ar.gov.afip.dif.FEV1/",
    "action": "FECAESolicitar",
    "cert_file": "certificado.crt",
    "key_file": "privada.key",
    "payload": {
        "Auth": {
            "Token": "PD94bWwgdmVyc2lvbj0i....", <-- Token en base64
            "Sign": "Bf64.... ", <-- Sign en base64
            "Cuit": "20222...." <-- CUIT del Emisor 
        },
        "FeCAEReq": {
            "FeCabReq": {
                "CantReg": 1,
                "PtoVta": 1,
                "CbteTipo": 6
            },
            "FeDetReq": {
                "FECAEDetRequest": {
                    "Concepto": 1,
                    "DocTipo": 99,
                    "DocNro": 0,
                    "CbteDesde": 2,
                    "CbteHasta": 2,
                    "CbteFch": "20251219",
                    "ImpTotal": 121.00,
                    "CondicionIVAReceptorId": 5,
                    "ImpTotConc": 0,
                    "ImpNeto": 100.00,
                    "ImpOpEx": 0,
                    "ImpTrib": 0,
                    "ImpIVA": 21.00,
                    "MonId": "PES",
                    "MonCotiz": 1,
                    "Iva": {
                        "AlicIva": [
                            {
                                "Id": 5,
                                "BaseImp": 100.00,
                                "Importe": 21.00
                            }
                        ]
                    }
                }
            }
        }
    },
    "output": "json"
}
```

### Ejecución
```bash
# Ejecutar y guardar respuesta
go run main.go soap factura.json > resultado.json

# O ver solo el CAE usando 'jq' (herramienta externa recomendada)
go run main.go soap factura.json | jq '.Envelope.Body.FECAESolicitarResponse.FECAESolicitarResult.FeDetResp.FECAEDetResponse.CAE'
```

---

## Tips para Datos Sensibles
*   **No guardes tokens en el repo**: Usa variables de entorno o inyecta los valores en el JSON dinámicamente usando herramientas como `envsubst` antes de llamar a `r2xml`.
*   **Gitignore**: Agrega `*.json` (o un patrón como `*_secure.json`) a tu `.gitignore` para evitar subir credenciales reales.

## Debugging
Si recibes errores de SOAP (`Server was unable to read request`):
1.  Cambia `"output": "json"` a `"output": "xml"` en tu configuración.
2.  Ejecuta de nuevo para ver el XML crudo que se está enviando.
3.  Verifica que el orden de los campos coincida con manual de ARCA.
