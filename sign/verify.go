package sign

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
)

// VerifyResult basarili dogrulamanin ozeti. Zincir dogrulamasi yapilmaz;
// sertifikaya guven karari (Kamu SM koku, sure, iptal) cagirana aittir.
type VerifyResult struct {
	Certificate *x509.Certificate
	SigningTime time.Time
}

// Verify belgedeki XAdES imzasini dogrular: referans digest'leri yeniden
// hesaplanir ve SignedInfo imzasi KeyInfo'daki sertifikayla kontrol edilir.
// Eski belgeler icin rsa-sha1 dogrulamada kabul edilir (uretimde asla).
func Verify(docBytes []byte) (*VerifyResult, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(docBytes); err != nil {
		return nil, fmt.Errorf("sign: belge parse edilemedi: %w", err)
	}
	sig := findSignature(doc.Root(), "")
	if sig == nil {
		return nil, errors.New("sign: belgede ds:Signature yok")
	}
	si := sig.SelectElement("ds:SignedInfo")
	if si == nil {
		return nil, errors.New("sign: ds:SignedInfo yok")
	}

	cert, err := embeddedCert(sig)
	if err != nil {
		return nil, err
	}

	c14nAlg := algAttr(si, "ds:CanonicalizationMethod")
	canonizer, err := canonicalizerFor(c14nAlg)
	if err != nil {
		return nil, err
	}

	for i, ref := range si.SelectElements("ds:Reference") {
		if err := verifyReference(doc, sig, ref, canonizer); err != nil {
			return nil, fmt.Errorf("sign: referans %d: %w", i+1, err)
		}
	}

	canon, err := canonInContext(si, canonizer)
	if err != nil {
		return nil, err
	}
	sigVal, err := base64.StdEncoding.DecodeString(textOf(sig, "ds:SignatureValue"))
	if err != nil {
		return nil, fmt.Errorf("sign: SignatureValue base64: %w", err)
	}
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("sign: yalniz RSA dogrulaniyor (%T)", cert.PublicKey)
	}
	switch alg := algAttr(si, "ds:SignatureMethod"); alg {
	case algRSASHA256:
		sum := sha256.Sum256(canon)
		err = rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sigVal)
	case algRSASHA1:
		sum := sha1.Sum(canon)
		err = rsa.VerifyPKCS1v15(pub, crypto.SHA1, sum[:], sigVal)
	default:
		return nil, fmt.Errorf("sign: desteklenmeyen imza algoritmasi %q", alg)
	}
	if err != nil {
		return nil, fmt.Errorf("sign: imza degeri gecersiz: %w", err)
	}

	res := &VerifyResult{Certificate: cert}
	if st := findFirst(sig, "Object", "QualifyingProperties", "SignedProperties", "SignedSignatureProperties", "SigningTime"); st != nil {
		res.SigningTime, _ = time.Parse(time.RFC3339, st.Text())
	}
	return res, nil
}

func verifyReference(doc *etree.Document, sig, ref *etree.Element, canonizer dsig.Canonicalizer) error {
	uri := ref.SelectAttrValue("URI", "")
	want, err := base64.StdEncoding.DecodeString(textOf(ref, "ds:DigestValue"))
	if err != nil {
		return fmt.Errorf("DigestValue base64: %w", err)
	}

	var canon []byte
	switch {
	case uri == "":
		// enveloped: imzasiz belge kopyasi
		cp := doc.Copy()
		cpSig := findSignature(cp.Root(), sig.SelectAttrValue("Id", ""))
		if cpSig == nil {
			return errors.New("kopyada imza bulunamadi")
		}
		cpSig.Parent().RemoveChild(cpSig)
		canon, err = dsig.MakeC14N10RecCanonicalizer().Canonicalize(cp.Root())
	case uri[0] == '#':
		target := findByID(doc.Root(), uri[1:])
		if target == nil {
			return fmt.Errorf("URI %q hedefi bulunamadi", uri)
		}
		canon, err = canonInContext(target, canonizer)
	default:
		return fmt.Errorf("desteklenmeyen URI %q", uri)
	}
	if err != nil {
		return err
	}

	var got []byte
	switch alg := algAttr(ref, "ds:DigestMethod"); alg {
	case algSHA256:
		sum := sha256.Sum256(canon)
		got = sum[:]
	case algSHA1:
		sum := sha1.Sum(canon)
		got = sum[:]
	default:
		return fmt.Errorf("desteklenmeyen digest algoritmasi %q", alg)
	}
	if !bytes.Equal(got, want) {
		return errors.New("digest uyusmuyor (belge imzadan sonra degismis olabilir)")
	}
	return nil
}

func canonicalizerFor(alg string) (dsig.Canonicalizer, error) {
	switch alg {
	case algC14N:
		return dsig.MakeC14N10RecCanonicalizer(), nil
	case algC14NComments:
		return dsig.MakeC14N10WithCommentsCanonicalizer(), nil
	case algC14N11:
		return dsig.MakeC14N11Canonicalizer(), nil
	case algC14NExc:
		return dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList(""), nil
	}
	return nil, fmt.Errorf("sign: desteklenmeyen c14n algoritmasi %q", alg)
}

func embeddedCert(sig *etree.Element) (*x509.Certificate, error) {
	certEl := findFirst(sig, "KeyInfo", "X509Data", "X509Certificate")
	if certEl == nil {
		return nil, errors.New("sign: KeyInfo/X509Data/X509Certificate yok")
	}
	der, err := base64.StdEncoding.DecodeString(compactWS(certEl.Text()))
	if err != nil {
		return nil, fmt.Errorf("sign: sertifika base64: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("sign: sertifika parse: %w", err)
	}
	return cert, nil
}

func findByID(root *etree.Element, id string) *etree.Element {
	if root.SelectAttrValue("Id", "") == id {
		return root
	}
	for _, ch := range root.ChildElements() {
		if found := findByID(ch, id); found != nil {
			return found
		}
	}
	return nil
}

func algAttr(parent *etree.Element, child string) string {
	if el := parent.SelectElement(child); el != nil {
		return el.SelectAttrValue("Algorithm", "")
	}
	return ""
}

func textOf(parent *etree.Element, child string) string {
	if el := parent.SelectElement(child); el != nil {
		return compactWS(el.Text())
	}
	return ""
}

func compactWS(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\n', '\r':
		default:
			out = append(out, s[i])
		}
	}
	return string(out)
}
