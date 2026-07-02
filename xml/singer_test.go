package xml

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
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

// buildSignableDoc mirrors the pattern demo.go uses: build the document,
// marshal it BEFORE the signature exists (that's what gets referenced), sign
// it, embed the returned signature, and re-marshal.
func buildSignableDoc(t *testing.T) (doc, inner *OrderedMap) {
	t.Helper()
	inner = NewMap()
	inner.Set("@xmlns", "urn:test:invoice")
	inner.Set("@xmlns:ds", dsigNS)
	inner.Set("ID", "SETT-100")
	inner.Set("Amount", "1000.00")

	doc = NewMap()
	doc.Set("Root", inner)
	return doc, inner
}

func TestSigner_CreateSignature_VerifyRoundTrip(t *testing.T) {
	certPEM, keyPEM := generateTestKeys(t)
	s, _ := NewSigner(certPEM, keyPEM)

	doc, inner := buildSignableDoc(t)
	preSignBytes, err := Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal (pre-sign) error: %v", err)
	}

	sig, err := s.CreateSignature([]byte(preSignBytes))
	if err != nil {
		t.Fatalf("CreateSignature error: %v", err)
	}
	inner.Set("ds:Signature", sig)

	finalXML, err := Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal (final) error: %v", err)
	}

	if err := s.Verify([]byte(finalXML)); err != nil {
		t.Fatalf("Verify failed on a signature this Signer just produced: %v\nXML: %s", err, finalXML)
	}
}

func TestSigner_CreateXadesSignature_VerifyRoundTrip(t *testing.T) {
	certPEM, keyPEM := generateTestKeys(t)
	s, _ := NewSigner(certPEM, keyPEM)

	doc, inner := buildSignableDoc(t)
	preSignBytes, err := Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal (pre-sign) error: %v", err)
	}

	sig, err := s.CreateXadesSignature([]byte(preSignBytes))
	if err != nil {
		t.Fatalf("CreateXadesSignature error: %v", err)
	}
	inner.Set("ds:Signature", sig)

	finalXML, err := Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal (final) error: %v", err)
	}

	if err := s.Verify([]byte(finalXML)); err != nil {
		t.Fatalf("Verify failed on a XAdES signature this Signer just produced: %v\nXML: %s", err, finalXML)
	}
}

// tamperFirstChar replaces the first character of the text content of the
// first <tag>...</tag> found in xmlStr with a different, still-valid
// character, guaranteeing the content changes without altering its length.
func tamperFirstChar(t *testing.T, xmlStr, tag string) string {
	t.Helper()
	open := "<" + tag + ">"
	start := strings.Index(xmlStr, open)
	if start == -1 {
		t.Fatalf("tag %q not found in: %s", tag, xmlStr)
	}
	start += len(open)
	end := strings.Index(xmlStr[start:], "<")
	if end <= 0 {
		t.Fatalf("empty or malformed content for tag %q", tag)
	}
	content := []byte(xmlStr[start : start+end])
	alt := byte('A')
	if content[0] == 'A' {
		alt = 'B'
	}
	content[0] = alt
	return xmlStr[:start] + string(content) + xmlStr[start+end:]
}

func TestSigner_Verify_DetectsTamperedDocument(t *testing.T) {
	certPEM, keyPEM := generateTestKeys(t)
	s, _ := NewSigner(certPEM, keyPEM)

	doc, inner := buildSignableDoc(t)
	preSignBytes, _ := Marshal(doc)
	sig, err := s.CreateXadesSignature([]byte(preSignBytes))
	if err != nil {
		t.Fatalf("CreateXadesSignature error: %v", err)
	}
	inner.Set("ds:Signature", sig)
	finalXML, _ := Marshal(doc)

	tampered := tamperFirstChar(t, finalXML, "ID")

	err = s.Verify([]byte(tampered))
	if err == nil {
		t.Fatal("expected Verify to fail on a tampered document, got nil")
	}
	if !strings.Contains(err.Error(), "digest mismatch") {
		t.Errorf("expected a digest mismatch error, got: %v", err)
	}
}

func TestSigner_Verify_DetectsTamperedSignatureValue(t *testing.T) {
	certPEM, keyPEM := generateTestKeys(t)
	s, _ := NewSigner(certPEM, keyPEM)

	doc, inner := buildSignableDoc(t)
	preSignBytes, _ := Marshal(doc)
	sig, err := s.CreateXadesSignature([]byte(preSignBytes))
	if err != nil {
		t.Fatalf("CreateXadesSignature error: %v", err)
	}
	inner.Set("ds:Signature", sig)
	finalXML, _ := Marshal(doc)

	tampered := tamperFirstChar(t, finalXML, "ds:SignatureValue")

	err = s.Verify([]byte(tampered))
	if err == nil {
		t.Fatal("expected Verify to fail on a tampered SignatureValue, got nil")
	}
	if !strings.Contains(err.Error(), "signature does not match") {
		t.Errorf("expected a signature mismatch error, got: %v", err)
	}
}
