package xml

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// Helper para obtener el Reader (File o Stdin)
func getInputReader(args []string) (io.Reader, error) {
	// Si hay argumentos y el primero no es un flag, asumimos que es el archivo
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		f, err := os.Open(args[0])
		if err != nil {
			return nil, err
		}
		return f, nil
	}

	// Si no, verificar Stdin
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		return os.Stdin, nil
	}

	return nil, fmt.Errorf("no input provided (pipe or file)")
}

// 1. Formatter (Pretty Print)
func CliFormat(args []string) {
	r, err := getInputReader(args)
	if err != nil {
		die(err)
	}

	// Leemos a OrderedMap
	m, err := MapXML(r, EnableLegacyCharsets()) // Robustez por defecto
	if err != nil {
		die(err)
	}

	// Escribimos con PrettyPrint
	enc := NewEncoder(os.Stdout, WithPrettyPrint())
	if err := enc.Encode(m); err != nil {
		die(err)
	}
	fmt.Println()
}

// 2. JSON Converter
func CliToJson(args []string) {
	r, err := getInputReader(args)
	if err != nil {
		die(err)
	}
	// Usamos ToJSON helper
	b, err := ToJSON(r)
	if err != nil {
		die(err)
	}
	fmt.Println(string(b))
}

// 3. CSV Converter (Flatten Lists)
// Uso: r2xml csv data.xml --path="orders/order"
func CliToCsv(args []string) {
	var targetPath string
	// Parse args manual simple
	cleanArgs := []string{}
	for _, a := range args {
		if strings.HasPrefix(a, "--path=") {
			targetPath = strings.TrimPrefix(a, "--path=")
		} else {
			cleanArgs = append(cleanArgs, a)
		}
	}

	if targetPath == "" {
		die(fmt.Errorf("parameter --path=\"node/list\" is required for CSV"))
	}

	r, err := getInputReader(cleanArgs)
	if err != nil {
		die(err)
	}

	// Forzamos array en el target path para asegurar lista
	nodeName := getLastSegment(targetPath)
	m, err := MapXML(r, ForceArray(nodeName))
	if err != nil {
		die(err)
	}

	// Extraer la lista
	list := m.List(targetPath)
	if len(list) == 0 {
		fmt.Fprintln(os.Stderr, "No rows found at path:", targetPath)
		return
	}

	// Convertir
	if err := ToCSV(os.Stdout, list); err != nil {
		die(err)
	}
}

// 4. Query
func CliQuery(args []string) {
	if len(args) < 1 {
		die(fmt.Errorf("xpath argument required"))
	}

	// El query suele ser el último arg o el segundo si hay archivo
	xpath := args[len(args)-1]
	fileArgs := args[:len(args)-1]

	r, err := getInputReader(fileArgs)
	if err != nil {
		die(err)
	}

	m, err := MapXML(r)
	if err != nil {
		die(err)
	}

	res, err := QueryAll(m, xpath)
	if err != nil {
		die(err)
	}

	// Salida JSON bonita de los resultados
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(res)
}

// 5. SOAP Client (Desde Config JSON)
// Permite probar servicios SOAP sin compilar Go.
// Input: Un JSON con {endpoint, action, payload, auth...}
func CliSoap(args []string) {
	r, err := getInputReader(args)
	if err != nil {
		die(err)
	}

	// 1. Leer Config JSON
	var cfg struct {
		Endpoint  string         `json:"endpoint"`
		Namespace string         `json:"namespace"`
		Action    string         `json:"action"`
		Payload   map[string]any `json:"payload"`
		Auth      struct {
			Type string `json:"type"` // basic, wsse
			User string `json:"user"`
			Pass string `json:"pass"`
		} `json:"auth"`
	}

	dec := json.NewDecoder(r)
	if err := dec.Decode(&cfg); err != nil {
		die(fmt.Errorf("invalid json config: %w", err))
	}

	// 2. Configurar Cliente
	opts := []ClientOption{}
	if cfg.Auth.Type == "wsse" {
		opts = append(opts, WithWSSecurity(cfg.Auth.User, cfg.Auth.Pass))
	} else if cfg.Auth.Type == "basic" {
		opts = append(opts, WithBasicAuth(cfg.Auth.User, cfg.Auth.Pass))
	}

	client := NewSoapClient(cfg.Endpoint, cfg.Namespace, opts...)

	// 3. Ejecutar
	resp, err := client.Call(cfg.Action, cfg.Payload)
	if err != nil {
		die(err)
	}

	// 4. Salida
	fmt.Println(resp.Dump())
}

