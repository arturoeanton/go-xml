package xml

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
	"time"
)

// ============================================================================
// SIGNER CORE
// ============================================================================

const dsigNS = "http://www.w3.org/2000/09/xmldsig#"

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
// MODE 1: XML-DSig (Simple Standard)
// ============================================================================

func (s *Signer) CreateSignature(xmlContent []byte) (*OrderedMap, error) {
	// 1. Digest: the Reference URI="" points to the whole document. Since
	// xmlContent is captured BEFORE inserting the signature, this already
	// implements the enveloped-signature transform (there is no ds:Signature
	// to remove yet); all that remains is to canonicalize it, exactly as the
	// algorithm declares.
	canonicalDoc, err := CanonicalizeXML(xmlContent)
	if err != nil {
		return nil, fmt.Errorf("error canonicalizing document: %w", err)
	}
	hash := sha256.Sum256(canonicalDoc)
	digestValue := base64.StdEncoding.EncodeToString(hash[:])

	// 2. SignedInfo
	signedInfo := NewMap()

	cMethod := NewMap()
	cMethod.Set("@Algorithm", ExclusiveC14NAlgorithm)
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

	// 3. Sign: SignedInfo is canonicalized as the root of its own
	// canonicalization (see Canonicalize), consistent with what
	// ds:CanonicalizationMethod declares and with what Verify() will
	// recompute later.
	wrapper := NewMap()
	signedInfo.Set("@xmlns:ds", dsigNS)
	wrapper.Set("ds:SignedInfo", signedInfo)

	siBytes, err := Canonicalize(wrapper)
	if err != nil {
		return nil, fmt.Errorf("error canonicalizing signedinfo: %w", err)
	}

	hashedSI := sha256.Sum256(siBytes)

	sigBytes, err := rsa.SignPKCS1v15(rand.Reader, s.Key, crypto.SHA256, hashedSI[:])
	if err != nil {
		return nil, err
	}

	// 4. Assemble
	dsSig := NewMap()
	dsSig.Set("@xmlns:ds", dsigNS)
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
// MODE 2: XAdES-BES (DIAN / Advanced Electronic Invoice)
// ============================================================================

func (s *Signer) CreateXadesSignature(xmlContent []byte) (*OrderedMap, error) {
	uniqueID := fmt.Sprintf("%d", time.Now().Unix())
	signatureID := "Signature-" + uniqueID
	sigPropsID := "SignedProperties-" + uniqueID
	xadesNS := "http://uri.etsi.org/01903/v1.3.2#"

	// --- 1. Prepare Signed Properties (XAdES) ---
	// This hash is the raw binary of the certificate, it does not require C14N
	certHash := sha256.Sum256(s.Cert.Raw)

	signedProperties := NewMap()
	signedProperties.Set("@Id", sigPropsID)
	// The namespaces go directly on the node that will be the marshaled root
	// (not on an outer wrapper): Marshal/Encode silently drops any attribute
	// placed alongside the single top-level root key, so declaring them here
	// is mandatory, not cosmetic.
	signedProperties.Set("@xmlns:xades", xadesNS)
	signedProperties.Set("@xmlns:ds", dsigNS)

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

	// --- 2. Hash Document and Properties ---
	// xmlContent is captured BEFORE inserting the signature: canonicalizing
	// it as-is already implements the enveloped-signature transform.
	canonicalDoc, err := CanonicalizeXML(xmlContent)
	if err != nil {
		return nil, fmt.Errorf("error canonicalizing document: %w", err)
	}
	docHash := sha256.Sum256(canonicalDoc)

	xpWrapper := NewMap()
	xpWrapper.Put("xades:SignedProperties", signedProperties)

	xpBytes, err := Canonicalize(xpWrapper)
	if err != nil {
		return nil, fmt.Errorf("error canonicalizing properties: %w", err)
	}

	propsHash := sha256.Sum256(xpBytes)

	// --- 3. Build SignedInfo (With double reference) ---
	signedInfo := NewMap()
	signedInfo.Set("ds:CanonicalizationMethod/@Algorithm", ExclusiveC14NAlgorithm)
	signedInfo.Set("ds:SignatureMethod/@Algorithm", "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256")

	// Ref 1: Document (Invoice)
	refDoc := NewMap()
	refDoc.Set("@URI", "")
	tr := NewMap()
	t1 := NewMap()
	t1.Set("@Algorithm", "http://www.w3.org/2000/09/xmldsig#enveloped-signature")
	tr.Set("ds:Transform", t1)
	refDoc.Set("ds:Transforms", tr)
	refDoc.Set("ds:DigestMethod/@Algorithm", "http://www.w3.org/2001/04/xmlenc#sha256")
	refDoc.Set("ds:DigestValue", base64.StdEncoding.EncodeToString(docHash[:]))

	// Ref 2: Properties (SignedProperties)
	refProps := NewMap()
	refProps.Set("@URI", "#"+sigPropsID)
	refProps.Set("@Type", "http://uri.etsi.org/01903#SignedProperties")
	refProps.Set("ds:DigestMethod/@Algorithm", "http://www.w3.org/2001/04/xmlenc#sha256")
	refProps.Set("ds:DigestValue", base64.StdEncoding.EncodeToString(propsHash[:]))

	signedInfo.Set("ds:Reference", []*OrderedMap{refDoc, refProps})

	// --- 4. Sign ---
	wrapper := NewMap()
	// The ds namespace is vital on the wrapper so that C14N propagates it
	signedInfo.Set("@xmlns:ds", dsigNS)
	wrapper.Set("ds:SignedInfo", signedInfo)

	siBytes, err := Canonicalize(wrapper)
	if err != nil {
		return nil, fmt.Errorf("error canonicalizing signedinfo: %w", err)
	}

	siHash := sha256.Sum256(siBytes)

	sigBytes, err := rsa.SignPKCS1v15(rand.Reader, s.Key, crypto.SHA256, siHash[:])
	if err != nil {
		return nil, err
	}

	// --- 5. Final Assembly ---
	finalSig := NewMap()
	finalSig.Set("@xmlns:ds", dsigNS)
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

// ============================================================================
// VERIFICATION
// ============================================================================

// Verify recomputes and checks an enveloped XML-DSig signature (as produced
// by CreateSignature or CreateXadesSignature and embedded into a document)
// against signedXML. It returns nil if every Reference digest matches and
// the RSA signature over SignedInfo verifies against the embedded X509
// certificate, or a descriptive error otherwise.
func (s *Signer) Verify(signedXML []byte) error {
	root, err := parseC14NTree(signedXML)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}

	sigNode := findElementNS(root, dsigNS, "Signature")
	if sigNode == nil {
		return fmt.Errorf("verify: no ds:Signature element found")
	}
	siNode := findElementNS(sigNode, dsigNS, "SignedInfo")
	if siNode == nil {
		return fmt.Errorf("verify: ds:SignedInfo not found")
	}

	// 1. Verify each Reference (the whole document and, if present,
	// fragments referenced by Id, e.g. xades:SignedProperties).
	refs := findChildrenNS(siNode, dsigNS, "Reference")
	if len(refs) == 0 {
		return fmt.Errorf("verify: no ds:Reference elements found")
	}
	for _, ref := range refs {
		uri := attrValue(ref, "URI")
		digestNode := findElementNS(ref, dsigNS, "DigestValue")
		if digestNode == nil {
			return fmt.Errorf("verify: ds:Reference (URI=%q) missing ds:DigestValue", uri)
		}
		wantDigest, err := base64.StdEncoding.DecodeString(nodeText(digestNode))
		if err != nil {
			return fmt.Errorf("verify: invalid DigestValue for Reference (URI=%q): %w", uri, err)
		}

		var gotDigest [32]byte
		if uri == "" {
			// Whole document with the enveloped-signature transform:
			// ds:Signature is removed from the tree and the rest is
			// canonicalized.
			stripped := cloneWithoutFirst(root, dsigNS, "Signature")
			canon, err := renderCanonicalized(stripped)
			if err != nil {
				return fmt.Errorf("verify: canonicalizing document: %w", err)
			}
			gotDigest = sha256.Sum256(canon)
		} else if strings.HasPrefix(uri, "#") {
			id := strings.TrimPrefix(uri, "#")
			target := findByID(root, id)
			if target == nil {
				return fmt.Errorf("verify: Reference target #%s not found", id)
			}
			canon, err := renderCanonicalized(target)
			if err != nil {
				return fmt.Errorf("verify: canonicalizing referenced element #%s: %w", id, err)
			}
			gotDigest = sha256.Sum256(canon)
		} else {
			return fmt.Errorf("verify: unsupported Reference URI %q", uri)
		}

		if !bytes.Equal(gotDigest[:], wantDigest) {
			return fmt.Errorf("verify: digest mismatch for Reference (URI=%q)", uri)
		}
	}

	// 2. Verify the RSA signature over SignedInfo, canonicalized in
	// isolation (just as was done when signing).
	siCanon, err := renderCanonicalized(siNode)
	if err != nil {
		return fmt.Errorf("verify: canonicalizing SignedInfo: %w", err)
	}
	siHash := sha256.Sum256(siCanon)

	sigValueNode := findElementNS(sigNode, dsigNS, "SignatureValue")
	if sigValueNode == nil {
		return fmt.Errorf("verify: ds:SignatureValue not found")
	}
	sigBytes, err := base64.StdEncoding.DecodeString(nodeText(sigValueNode))
	if err != nil {
		return fmt.Errorf("verify: invalid SignatureValue: %w", err)
	}

	certNode := findElementNS(sigNode, dsigNS, "X509Certificate")
	if certNode == nil {
		return fmt.Errorf("verify: ds:X509Certificate not found")
	}
	certBytes, err := base64.StdEncoding.DecodeString(nodeText(certNode))
	if err != nil {
		return fmt.Errorf("verify: invalid X509Certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return fmt.Errorf("verify: parsing embedded certificate: %w", err)
	}
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("verify: embedded certificate does not use an RSA key")
	}

	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, siHash[:], sigBytes); err != nil {
		return fmt.Errorf("verify: signature does not match: %w", err)
	}
	return nil
}
