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

// LoadP12Cert es un placeholder para el futuro.
// En el Paso 2 implementaremos esto agregando la dependencia externa.
func LoadP12Cert(path, password string) (tls.Certificate, error) {
	return tls.Certificate{}, fmt.Errorf("P12 support not installed yet (requires external dependency)")
}
