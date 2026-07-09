package xml

import (
	"crypto/sha512"
	"fmt"
)

// CalculateCUFE generates the mandatory SHA-384 hash.
// ClaveTecnica: provided by DIAN in the enablement portal.
func CalculateCUFE(
	NumFac string, // SETT-100
	FecFac string, // 2025-12-19
	HorFac string, // 12:00:00-05:00
	ValFac string, // 1000.00 (Total without taxes)
	CodImp1 string, // 01 (VAT)
	ValImp1 string, // 190.00
	CodImp2 string, // 04 (Consumption) - use "04" if none, with value 0.00
	ValImp2 string, // 0.00
	ValTot string, // 1190.00 (Total + Taxes)
	NitEmi string, // 900123456
	NumAdq string, // 222222222222
	ClaveTec string, // Test technical key
	TipoAmb string, // 2 = Testing, 1 = Production
) string {

	// The DIAN formula is strict about this order:
	// NumFac + FecFac + HorFac + ValFac + CodImp1 + ValImp1 + CodImp2 + ValImp2 + ValTot + NitEmi + NumAdq + ClaveTec + TipoAmb

	raw := fmt.Sprintf("%s%s%s%s%s%s%s%s%s%s%s%s%s",
		NumFac, FecFac, HorFac, ValFac,
		CodImp1, ValImp1,
		CodImp2, ValImp2,
		ValTot,
		NitEmi, NumAdq, ClaveTec, TipoAmb)

	// Debugging: Print to verify what we are hashing
	// fmt.Println("DEBUG CUFE String:", raw)

	hash := sha512.Sum384([]byte(raw))
	return fmt.Sprintf("%x", hash)
}
