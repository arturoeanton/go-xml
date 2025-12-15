package xml

import (
	"encoding/json"
)

// ToJSON convierte el mapa XML a un string JSON (útil para debug o APIs)
func ToJSON(data map[string]any) (string, error) {
	b, err := json.Marshal(data)
	return string(b), err
}

// MapToStruct convierte el mapa dinámico a un Struct definido por el usuario.
// El truco más limpio y robusto es usar JSON como intermediario.
func MapToStruct(data map[string]any, result any) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBytes, result)
}

// Merge mezcla dos mapas (data2 sobre data1)
func Merge(base, override map[string]any) {
	for k, v := range override {
		// Si ambos son mapas, merge recursivo
		if vMap, ok := v.(map[string]any); ok {
			if baseMap, ok := base[k].(map[string]any); ok {
				Merge(baseMap, vMap)
				continue
			}
		}
		// Si no, sobrescribir
		base[k] = v
	}
}
