package xml

import (
	"crypto/sha512"
	"fmt"
)

// CalculateCUFE genera el hash SHA-384 obligatorio.
// ClaveTecnica: Te la da la DIAN en el portal de habilitación.
func CalculateCUFE(
	NumFac string, // SETT-100
	FecFac string, // 2025-12-19
	HorFac string, // 12:00:00-05:00
	ValFac string, // 1000.00 (Total sin imp)
	CodImp1 string, // 01 (IVA)
	ValImp1 string, // 190.00
	CodImp2 string, // 04 (Consumo) - poner "04" si no hay, con valor 0.00
	ValImp2 string, // 0.00
	ValTot string, // 1190.00 (Total + Imp)
	NitEmi string, // 900123456
	NumAdq string, // 222222222222
	ClaveTec string, // Clave tecnica de pruebas
	TipoAmb string, // 2 = Pruebas, 1 = Producción
) string {

	// La fórmula de la DIAN es estricta en este orden:
	// NumFac + FecFac + HorFac + ValFac + CodImp1 + ValImp1 + CodImp2 + ValImp2 + ValTot + NitEmi + NumAdq + ClaveTec + TipoAmb

	raw := fmt.Sprintf("%s%s%s%s%s%s%s%s%s%s%s%s%s",
		NumFac, FecFac, HorFac, ValFac,
		CodImp1, ValImp1,
		CodImp2, ValImp2,
		ValTot,
		NitEmi, NumAdq, ClaveTec, TipoAmb)

	// Depuración: Imprimir para verificar qué estamos hasheando
	// fmt.Println("DEBUG CUFE String:", raw)

	hash := sha512.Sum384([]byte(raw))
	return fmt.Sprintf("%x", hash)
}
