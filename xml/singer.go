package xml

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"time"
)

// ============================================================================
// SIGNER CORE
// ============================================================================

type Signer struct {
	Cert *x509.Certificate
	Key  *rsa.PrivateKey
}

func NewSigner(certPEM, keyPEM []byte) (*Signer, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse x509 certificate: %w", err)
	}

	blockKey, _ := pem.Decode(keyPEM)
	if blockKey == nil {
		return nil, fmt.Errorf("failed to decode private key PEM")
	}
	key, err := x509.ParsePKCS1PrivateKey(blockKey.Bytes)
	if err != nil {
		keyInterface, err2 := x509.ParsePKCS8PrivateKey(blockKey.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("failed to parse private key: %v", err)
		}
		var ok bool
		key, ok = keyInterface.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
	}
	return &Signer{Cert: cert, Key: key}, nil
}

// ============================================================================
// MODO 1: XML-DSig (Estándar Simple)
// ============================================================================

func (s *Signer) CreateSignature(xmlContent []byte) (*OrderedMap, error) {
	// 1. Digest
	hash := sha256.Sum256(xmlContent)
	digestValue := base64.StdEncoding.EncodeToString(hash[:])

	// 2. SignedInfo
	signedInfo := NewMap()

	cMethod := NewMap()
	cMethod.Set("@Algorithm", "http://www.w3.org/TR/2001/REC-xml-c14n-20010315")
	signedInfo.Set("ds:CanonicalizationMethod", cMethod)

	sMethod := NewMap()
	sMethod.Set("@Algorithm", "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256")
	signedInfo.Set("ds:SignatureMethod", sMethod)

	ref := NewMap()
	ref.Set("@URI", "")

	transforms := NewMap()
	t1 := NewMap()
	t1.Set("@Algorithm", "http://www.w3.org/2000/09/xmldsig#enveloped-signature")
	transforms.Set("ds:Transform", t1)
	ref.Set("ds:Transforms", transforms)

	dMethod := NewMap()
	dMethod.Set("@Algorithm", "http://www.w3.org/2001/04/xmlenc#sha256")
	ref.Set("ds:DigestMethod", dMethod)
	ref.Set("ds:DigestValue", digestValue)

	signedInfo.Set("ds:Reference", ref)

	// 3. Firmar
	wrapper := NewMap()
	signedInfo.Set("@xmlns:ds", "http://www.w3.org/2000/09/xmldsig#")
	wrapper.Set("ds:SignedInfo", signedInfo)

	siBytes, err := Marshal(wrapper) // Esto devuelve STRING
	if err != nil {
		return nil, err
	}

	// CORRECCIÓN 1: Convertir string a []byte
	hashedSI := sha256.Sum256([]byte(siBytes))

	sigBytes, err := rsa.SignPKCS1v15(rand.Reader, s.Key, crypto.SHA256, hashedSI[:])
	if err != nil {
		return nil, err
	}

	// 4. Armar
	dsSig := NewMap()
	dsSig.Set("@xmlns:ds", "http://www.w3.org/2000/09/xmldsig#")
	dsSig.Set("ds:SignedInfo", signedInfo)
	dsSig.Set("ds:SignatureValue", base64.StdEncoding.EncodeToString(sigBytes))

	ki := NewMap()
	xd := NewMap()
	xd.Set("ds:X509Certificate", base64.StdEncoding.EncodeToString(s.Cert.Raw))
	ki.Set("ds:X509Data", xd)
	dsSig.Set("ds:KeyInfo", ki)

	return dsSig, nil
}

