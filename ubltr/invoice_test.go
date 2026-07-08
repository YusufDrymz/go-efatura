package ubltr

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/gib/" + name)
	require.NoError(t, err)
	return data
}

func TestParseInvoiceTemelFatura(t *testing.T) {
	inv, err := ParseInvoice(readFixture(t, "satis-temelfatura.xml"))
	require.NoError(t, err)

	assert.Equal(t, "2.1", inv.UBLVersionID)
	assert.Equal(t, "TR1.2", inv.CustomizationID)
	assert.Equal(t, "TEMELFATURA", inv.ProfileID)
	assert.Equal(t, "SATIS", inv.InvoiceTypeCode)
	assert.Equal(t, "GIB20090000000001", inv.ID)
	assert.Equal(t, "F47AC10B-58CC-4372-A567-0E02B2C3D479", inv.UUID)
	assert.Equal(t, "2009-01-05", inv.IssueDate)
	assert.Equal(t, "TRY", inv.DocumentCurrencyCode)
	assert.Equal(t, 1, inv.LineCountNumeric)

	sup := inv.AccountingSupplierParty.Party
	require.NotEmpty(t, sup.PartyIdentifications)
	assert.Equal(t, "VKN", sup.PartyIdentifications[0].ID.SchemeID)
	assert.Equal(t, "1288331521", sup.PartyIdentifications[0].ID.Value)
	require.NotNil(t, sup.PartyName)
	assert.Equal(t, "AAA Anonim Şirketi", sup.PartyName.Name)
	assert.Equal(t, "İstanbul", sup.PostalAddress.CityName)

	cus := inv.AccountingCustomerParty.Party
	require.Len(t, cus.PartyIdentifications, 3) // TCKN + TESISATNO + SAYACNO
	assert.Equal(t, "TCKN", cus.PartyIdentifications[0].ID.SchemeID)
	require.NotNil(t, cus.Person)
	assert.Equal(t, "Ali", cus.Person.FirstName)

	require.Len(t, inv.InvoiceLines, 1)
	line := inv.InvoiceLines[0]
	assert.Equal(t, "KWH", line.InvoicedQuantity.UnitCode)
	assert.Equal(t, "101", line.InvoicedQuantity.Value.String())
	assert.Equal(t, "15.15", line.LineExtensionAmount.Value.String())
	assert.Equal(t, "TRY", line.LineExtensionAmount.CurrencyID)
	assert.Equal(t, "Elektrik Tüketim Bedeli", line.Item.Name)
	assert.Equal(t, "0.15", line.Price.PriceAmount.Value.String())
	require.NotNil(t, line.TaxTotal)
	require.Len(t, line.TaxTotal.TaxSubtotals, 1)
	require.NotNil(t, line.TaxTotal.TaxSubtotals[0].Percent)
	assert.Equal(t, "18", line.TaxTotal.TaxSubtotals[0].Percent.String())
	assert.Equal(t, "0015", line.TaxTotal.TaxSubtotals[0].TaxCategory.TaxScheme.TaxTypeCode)

	require.Len(t, inv.TaxTotals, 1)
	assert.Equal(t, "2.73", inv.TaxTotals[0].TaxAmount.Value.String())
	assert.Equal(t, "17.88", inv.LegalMonetaryTotal.PayableAmount.Value.String())
}

// Round-trip: parse -> XML -> parse ayni degeri vermeli, cikti deterministik
// olmali ve orijinaldeki dolu elemanlarin hicbiri kaybolmamali. GIB
// orijinaliyle bayt esitligi hedef degil (imza/xsi anotasyonlari korunmuyor);
// "bos eleman == yok" sayilir, cunku omitempty bos opsiyonelleri yazmaz.
var gibSamples = []string{
	"satis-temelfatura.xml",
	"satis-ticarifatura.xml",
	"iade-ticarifatura.xml",
	"tevkifat-ticarifatura.xml",
	"istisna-usd-ticarifatura.xml",
	"istisna2-ticarifatura.xml",
	"ozelmatrah-ticarifatura.xml",
}

func TestInvoiceRoundTrip(t *testing.T) {
	for _, name := range gibSamples {
		t.Run(name, func(t *testing.T) {
			data := readFixture(t, name)
			inv, err := ParseInvoice(data)
			require.NoError(t, err)

			out, err := inv.XML()
			require.NoError(t, err)

			inv2, err := ParseInvoice(out)
			require.NoError(t, err)
			if !reflect.DeepEqual(inv, inv2) {
				t.Fatal("parse -> XML -> parse degisti")
			}

			out2, err := inv2.XML()
			require.NoError(t, err)
			assert.Equal(t, string(out), string(out2), "cikti deterministik degil")

			assert.Equal(t, elementCounts(t, data), elementCounts(t, out), "eleman kaybi/fazlasi var")
		})
	}
}

