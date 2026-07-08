package sign

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/russellhaering/goxmldsig/etreeutils"
)

// Signer, serialize edilmis belgeyi imzalayip XAdES'li halini dondurur.
// Entegrator kullanan cogunluk imzasiz uretir; bu interface sayesinde imza
// katmani cekirdege bulasmaz ve HSM gibi implementasyonlar takilabilir.
type Signer interface {
	Sign(ctx context.Context, doc []byte) ([]byte, error)
}

const (
	nsDS    = "http://www.w3.org/2000/09/xmldsig#"
	nsXAdES = "http://uri.etsi.org/01903/v1.3.2#"

	algC14N         = "http://www.w3.org/TR/2001/REC-xml-c14n-20010315"
	algC14NComments = algC14N + "#WithComments"
	algC14N11       = "http://www.w3.org/2006/12/xml-c14n11"
	algC14NExc      = "http://www.w3.org/2001/10/xml-exc-c14n#"
	algRSASHA256    = "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"
	algRSASHA1      = "http://www.w3.org/2000/09/xmldsig#rsa-sha1"
	algSHA256       = "http://www.w3.org/2001/04/xmlenc#sha256"
	algSHA1         = "http://www.w3.org/2000/09/xmldsig#sha1"
	algEnveloped    = "http://www.w3.org/2000/09/xmldsig#enveloped-signature"
	typeSignedProps = "http://uri.etsi.org/01903#SignedProperties"
)

// XAdESSigner dosya/bellek tabanli anahtar ile XAdES-BES uretir.
type XAdESSigner struct {
	cert  *x509.Certificate
	key   crypto.Signer
	sigID string
	now   func() time.Time
}

type Option func(*XAdESSigner)

// WithSignatureID ds:Signature Id degerini degistirir. Varsayilan
// "Signature", ubltr builder'in yazdigi cac:Signature URI'siyle (#Signature)
// eslesir.
func WithSignatureID(id string) Option { return func(s *XAdESSigner) { s.sigID = id } }

// WithTimeFunc SigningTime kaynagini degistirir (test determinizmi).
func WithTimeFunc(now func() time.Time) Option { return func(s *XAdESSigner) { s.now = now } }

// New bir XAdES imzalayici kurar. Simdilik yalniz RSA anahtarlar
// desteklenir (mali muhur sertifikalari RSA'dir).
func New(cert *x509.Certificate, key crypto.Signer, opts ...Option) (*XAdESSigner, error) {
	if cert == nil || key == nil {
		return nil, errors.New("sign: sertifika ve anahtar zorunlu")
	}
	if _, ok := key.Public().(*rsa.PublicKey); !ok {
		return nil, fmt.Errorf("sign: yalniz RSA anahtar destekleniyor (%T verildi)", key.Public())
	}
	s := &XAdESSigner{cert: cert, key: key, sigID: "Signature", now: time.Now}
	for _, o := range opts {
		o(s)
	}
	return s, nil
}

// Sign belgeyi XAdES-BES ile imzalar. Belgede ext:ExtensionContent bulunmali
// (ubltr.XML() ciktisi hazir gelir); icindeki placeholder imzayla degistirilir.
func (s *XAdESSigner) Sign(ctx context.Context, docBytes []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(docBytes); err != nil {
		return nil, fmt.Errorf("sign: belge parse edilemedi: %w", err)
	}
	content := findFirst(doc.Root(), "UBLExtensions", "UBLExtension", "ExtensionContent")
	if content == nil {
		return nil, errors.New("sign: ext:ExtensionContent bulunamadi (belge ubltr.XML() ile uretilmeli)")
	}
	for _, ch := range content.ChildElements() {
		content.RemoveChild(ch)
	}

	sig := s.buildSkeleton(content)

	// referans 1: URI="" — enveloped; digest, imzasiz belgenin C14N'i
	env, err := envelopedDigest(doc, s.sigID)
	if err != nil {
		return nil, err
	}
	setDigest(sig, "SignedInfo/Reference[0]", env)

	// referans 2: #SignedProperties — belge baglaminda (ns miras) C14N
	props := findFirst(sig, "Object", "QualifyingProperties", "SignedProperties")
	propDigest, err := digestInContext(props)
	if err != nil {
		return nil, err
	}
	setDigest(sig, "SignedInfo/Reference[1]", propDigest)

	// SignedInfo C14N + RSA-SHA256
	si := sig.SelectElement("ds:SignedInfo")
	canon, err := canonInContext(si, dsig.MakeC14N10RecCanonicalizer())
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(canon)
	sigVal, err := s.key.Sign(rand.Reader, sum[:], crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("sign: rsa imza: %w", err)
	}
	sig.SelectElement("ds:SignatureValue").SetText(base64.StdEncoding.EncodeToString(sigVal))

	doc.WriteSettings.CanonicalEndTags = false
	return doc.WriteToBytes()
}

