package envelope

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/YusufDrymz/go-efatura/sign"
	"github.com/YusufDrymz/go-efatura/ubltr"
	"github.com/YusufDrymz/go-efatura/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testParties() (Party, Party) {
	return Party{Alias: "urn:mail:defaultgb@ornek.com.tr", VKN: "9990000005", Title: "Örnek A.Ş."},
		Party{Alias: "urn:mail:defaultpk@alici.com.tr", VKN: "9990000013", Title: "Alıcı Ltd."}
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

// Uctan uca: kur -> imzala -> zarfla -> zip'le -> ac. Belge baytlari
// birebir korunmali ve imza zarftan ciktiktan sonra da dogrulanmali.
func TestEndToEnd(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(7),
		Subject:      pkix.Name{CommonName: "Örnek Test Mührü"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	signer, err := sign.New(cert, key)
	require.NoError(t, err)
	signed, err := signer.Sign(context.Background(), testInvoiceXML(t))
	require.NoError(t, err)

	snd, rcv := testParties()
	env, err := Build(Envelope{Sender: snd, Receiver: rcv, Documents: [][]byte{signed}})
	require.NoError(t, err)

	opened, err := Open(env)
	require.NoError(t, err)
	require.Len(t, opened.Documents, 1)

	// imzali belge bayt bayt ayni cikmali (xml bildirimi haric, o zarfta olmaz)
	assert.True(t, bytes.Equal(stripXMLDecl(signed), opened.Documents[0]),
		"belge baytlari zarf gidis-donusunde degisti")

	// zarftan cikan imza hala gecerli
	res, err := sign.Verify(opened.Documents[0])
	require.NoError(t, err)
	assert.Equal(t, "Örnek Test Mührü", res.Certificate.Subject.CommonName)

	// belge parse edilip is kurallarindan geciyor
	inv, err := ubltr.ParseInvoice(opened.Documents[0])
	require.NoError(t, err)
	assert.Empty(t, validate.Invoice(inv))

	// zip gidis-donusu
	zipped, err := Zip(env, opened.ID)
	require.NoError(t, err)
	unzipped, err := OpenZip(zipped)
	require.NoError(t, err)
	assert.Equal(t, opened.ID, unzipped.ID)
	assert.True(t, bytes.Equal(opened.Documents[0], unzipped.Documents[0]))
}

func TestBuildHeader(t *testing.T) {
	snd, rcv := testParties()
	env, err := Build(Envelope{
		ID:       "e002da78-223f-438c-addb-16badeb047b5",
		Sender:   snd,
		Receiver: rcv,
		Created:  time.Date(2026, 7, 9, 4, 30, 0, 0, time.Local),
		Documents: [][]byte{
			testInvoiceXML(t),
		},
	})
	require.NoError(t, err)
	s := string(env)
	assert.Contains(t, s, "<sh:TypeVersion>1.2</sh:TypeVersion>") // schematron TypeVersionCheck
	assert.Contains(t, s, "PackageProxy_1_2.xsd")                 // schematron DocumentCheck
	assert.Contains(t, s, "<sh:Type>SENDERENVELOPE</sh:Type>")
	assert.Contains(t, s, "<ElementType>INVOICE</ElementType>")
	assert.Contains(t, s, "<ElementCount>1</ElementCount>")
	assert.Contains(t, s, "<sh:CreationDateAndTime>2026-07-09T04:30:00</sh:CreationDateAndTime>")
	assert.Contains(t, s, "<sh:ContactTypeIdentifier>VKN_TCKN</sh:ContactTypeIdentifier>")
}

func TestBuildErrors(t *testing.T) {
	snd, rcv := testParties()
	doc := []byte("<Invoice/>")
	cases := []struct {
		name string
		env  Envelope
		want string
	}{
		{"belge yok", Envelope{Sender: snd, Receiver: rcv}, "en az bir belge"},
		{"alias yok", Envelope{Sender: Party{VKN: "9990000005"}, Receiver: rcv, Documents: [][]byte{doc}}, "Alias ve VKN zorunlu"},
		{"vkn hane", Envelope{Sender: Party{Alias: "urn:mail:x@y", VKN: "123"}, Receiver: rcv, Documents: [][]byte{doc}}, "10 veya 11 haneli"},
		{"kotu id", Envelope{ID: "zarf-1", Sender: snd, Receiver: rcv, Documents: [][]byte{doc}}, "UUID formatinda"},
		{"kotu tur", Envelope{Type: "ZARF", Sender: snd, Receiver: rcv, Documents: [][]byte{doc}}, "gecersiz zarf turu"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Build(tc.env)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}

	t.Run("fatura limiti", func(t *testing.T) {
		docs := make([][]byte, 101)
		for i := range docs {
			docs[i] = doc
		}
		_, err := Build(Envelope{Sender: snd, Receiver: rcv, Documents: docs})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "en fazla 100 fatura")
	})
}

// Resmi GIB zarf ornegi acilabilmeli; icindeki imzali fatura oldugu gibi
// cikmali ve ubltr ile parse edilebilmeli.
func TestOpenOfficialEnvelope(t *testing.T) {
	data, err := os.ReadFile("testdata/gib/zarf-temelfatura.xml")
	require.NoError(t, err)

	env, err := Open(data)
	require.NoError(t, err)
	assert.Equal(t, "e002da78-223f-438c-addb-16badeb047b5", env.ID)
	assert.Equal(t, TypeSender, env.Type)
	assert.Equal(t, ElementInvoice, env.ElementType)
	assert.Equal(t, "urn:mail:defaultgb@gib.gov.tr", env.Sender.Alias)
	assert.Equal(t, "9999999999", env.Sender.VKN)
	assert.Equal(t, "e-Fatura Deneme A.Ş.", env.Sender.Title)
	require.Len(t, env.Documents, 1)

	inv, err := ubltr.ParseInvoice(env.Documents[0])
	require.NoError(t, err)
	assert.Equal(t, "TEMELFATURA", inv.ProfileID)

	// ham dilimleme: cikan belge orijinal zarf iceriginin birebir parcasi
	assert.True(t, bytes.Contains(data, env.Documents[0]))
}

func TestParseOfficialSystemResponse(t *testing.T) {
	data, err := os.ReadFile("testdata/gib/sistem-yaniti-merkez.xml")
	require.NoError(t, err)

	env, err := Open(data)
	require.NoError(t, err)
	assert.Equal(t, ElementApplicationResponse, env.ElementType)
	require.Len(t, env.Documents, 1)

	r, err := ParseResponse(env.Documents[0])
	require.NoError(t, err)
	assert.Equal(t, "e002da78-223f-438c-addb-16badeb047b5", r.EnvelopeID)
	assert.Equal(t, "SENDERENVELOPE", r.EnvelopeType)
	assert.Equal(t, 1200, r.Code)
	assert.Equal(t, "BASARIYLA ISLENDI", r.Description)
	assert.True(t, StatusSucceeded(r.Code))
}

func TestStatusPredicates(t *testing.T) {
	assert.Equal(t, "ZARF BASARIYLA ISLENDI", StatusText(1200))
	assert.Equal(t, "BILINMEYEN DURUM KODU (9)", StatusText(9))

	assert.True(t, StatusSucceeded(1300))
	assert.False(t, StatusSucceeded(1100))
	assert.True(t, StatusFailed(1150))
	assert.True(t, StatusFailed(1215))
	assert.False(t, StatusFailed(1200))
	assert.False(t, StatusFailed(1100)) // isleniyor: hata degil
	assert.True(t, StatusPending(1000))
	assert.True(t, StatusPending(1220))
	assert.False(t, StatusPending(1235)) // iptal: ne basari ne bekleme
}

func TestZipNamingRule(t *testing.T) {
	snd, rcv := testParties()
	env, err := Build(Envelope{Sender: snd, Receiver: rcv, Documents: [][]byte{testInvoiceXML(t)}})
	require.NoError(t, err)
	opened, err := Open(env)
	require.NoError(t, err)

	zipped, err := Zip(env, "yanlis-isim")
	require.NoError(t, err)
	_, err = OpenZip(zipped)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ayni olmali")

	zipped, err = Zip(env, opened.ID)
	require.NoError(t, err)
	_, err = OpenZip(zipped)
	require.NoError(t, err)
}