// ============================================================================
// MODO 2: XAdES-BES (DIAN / Factura Electrónica Avanzada)
// ============================================================================
/*
func (s *Signer) CreateXadesSignature(xmlContent []byte) (*OrderedMap, error) {
	uniqueID := fmt.Sprintf("%d", time.Now().Unix())
	signatureID := "Signature-" + uniqueID
	sigPropsID := "SignedProperties-" + uniqueID
	xadesNS := "http://uri.etsi.org/01903/v1.3.2#"

	// --- 1. Preparar Propiedades Firmadas (XAdES) ---
	certHash := sha256.Sum256(s.Cert.Raw)

	signedProperties := NewMap()
	signedProperties.Set("@Id", sigPropsID)

	// SigningTime
	sigSigProps := NewMap()
	sigSigProps.Set("xades:SigningTime", time.Now().Format("2006-01-02T15:04:05"))

	// SigningCertificate
	signingCert := NewMap()
	certDef := NewMap()
	cd := NewMap()
	cd.Set("ds:DigestMethod/@Algorithm", "http://www.w3.org/2001/04/xmlenc#sha256")
	cd.Set("ds:DigestValue", base64.StdEncoding.EncodeToString(certHash[:]))
	certDef.Set("xades:CertDigest", cd)

	is := NewMap()
	is.Set("ds:X509IssuerName", s.Cert.Issuer.String())
	is.Set("ds:X509SerialNumber", s.Cert.SerialNumber.String())
	certDef.Set("xades:IssuerSerial", is)

	signingCert.Set("xades:Cert", certDef)
	sigSigProps.Set("xades:SigningCertificate", signingCert)
	signedProperties.Set("xades:SignedSignatureProperties", sigSigProps)

	// --- 2. Hashear Documento y Propiedades ---
	docHash := sha256.Sum256(xmlContent)

	// Hashear SignedProperties (Canonicalización simulada)
	xpWrapper := NewMap()
	xpWrapper.Set("@xmlns:xades", xadesNS)
	xpWrapper.Set("@xmlns:ds", "http://www.w3.org/2000/09/xmldsig#")
	xpWrapper.Set("xades:SignedProperties", signedProperties)

	xpBytes, _ := Marshal(xpWrapper) // Devuelve STRING

	// CORRECCIÓN 2: Convertir string a []byte
	propsHash := sha256.Sum256([]byte(xpBytes))

	// --- 3. Construir SignedInfo (Con doble referencia) ---
	signedInfo := NewMap()
	signedInfo.Set("ds:CanonicalizationMethod/@Algorithm", "http://www.w3.org/TR/2001/REC-xml-c14n-20010315")
	signedInfo.Set("ds:SignatureMethod/@Algorithm", "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256")

	// Ref 1: Documento
	refDoc := NewMap()
	refDoc.Set("@URI", "")
	tr := NewMap()
	t1 := NewMap()
	t1.Set("@Algorithm", "http://www.w3.org/2000/09/xmldsig#enveloped-signature")
	tr.Set("ds:Transform", t1)
	refDoc.Set("ds:Transforms", tr)
	refDoc.Set("ds:DigestMethod/@Algorithm", "http://www.w3.org/2001/04/xmlenc#sha256")
	refDoc.Set("ds:DigestValue", base64.StdEncoding.EncodeToString(docHash[:]))

	// Ref 2: Propiedades
	refProps := NewMap()
	refProps.Set("@URI", "#"+sigPropsID)
	refProps.Set("@Type", "http://uri.etsi.org/01903#SignedProperties")
	refProps.Set("ds:DigestMethod/@Algorithm", "http://www.w3.org/2001/04/xmlenc#sha256")
	refProps.Set("ds:DigestValue", base64.StdEncoding.EncodeToString(propsHash[:]))

	// IMPORTANTE: Slice de referencias
	signedInfo.Set("ds:Reference", []*OrderedMap{refDoc, refProps})

	// --- 4. Firmar ---
	wrapper := NewMap()
	signedInfo.Set("@xmlns:ds", "http://www.w3.org/2000/09/xmldsig#")
	wrapper.Set("ds:SignedInfo", signedInfo)

	siBytes, err := Marshal(wrapper) // Devuelve STRING
	if err != nil {
		return nil, err
	}

	// CORRECCIÓN 3: Convertir string a []byte
	siHash := sha256.Sum256([]byte(siBytes))

	sigBytes, err := rsa.SignPKCS1v15(rand.Reader, s.Key, crypto.SHA256, siHash[:])
	if err != nil {
		return nil, err
	}

	// --- 5. Armar Final ---
	finalSig := NewMap()
	finalSig.Set("@xmlns:ds", "http://www.w3.org/2000/09/xmldsig#")
	finalSig.Set("@Id", signatureID)
	finalSig.Set("ds:SignedInfo", signedInfo)
	finalSig.Set("ds:SignatureValue", base64.StdEncoding.EncodeToString(sigBytes))

	ki := NewMap()
	xd := NewMap()
	xd.Set("ds:X509Certificate", base64.StdEncoding.EncodeToString(s.Cert.Raw))
	ki.Set("ds:X509Data", xd)
	finalSig.Set("ds:KeyInfo", ki)

	// Object (XAdES)
	obj := NewMap()
	qp := NewMap()
	qp.Set("@xmlns:xades", xadesNS)
	qp.Set("@Target", "#"+signatureID)
	qp.Set("xades:SignedProperties", signedProperties)
	obj.Set("xades:QualifyingProperties", qp)

	finalSig.Set("ds:Object", obj)

	return finalSig, nil
}//*/

