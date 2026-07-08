package ubltr

import (
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

// Round-trip: parse -> XML -> parse ayni degeri vermeli, ikinci tur cikti
// bayt bazinda deterministik olmali. GIB orijinaliyle bayt esitligi hedef
// degil (imza/xsi anotasyonlari korunmuyor), deger esitligi hedef.
func TestInvoiceRoundTrip(t *testing.T) {
	inv, err := ParseInvoice(readFixture(t, "satis-temelfatura.xml"))
	require.NoError(t, err)

	out, err := inv.XML()
	require.NoError(t, err)

	inv2, err := ParseInvoice(out)
	require.NoError(t, err)
	if !reflect.DeepEqual(inv, inv2) {
		t.Fatalf("roundtrip degisti:\nilk:  %+v\nson:  %+v", inv, inv2)
	}

	out2, err := inv2.XML()
	require.NoError(t, err)
	assert.Equal(t, string(out), string(out2))
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
