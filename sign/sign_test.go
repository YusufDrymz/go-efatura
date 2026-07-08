package sign

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/YusufDrymz/go-efatura/ubltr"
	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gecici self-signed sertifika: format testleri icin yeterli, zincir
// dogrulamasi zaten kapsam disi (Kamu SM test PFX'iyle smoke kullaniciya).
func testCert(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject:      pkix.Name{CommonName: "Örnek Yazılım A.Ş. Test Mührü", Organization: []string{"Örnek A.Ş."}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert, key
}

func testInvoiceXML(t *testing.T) []byte {
	t.Helper()
	b := ubltr.NewInvoice(
		ubltr.WithProfile(ubltr.ProfileTemelFatura),
		ubltr.WithType(ubltr.TypeSatis),
		ubltr.WithID("ABC2026000000001"),
		ubltr.WithUUID("F47AC10B-58CC-4372-A567-0E02B2C3D479"),
		ubltr.WithIssueDate(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)),
		ubltr.WithSupplier(ubltr.PartyInfo{
			VKN: "9990000005", Name: "Örnek A.Ş.", TaxOffice: "Beşiktaş",
			Address: ubltr.Address{CitySubdivisionName: "Beşiktaş", CityName: "İstanbul", Country: ubltr.Country{Name: "Türkiye"}},
		}),
		ubltr.WithCustomer(ubltr.PartyInfo{
			TCKN: "99900000074", FirstName: "Ali", FamilyName: "Yılmaz",
			Address: ubltr.Address{CitySubdivisionName: "Çankaya", CityName: "Ankara", Country: ubltr.Country{Name: "Türkiye"}},
		}),
	)
	b.AddLine(ubltr.Line{Name: "Danışmanlık", Qty: ubltr.D("2"), Unit: "C62", UnitPrice: ubltr.D("1500"), VATRate: ubltr.D("20")})
	inv, err := b.Build()
	require.NoError(t, err)
	out, err := inv.XML()
	require.NoError(t, err)
	return out
}

func signedInvoice(t *testing.T) ([]byte, *x509.Certificate) {
	t.Helper()
	cert, key := testCert(t)
	s, err := New(cert, key, WithTimeFunc(func() time.Time {
		return time.Date(2026, 7, 9, 3, 0, 0, 0, time.UTC)
	}))
	require.NoError(t, err)
	out, err := s.Sign(context.Background(), testInvoiceXML(t))
	require.NoError(t, err)
	return out, cert
}

func TestSignVerify(t *testing.T) {
	signed, cert := signedInvoice(t)

	res, err := Verify(signed)
	require.NoError(t, err)
	assert.Equal(t, cert.SerialNumber, res.Certificate.SerialNumber)
	assert.Equal(t, 2026, res.SigningTime.Year())
}

func TestSignStructure(t *testing.T) {
	signed, _ := signedInvoice(t)
	s := string(signed)

	assert.Contains(t, s, `<ds:Signature xmlns:ds="http://www.w3.org/2000/09/xmldsig#" Id="Signature">`)
	assert.Contains(t, s, `Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"`)
	assert.NotContains(t, s, "rsa-sha1") // SignatureMethodCheck: rsa-sha1 yasak
	assert.Contains(t, s, `Type="http://uri.etsi.org/01903#SignedProperties"`)
	assert.Contains(t, s, "<xades:SigningTime>2026-07-09T03:00:00Z</xades:SigningTime>")
	// TransformCountCheck: tek transform
	assert.Equal(t, 1, strings.Count(s, "<ds:Transform "))
	// placeholder imzayla degismis olmali
	assert.NotContains(t, s, "SignaturePlaceholder")
}

func TestSignedDocStillParses(t *testing.T) {
	signed, _ := signedInvoice(t)
	inv, err := ubltr.ParseInvoice(signed)
	require.NoError(t, err)
	assert.Equal(t, "ABC2026000000001", inv.ID)
	assert.Equal(t, "600", inv.TaxTotals[0].TaxAmount.Value.String())
}

func TestTamperDetected(t *testing.T) {
	signed, _ := signedInvoice(t)
	tampered := strings.Replace(string(signed), "Danışmanlık", "Damışmanlık", 1)
	require.NotEqual(t, string(signed), tampered)

	_, err := Verify([]byte(tampered))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "digest")
}

// GIB'in yayinladigi imzali ornek, yayin oncesi yeniden bicimlendirilmis
// (xml bildirimi bile silinmis) — enveloped digest bu yuzden HICBIR
// implementasyonda tutmaz ve Verify'in bunu yakalamasi gerekir. Ancak
// SignedInfo imzasinin kendisi ve #SignedProperties digest'i, imza blogu
// icinde dokunulmadan kaldigi icin dogrulanabilir: c14n hesabimizin gercek
// GIB imzasiyla byte-uyumlu oldugunun kaniti.
func TestOfficialSignedSample(t *testing.T) {
	data, err := os.ReadFile("testdata/gib/satis-temelfatura-kdv-sifir.xml")
	require.NoError(t, err)

	// strict Verify: yeniden bicimlendirilmis belge "degismis" sayilmali
	_, err = Verify(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "referans 1")

	// imza blogunun ic butunlugu: SignedInfo RSA imzasi + SignedProperties
	// digest'i bizim c14n ile dogrulanmali
	doc := etree.NewDocument()
	require.NoError(t, doc.ReadFromBytes(data))
	sig := findSignature(doc.Root(), "")
	require.NotNil(t, sig)

	cert, err := embeddedCert(sig)
	require.NoError(t, err)
	assert.Equal(t, "e-Fatura Deneme A.Ş.", cert.Subject.CommonName)

	si := sig.SelectElement("ds:SignedInfo")
	canon, err := canonInContext(si, dsig.MakeC14N10WithCommentsCanonicalizer())
	require.NoError(t, err)
	sum := sha256.Sum256(canon)
	sigVal, err := base64.StdEncoding.DecodeString(textOf(sig, "ds:SignatureValue"))
	require.NoError(t, err)
	require.NoError(t, rsa.VerifyPKCS1v15(cert.PublicKey.(*rsa.PublicKey), crypto.SHA256, sum[:], sigVal),
		"gercek GIB imzasi bizim SignedInfo c14n'imizle dogrulanmali")

	props := findByID(doc.Root(), "SignedProperties")
	require.NotNil(t, props)
	got, err := digestInContext(props)
	require.NoError(t, err)
	var want []byte
	for _, ref := range si.SelectElements("ds:Reference") {
		if ref.SelectAttrValue("URI", "") == "#SignedProperties" {
			want, err = base64.StdEncoding.DecodeString(textOf(ref, "ds:DigestValue"))
			require.NoError(t, err)
		}
	}
	assert.Equal(t, want, got, "SignedProperties digest'i gercek imzayla eslesmiyor")
}

func TestVerifyRejectsUnsigned(t *testing.T) {
	_, err := Verify(testInvoiceXML(t))
	require.Error(t, err)
}

func TestNewRejectsNonRSA(t *testing.T) {
	cert, _ := testCert(t)
	_, err := New(cert, nil)
	assert.Error(t, err)
}