func (s *Signer) CreateXadesSignature(xmlContent []byte) (*OrderedMap, error) {
	uniqueID := fmt.Sprintf("%d", time.Now().Unix())
	signatureID := "Signature-" + uniqueID
	sigPropsID := "SignedProperties-" + uniqueID
	xadesNS := "http://uri.etsi.org/01903/v1.3.2#"

	// --- 1. Preparar Propiedades Firmadas (XAdES) ---
	// Este hash es binario puro del certificado, no requiere C14N
	certHash := sha256.Sum256(s.Cert.Raw)

	signedProperties := NewMap()
	signedProperties.Set("@Id", sigPropsID)

	// SigningTime
	sigSigProps := NewMap()
	sigSigProps.Set("xades:SigningTime", time.Now().Format("2006-01-02T15:04:05"))

	// SigningCertificate
	signingCert := NewMap()
	certDef := NewMap()
	cd := NewMap()
	cd.Set("ds:DigestMethod/@Algorithm", "http://www.w3.org/2001/04/xmlenc#sha256")
	cd.Set("ds:DigestValue", base64.StdEncoding.EncodeToString(certHash[:]))
	certDef.Set("xades:CertDigest", cd)

	is := NewMap()
	is.Set("ds:X509IssuerName", s.Cert.Issuer.String())
	is.Set("ds:X509SerialNumber", s.Cert.SerialNumber.String())
	certDef.Set("xades:IssuerSerial", is)

	signingCert.Set("xades:Cert", certDef)
	sigSigProps.Set("xades:SigningCertificate", signingCert)
	signedProperties.Set("xades:SignedSignatureProperties", sigSigProps)

	// --- 2. Hashear Documento y Propiedades ---
	docHash := sha256.Sum256(xmlContent)

	// Preparar Wrapper para C14N de Propiedades
	xpWrapper := NewMap()
	xpWrapper.Set("@xmlns:xades", xadesNS)
	xpWrapper.Set("@xmlns:ds", "http://www.w3.org/2000/09/xmldsig#")
	xpWrapper.Set("xades:SignedProperties", signedProperties)

	// ✅ CAMBIO 1: Usar Canonicalize en lugar de Marshal
	// Esto ordena atributos y gestiona espacios según C14N
	xpBytes, err := Canonicalize(xpWrapper)
	if err != nil {
		return nil, fmt.Errorf("error canonicalizing properties: %w", err)
	}

	propsHash := sha256.Sum256(xpBytes)

	// --- 3. Construir SignedInfo (Con doble referencia) ---
	signedInfo := NewMap()
	// Aquí declaramos el algoritmo que AHORA SÍ estamos cumpliendo
	signedInfo.Set("ds:CanonicalizationMethod/@Algorithm", "http://www.w3.org/TR/2001/REC-xml-c14n-20010315")
	signedInfo.Set("ds:SignatureMethod/@Algorithm", "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256")

	// Ref 1: Documento (Factura)
	refDoc := NewMap()
	refDoc.Set("@URI", "")
	tr := NewMap()
	t1 := NewMap()
	t1.Set("@Algorithm", "http://www.w3.org/2000/09/xmldsig#enveloped-signature")
	tr.Set("ds:Transform", t1)
	refDoc.Set("ds:Transforms", tr)
	refDoc.Set("ds:DigestMethod/@Algorithm", "http://www.w3.org/2001/04/xmlenc#sha256")
	refDoc.Set("ds:DigestValue", base64.StdEncoding.EncodeToString(docHash[:]))

	// Ref 2: Propiedades (SignedProperties)
	refProps := NewMap()
	refProps.Set("@URI", "#"+sigPropsID)
	refProps.Set("@Type", "http://uri.etsi.org/01903#SignedProperties")
	refProps.Set("ds:DigestMethod/@Algorithm", "http://www.w3.org/2001/04/xmlenc#sha256")
	refProps.Set("ds:DigestValue", base64.StdEncoding.EncodeToString(propsHash[:]))

	signedInfo.Set("ds:Reference", []*OrderedMap{refDoc, refProps})

	// --- 4. Firmar ---
	wrapper := NewMap()
	// El namespace ds es vital en el wrapper para que C14N lo propague
	signedInfo.Set("@xmlns:ds", "http://www.w3.org/2000/09/xmldsig#")
	wrapper.Set("ds:SignedInfo", signedInfo)

	// ✅ CAMBIO 2: Usar Canonicalize para el SignedInfo
	// El hash resultante será el que firmaremos con RSA
	siBytes, err := Canonicalize(wrapper)
	if err != nil {
		return nil, fmt.Errorf("error canonicalizing signedinfo: %w", err)
	}

	siHash := sha256.Sum256(siBytes)

	sigBytes, err := rsa.SignPKCS1v15(rand.Reader, s.Key, crypto.SHA256, siHash[:])
	if err != nil {
		return nil, err
	}

	// --- 5. Armar Final ---
	finalSig := NewMap()
	finalSig.Set("@xmlns:ds", "http://www.w3.org/2000/09/xmldsig#")
	finalSig.Set("@Id", signatureID)
	finalSig.Set("ds:SignedInfo", signedInfo)
	finalSig.Set("ds:SignatureValue", base64.StdEncoding.EncodeToString(sigBytes))

	ki := NewMap()
	xd := NewMap()
	xd.Set("ds:X509Certificate", base64.StdEncoding.EncodeToString(s.Cert.Raw))
	ki.Set("ds:X509Data", xd)
	finalSig.Set("ds:KeyInfo", ki)

	// Object (XAdES)
	obj := NewMap()
	qp := NewMap()
	qp.Set("@xmlns:xades", xadesNS)
	qp.Set("@Target", "#"+signatureID)
	qp.Set("xades:SignedProperties", signedProperties)
	obj.Set("xades:QualifyingProperties", qp)

	finalSig.Set("ds:Object", obj)

	return finalSig, nil
}