// elementCounts belge icindeki dolu elemanlari sayar. Imza tarafi
// (ext:UBLExtensions alti, korunmuyor) ve bilinmeyen namespace'ler atlanir;
// attr'siz, cocuksuz, metinsiz bos yaprak elemanlar sayilmaz.
func elementCounts(t *testing.T, data []byte) map[string]int {
	t.Helper()
	type elem struct {
		name                       string
		hasAttr, hasChild, hasText bool
	}
	counts := map[string]int{}
	d := xml.NewTokenDecoder(prefixer{xml.NewDecoder(bytes.NewReader(data))})
	var stack []elem
	skip := 0
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		switch e := tok.(type) {
		case xml.StartElement:
			if skip > 0 || e.Name.Space != "" || e.Name.Local == "ext:UBLExtensions" {
				skip++
				continue
			}
			if len(stack) > 0 {
				stack[len(stack)-1].hasChild = true
			}
			stack = append(stack, elem{name: e.Name.Local, hasAttr: len(e.Attr) > 0})
		case xml.EndElement:
			if skip > 0 {
				skip--
				continue
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if top.hasAttr || top.hasChild || top.hasText {
				counts[top.name]++
			}
		case xml.CharData:
			if skip == 0 && len(stack) > 0 && len(bytes.TrimSpace(e)) > 0 {
				stack[len(stack)-1].hasText = true
			}
		}
	}
	return counts
}

func TestParseTevkifat(t *testing.T) {
	inv, err := ParseInvoice(readFixture(t, "tevkifat-ticarifatura.xml"))
	require.NoError(t, err)

	assert.Equal(t, "TEVKIFAT", inv.InvoiceTypeCode)
	require.Len(t, inv.WithholdingTaxTotals, 1)
	wht := inv.WithholdingTaxTotals[0]
	assert.Equal(t, "3240", wht.TaxAmount.Value.String())
	require.Len(t, wht.TaxSubtotals, 1)
	require.NotNil(t, wht.TaxSubtotals[0].Percent)
	assert.Equal(t, "90", wht.TaxSubtotals[0].Percent.String())
	assert.Equal(t, "606", wht.TaxSubtotals[0].TaxCategory.TaxScheme.TaxTypeCode)

	// ust TaxAmount = kalan KDV (3600-3240), alt subtotal tam KDV: GIB teamulu
	require.Len(t, inv.TaxTotals, 1)
	assert.Equal(t, "360", inv.TaxTotals[0].TaxAmount.Value.String())
	assert.Equal(t, "3600", inv.TaxTotals[0].TaxSubtotals[0].TaxAmount.Value.String())
	assert.Equal(t, "20360", inv.LegalMonetaryTotal.PayableAmount.Value.String())
}

func TestParseIstisnaUSD(t *testing.T) {
	inv, err := ParseInvoice(readFixture(t, "istisna-usd-ticarifatura.xml"))
	require.NoError(t, err)

	assert.Equal(t, "ISTISNA", inv.InvoiceTypeCode)
	assert.Equal(t, "USD", inv.DocumentCurrencyCode)
	assert.Equal(t, "USD", inv.LegalMonetaryTotal.PayableAmount.CurrencyID)
	assert.Equal(t, "301", inv.TaxTotals[0].TaxSubtotals[0].TaxCategory.TaxExemptionReasonCode)

	// navlun + sigorta Shipment altinda tasiniyor (AllowanceCharge degil)
	require.NotEmpty(t, inv.Deliveries)
	ship := inv.Deliveries[0].Shipment
	require.NotNil(t, ship)
	require.NotNil(t, ship.DeclaredForCarriageValueAmount)
	assert.Equal(t, "3900", ship.DeclaredForCarriageValueAmount.Value.String())
	require.NotNil(t, ship.InsuranceValueAmount)
	assert.Equal(t, "150", ship.InsuranceValueAmount.Value.String())
}

// Olcek korunumu: "18.0" -> "18.0", "0" -> "0" (decimal.String trailing
// zero'lari kirpar, Dec kirpmamali).
func TestDecScalePreserved(t *testing.T) {
	inv, err := ParseInvoice(readFixture(t, "satis-temelfatura.xml"))
	require.NoError(t, err)

	out, err := inv.XML()
	require.NoError(t, err)

	assert.Contains(t, string(out), "<cbc:Percent>18.0</cbc:Percent>")
	assert.Contains(t, string(out), "<cbc:MultiplierFactorNumeric>0.0</cbc:MultiplierFactorNumeric>")
	assert.Contains(t, string(out), `<cbc:Amount currencyID="TRY">0</cbc:Amount>`)
	assert.Contains(t, string(out), `<cbc:TaxAmount currencyID="TRY">2.73</cbc:TaxAmount>`)
}
