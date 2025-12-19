package xml

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

// Helper to generate in-memory Cert and Key for testing
func generateTestKeys(t *testing.T) ([]byte, []byte) {
	// 1. Generate Private Key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	// 2. Create Certificate Template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
			CommonName:   "Test Cert",
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 1),

		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// 3. Create Self-Signed Certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	// 4. Encode to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyBytes := x509.MarshalPKCS1PrivateKey(priv)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})

	return certPEM, keyPEM
}

func TestNewSigner(t *testing.T) {
	certPEM, keyPEM := generateTestKeys(t)

	s, err := NewSigner(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("NewSigner failed: %v", err)
	}
	if s == nil {
		t.Fatal("Signer is nil")
	}
	if s.Cert == nil || s.Key == nil {
		t.Fatal("Signer Cert or Key is nil")
	}

	// Test Invalid PEM
	_, err = NewSigner([]byte("bad"), keyPEM)
	if err == nil {
		t.Error("Expected error for bad cert PEM, got nil")
	}
}

func TestCreateSignature_Standard(t *testing.T) {
	certPEM, keyPEM := generateTestKeys(t)
	s, _ := NewSigner(certPEM, keyPEM)

	xmlContent := []byte(`<root><data>hello</data></root>`)

	sigMap, err := s.CreateSignature(xmlContent)
	if err != nil {
		t.Fatalf("CreateSignature failed: %v", err)
	}

	// Basic Structure Verification
	// Should have ds:SignedInfo, ds:SignatureValue, ds:KeyInfo
	if sigMap.Get("ds:SignedInfo") == nil {
		t.Error("Missing ds:SignedInfo")
	}
	if sigMap.Get("ds:SignatureValue") == nil {
		t.Error("Missing ds:SignatureValue")
	}
	if sigMap.Get("ds:KeyInfo") == nil {
		t.Error("Missing ds:KeyInfo")
	}

	// Verify Digest Value roughly (we can't easily reproduce exact C14N without replicating logic)
	// But we can check that it didn't crash and produced base64 looking output.
	si := sigMap.Get("ds:SignedInfo").(*OrderedMap)
	ref := si.Get("ds:Reference").(*OrderedMap)
	digestVal := ref.Get("ds:DigestValue")
	if digestVal == "" {
		t.Error("DigestValue is empty")
	}
}

func TestCreateXadesSignature(t *testing.T) {
	certPEM, keyPEM := generateTestKeys(t)
	s, _ := NewSigner(certPEM, keyPEM)

	xmlContent := []byte(`<Invoice>...</Invoice>`)

	sigMap, err := s.CreateXadesSignature(xmlContent)
	if err != nil {
		t.Fatalf("CreateXadesSignature failed: %v", err)
	}

	// Check Structure
	if sigMap.Get("ds:Object") == nil {
		t.Error("Missing ds:Object (XAdES wrapper)")
	}

	obj := sigMap.Get("ds:Object").(*OrderedMap)
	qp := obj.Get("xades:QualifyingProperties")
	if qp == nil {
		t.Error("Missing xades:QualifyingProperties")
	}

	// Verify References (Should be slice with 2 valid references)
	si := sigMap.Get("ds:SignedInfo").(*OrderedMap)
	refs := si.Get("ds:Reference")

	refList, ok := refs.([]*OrderedMap)
	if !ok {
		// Maybe it's []interface{} or single item?
		// singer.go Line 319: signedInfo.Set("ds:Reference", []*OrderedMap{refDoc, refProps})
		t.Fatalf("ds:Reference is not []*OrderedMap, got %T", refs)
	}

	if len(refList) != 2 {
		t.Errorf("Expected 2 references (Doc + Props), got %d", len(refList))
	}
}
