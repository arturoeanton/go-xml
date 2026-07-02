package xml

import (
	"crypto/tls"
	"fmt"
)

// ============================================================================
// CERTIFICATE UTILITIES (Standard PEM Support)
// ============================================================================

// LoadCert carga un par de certificados (Pública + Privada) desde archivos PEM (.crt / .key).
// Este es el formato nativo soportado por Go y la mayoría de servidores Linux/Docker.
func LoadCert(certFile, keyFile string) (tls.Certificate, error) {
	// Usamos la librería estándar.
	// Esto espera que los archivos estén en formato PEM (texto plano con headers -----BEGIN CERTIFICATE-----)
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to load x509 key pair: %w", err)
	}
	return cert, nil
}

// LoadP12Cert is intentionally unimplemented: parsing PKCS#12 requires a
// dependency this package deliberately doesn't have (go.mod has zero
// requires). Convert the .p12/.pfx to PEM once with OpenSSL and use LoadCert
// or NewSigner instead:
//
//	openssl pkcs12 -in cert.p12 -out cert.pem -clcerts -nokeys
//	openssl pkcs12 -in cert.p12 -out key.pem -nocerts -nodes
func LoadP12Cert(path, password string) (tls.Certificate, error) {
	return tls.Certificate{}, fmt.Errorf("P12 support not implemented: convert to PEM with 'openssl pkcs12 -in %s -out cert.pem -clcerts -nokeys' and 'openssl pkcs12 -in %s -out key.pem -nocerts -nodes', then use LoadCert", path, path)
}