// buildSkeleton, GIB imzali ornekleriyle ayni iskelette ds:Signature kurar
// (digest ve imza degerleri sonradan doldurulur).
func (s *XAdESSigner) buildSkeleton(parent *etree.Element) *etree.Element {
	certB64 := base64.StdEncoding.EncodeToString(s.cert.Raw)
	certSum := sha256.Sum256(s.cert.Raw)

	sig := parent.CreateElement("ds:Signature")
	sig.CreateAttr("xmlns:ds", nsDS)
	sig.CreateAttr("Id", s.sigID)

	si := sig.CreateElement("ds:SignedInfo")
	si.CreateElement("ds:CanonicalizationMethod").CreateAttr("Algorithm", algC14N)
	si.CreateElement("ds:SignatureMethod").CreateAttr("Algorithm", algRSASHA256)

	ref := si.CreateElement("ds:Reference")
	ref.CreateAttr("URI", "")
	// schematron TransformCountCheck: en fazla bir ds:Transform
	ref.CreateElement("ds:Transforms").CreateElement("ds:Transform").CreateAttr("Algorithm", algEnveloped)
	ref.CreateElement("ds:DigestMethod").CreateAttr("Algorithm", algSHA256)
	ref.CreateElement("ds:DigestValue")

	pref := si.CreateElement("ds:Reference")
	pref.CreateAttr("Id", "SignedProperties-Reference")
	pref.CreateAttr("Type", typeSignedProps)
	pref.CreateAttr("URI", "#SignedProperties")
	pref.CreateElement("ds:DigestMethod").CreateAttr("Algorithm", algSHA256)
	pref.CreateElement("ds:DigestValue")

	sig.CreateElement("ds:SignatureValue")

	ki := sig.CreateElement("ds:KeyInfo")
	x509data := ki.CreateElement("ds:X509Data")
	x509data.CreateElement("ds:X509SubjectName").SetText(s.cert.Subject.String())
	x509data.CreateElement("ds:X509Certificate").SetText(certB64)

	qp := sig.CreateElement("ds:Object").CreateElement("xades:QualifyingProperties")
	qp.CreateAttr("xmlns:xades", nsXAdES)
	qp.CreateAttr("Target", "#"+s.sigID)
	sp := qp.CreateElement("xades:SignedProperties")
	sp.CreateAttr("Id", "SignedProperties")
	ssp := sp.CreateElement("xades:SignedSignatureProperties")
	ssp.CreateElement("xades:SigningTime").SetText(s.now().UTC().Format("2006-01-02T15:04:05Z"))
	cert := ssp.CreateElement("xades:SigningCertificate").CreateElement("xades:Cert")
	cd := cert.CreateElement("xades:CertDigest")
	cd.CreateElement("ds:DigestMethod").CreateAttr("Algorithm", algSHA256)
	cd.CreateElement("ds:DigestValue").SetText(base64.StdEncoding.EncodeToString(certSum[:]))
	is := cert.CreateElement("xades:IssuerSerial")
	is.CreateElement("ds:X509IssuerName").SetText(s.cert.Issuer.String())
	is.CreateElement("ds:X509SerialNumber").SetText(s.cert.SerialNumber.String())
	return sig
}

// envelopedDigest imzasiz belge kopyasinin C14N sha256'sini hesaplar
// (enveloped-signature transform = imza elemani cikarilir).
func envelopedDigest(doc *etree.Document, sigID string) ([]byte, error) {
	cp := doc.Copy()
	sig := findSignature(cp.Root(), sigID)
	if sig == nil {
		return nil, errors.New("sign: kopyada imza bulunamadi")
	}
	sig.Parent().RemoveChild(sig)
	canon, err := dsig.MakeC14N10RecCanonicalizer().Canonicalize(cp.Root())
	if err != nil {
		return nil, fmt.Errorf("sign: c14n: %w", err)
	}
	sum := sha256.Sum256(canon)
	return sum[:], nil
}

// digestInContext elemani ata namespace baglamiyla birlikte kanonize edip
// sha256'sini dondurur. XAdES'in klasik tuzagi: SignedProperties digest'i
// belge icindeki haliyle (miras alinan xmlns'lerle) hesaplanmalidir.
func digestInContext(el *etree.Element) ([]byte, error) {
	canon, err := canonInContext(el, dsig.MakeC14N10RecCanonicalizer())
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(canon)
	return sum[:], nil
}

func canonInContext(el *etree.Element, c dsig.Canonicalizer) ([]byte, error) {
	ctx, err := etreeutils.NSBuildParentContext(el)
	if err != nil {
		return nil, fmt.Errorf("sign: ns baglami: %w", err)
	}
	detached, err := etreeutils.NSDetatch(ctx, el)
	if err != nil {
		return nil, fmt.Errorf("sign: ns detach: %w", err)
	}
	out, err := c.Canonicalize(detached)
	if err != nil {
		return nil, fmt.Errorf("sign: c14n: %w", err)
	}
	return out, nil
}

func setDigest(sig *etree.Element, which string, digest []byte) {
	refs := sig.SelectElement("ds:SignedInfo").SelectElements("ds:Reference")
	i := 0
	if strings.HasSuffix(which, "[1]") {
		i = 1
	}
	refs[i].SelectElement("ds:DigestValue").SetText(base64.StdEncoding.EncodeToString(digest))
}

// findFirst yerel isimle (prefix'ten bagimsiz) sirali cocuk arar.
func findFirst(el *etree.Element, names ...string) *etree.Element {
	cur := el
	for _, name := range names {
		var next *etree.Element
		for _, ch := range cur.ChildElements() {
			if ch.Tag == name {
				next = ch
				break
			}
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

// findSignature belgedeki ds:Signature'i bulur (Id eslesirse onu, yoksa
// ilk bulunani).
func findSignature(root *etree.Element, sigID string) *etree.Element {
	var first *etree.Element
	var walk func(*etree.Element) *etree.Element
	walk = func(el *etree.Element) *etree.Element {
		for _, ch := range el.ChildElements() {
			if ch.Tag == "Signature" && ch.NamespaceURI() == nsDS {
				if first == nil {
					first = ch
				}
				if sigID == "" || ch.SelectAttrValue("Id", "") == sigID {
					return ch
				}
			}
			if found := walk(ch); found != nil {
				return found
			}
		}
		return nil
	}
	if found := walk(root); found != nil {
		return found
	}
	return first
}