func die(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}

func getLastSegment(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

// 6. SOAP Client Quick (Flags)
// Uso: r2xml call --url=http://... --action=GetData --data="User/Id=123" --data="User/Active=true"
func CliSoapQuick(args []string) {
	// Definimos un FlagSet independiente para no ensuciar el global
	fs := flag.NewFlagSet("call", flag.ExitOnError)

	var url, action, ns, user, pass, authType string
	var dataFlags arrayFlags // Tipo custom para soportar múltiples --data

	fs.StringVar(&url, "url", "", "SOAP Endpoint URL")
	fs.StringVar(&action, "action", "", "SOAP Action (Method Name)")
	fs.StringVar(&ns, "ns", "", "XML Namespace")
	fs.StringVar(&authType, "auth", "none", "Auth type: basic, wsse")
	fs.StringVar(&user, "user", "", "Username")
	fs.StringVar(&pass, "pass", "", "Password")

	// Bind del flag repetible
	fs.Var(&dataFlags, "data", "Payload key=value (Ex: --data 'User/Id=100'). Can be repeated.")

	fs.Parse(args)

	// Validaciones básicas
	if url == "" || action == "" {
		die(fmt.Errorf("required flags: --url and --action"))
	}

	// 1. Construir Cliente
	opts := []ClientOption{}
	if authType == "wsse" {
		opts = append(opts, WithWSSecurity(user, pass))
	} else if authType == "basic" {
		opts = append(opts, WithBasicAuth(user, pass))
	}

	// Si no pasaron namespace, intentamos usar la URL base o vacío
	if ns == "" {
		// A veces funciona vacío, a veces no.
		// Mejor dejarlo vacío si el usuario no lo pone.
	}

	client := NewSoapClient(url, ns, opts...)

	// 2. Construir Payload usando OrderedMap.Set (¡La Magia!)
	payload := NewMap()
	for _, d := range dataFlags {
		// Separar Key=Value
		// Usamos SplitN por si el valor contiene '=' (ej: token=abc=def)
		parts := strings.SplitN(d, "=", 2)
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "Warning: ignoring invalid data format '%s'. Use key=value\n", d)
			continue
		}

		key := parts[0]
		val := parts[1]

		// Opcional: Inferencia de tipos básica para la CLI
		// Si parece número, mandarlo como int/float?
		// Por seguridad en SOAP, String suele ser lo más compatible si no tenemos WSDL.
		// Pero si quieres soportar bools/ints:
		payload.Set(key, inferCLIValue(val))
	}

	// 3. Ejecutar
	resp, err := client.Call(action, payload)
	if err != nil {
		die(err)
	}

	// 4. Salida
	fmt.Println(resp.Dump())
}

// Helper para flags repetibles (--data "a=1" --data "b=2")
type arrayFlags []string

func (i *arrayFlags) String() string {
	return "my string representation"
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

// inferCLIValue intenta convertir strings de consola a tipos Go útiles
func inferCLIValue(val string) any {
	if val == "true" {
		return true
	}
	if val == "false" {
		return false
	}
	// Intenta int
	// (Ojo: strconv requiere importarlo si no está ya)
	// return val // Por ahora devolvemos string para máxima compatibilidad
	return val
}
