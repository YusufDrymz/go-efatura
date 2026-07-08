package ubltr

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sentetik, checksum-gecerli kimlikler (999 onekli, gercek mukellefle iliskisiz)
func testSupplier() PartyInfo {
	return PartyInfo{
		VKN:       "9990000005",
		Name:      "Örnek Yazılım A.Ş.",
		TaxOffice: "Beşiktaş",
		Address: Address{
			StreetName:          "Örnek Cad.",
			BuildingNumber:      "1",
			CitySubdivisionName: "Beşiktaş",
			CityName:            "İstanbul",
			Country:             Country{Name: "Türkiye"},
		},
	}
}

func testCustomer() PartyInfo {
	return PartyInfo{
		TCKN:       "99900000074",
		FirstName:  "Ali",
		FamilyName: "Yılmaz",
		Address: Address{
			CitySubdivisionName: "Çankaya",
			CityName:            "Ankara",
			Country:             Country{Name: "Türkiye"},
		},
	}
}

func testBuilder(opts ...InvoiceOption) *InvoiceBuilder {
	base := []InvoiceOption{
		WithProfile(ProfileTemelFatura),
		WithType(TypeSatis),
		WithID("ABC2026000000001"),
		WithUUID("F47AC10B-58CC-4372-A567-0E02B2C3D479"),
		WithIssueDate(time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)),
		WithSupplier(testSupplier()),
		WithCustomer(testCustomer()),
	}
	return NewInvoice(append(base, opts...)...)
}

func TestBuildSatisFatura(t *testing.T) {
	b := testBuilder()
	// temel fatura orneginin degerleri: 101 x 0.15 @ %18 -> 15.15 / 2.73
	b.AddLine(Line{Name: "Elektrik", Qty: D("101"), Unit: "KWH", UnitPrice: D("0.15"), VATRate: D("18")})
	// %10 iskontolu satir: 2 x 150 = 300, iskonto 30, matrah 270, KDV %20 = 54
	b.AddLine(Line{Name: "Danışmanlık", Qty: D("2"), Unit: "C62", UnitPrice: D("150"), VATRate: D("20"), DiscountPercent: D("10")})

	inv, err := b.Build()
	require.NoError(t, err)

	assert.Equal(t, "2.1", inv.UBLVersionID)
	assert.Equal(t, "TR1.2", inv.CustomizationID)
	assert.Equal(t, 2, inv.LineCountNumeric)

	l1, l2 := inv.InvoiceLines[0], inv.InvoiceLines[1]
	assert.Equal(t, "1", l1.ID)
	assert.Equal(t, "15.15", l1.LineExtensionAmount.Value.String())
	assert.Equal(t, "2.73", l1.TaxTotal.TaxAmount.Value.String())
	assert.Equal(t, "270", l2.LineExtensionAmount.Value.String())
	assert.Equal(t, "54", l2.TaxTotal.TaxAmount.Value.String())
	require.Len(t, l2.AllowanceCharges, 1)
	assert.Equal(t, "30", l2.AllowanceCharges[0].Amount.Value.String())
	assert.Equal(t, "300", l2.AllowanceCharges[0].BaseAmount.Value.String())

	// oran basina bir TaxSubtotal
	require.Len(t, inv.TaxTotals, 1)
	require.Len(t, inv.TaxTotals[0].TaxSubtotals, 2)
	assert.Equal(t, "56.73", inv.TaxTotals[0].TaxAmount.Value.String())

	tot := inv.LegalMonetaryTotal
	assert.Equal(t, "285.15", tot.LineExtensionAmount.Value.String())
	assert.Equal(t, "285.15", tot.TaxExclusiveAmount.Value.String())
	assert.Equal(t, "341.88", tot.TaxInclusiveAmount.Value.String())
	assert.Equal(t, "341.88", tot.PayableAmount.Value.String())

	// imza blogu ve taraflar
	require.Len(t, inv.Signatures, 1)
	assert.Equal(t, "VKN_TCKN", inv.Signatures[0].ID.SchemeID)
	assert.Equal(t, "9990000005", inv.Signatures[0].ID.Value)
	assert.Equal(t, "TCKN", inv.AccountingCustomerParty.Party.PartyIdentifications[0].ID.SchemeID)
	require.NotNil(t, inv.AccountingCustomerParty.Party.Person)

	// uretilen belge parse edilebilir ve deterministik olmali
	out, err := inv.XML()
	require.NoError(t, err)
	inv2, err := ParseInvoice(out)
	require.NoError(t, err)
	out2, err := inv2.XML()
	require.NoError(t, err)
	assert.Equal(t, string(out), string(out2))
}

func TestBuildCalc(t *testing.T) {
	cases := []struct {
		name          string
		line          Line
		lea, vat, pay string
	}{
		{
			// ozelmatrah orneginin satiri: 0.5 x 49.87 -> 24.935 -> 24.94
			name: "yarim miktar yuvarlama",
			line: Line{Name: "Tütün", Qty: D("0.5"), Unit: "C62", UnitPrice: D("49.87"), VATRate: D("18")},
			lea:  "24.94", vat: "4.49", pay: "29.43",
		},
		{
			name: "half-up siniri",
			line: Line{Name: "X", Qty: D("1"), Unit: "C62", UnitPrice: D("2.725"), VATRate: D("18")},
			lea:  "2.73", vat: "0.49", pay: "3.22",
		},
		{
			name: "dusuk oran",
			line: Line{Name: "X", Qty: D("3"), Unit: "C62", UnitPrice: D("10"), VATRate: D("1")},
			lea:  "30", vat: "0.3", pay: "30.3",
		},
		{
			name: "tutar iskontosu",
			line: Line{Name: "X", Qty: D("1"), Unit: "C62", UnitPrice: D("100"), VATRate: D("20"), DiscountAmount: D("15.5")},
			lea:  "84.5", vat: "16.9", pay: "101.4",
		},
		{
			name: "kdv sifir istisna",
			line: Line{Name: "X", Qty: D("1"), Unit: "C62", UnitPrice: D("500"), VATRate: D("0"), ExemptionCode: "301", ExemptionReason: "İhracat istisnası"},
			lea:  "500", vat: "0", pay: "500",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := testBuilder()
			b.AddLine(tc.line)
			inv, err := b.Build()
			require.NoError(t, err)
			assert.Equal(t, tc.lea, inv.InvoiceLines[0].LineExtensionAmount.Value.String())
			assert.Equal(t, tc.vat, inv.TaxTotals[0].TaxAmount.Value.String())
			assert.Equal(t, tc.pay, inv.LegalMonetaryTotal.PayableAmount.Value.String())
		})
	}
}

func TestBuildErrors(t *testing.T) {
	okLine := Line{Name: "X", Qty: D("1"), Unit: "C62", UnitPrice: D("10"), VATRate: D("20")}
	cases := []struct {
		name    string
		mutate  func(*InvoiceBuilder)
		wantErr string
	}{
		{"profil yok", func(b *InvoiceBuilder) { b.inv.ProfileID = "" }, "profil zorunlu"},
		{"fatura no bicimi", func(b *InvoiceBuilder) { b.inv.ID = "FTR-2026-1" }, "bicimi gecersiz"},
		{"satir yok", func(b *InvoiceBuilder) { b.lines = nil }, "en az bir satir"},
		{"gecersiz vkn", func(b *InvoiceBuilder) { b.supplier.VKN = "9990000006" }, "gecersiz VKN"},
		{"gecersiz tckn", func(b *InvoiceBuilder) { b.customer.TCKN = "12345678901" }, "gecersiz TCKN"},
		{"adres eksik", func(b *InvoiceBuilder) { b.customer.Address.CityName = "" }, "ilce, il ve ulke"},
		{"doviz kursuz", func(b *InvoiceBuilder) { b.inv.DocumentCurrencyCode = "USD" }, "kur zorunlu"},
		{"kdv sifir gerekcesiz", func(b *InvoiceBuilder) { b.lines[0].VATRate = D("0") }, "ExemptionReason zorunlu"},
		{"cift iskonto", func(b *InvoiceBuilder) {
			b.lines[0].DiscountAmount = D("1")
			b.lines[0].DiscountPercent = D("1")
		}, "birlikte verilemez"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := testBuilder()
			b.AddLine(okLine)
			tc.mutate(b)
			_, err := b.Build()
			require.Error(t, err)
			assert.True(t, strings.Contains(err.Error(), tc.wantErr), "beklenen %q, gelen: %v", tc.wantErr, err)
		})
	}
}

func TestBuildDoviz(t *testing.T) {
	b := testBuilder(WithCurrency("USD"), WithExchangeRate(D("41.35"), "2026-07-08"))
	b.AddLine(Line{Name: "X", Qty: D("1"), Unit: "C62", UnitPrice: D("100"), VATRate: D("20")})
	inv, err := b.Build()
	require.NoError(t, err)

	require.NotNil(t, inv.PricingExchangeRate)
	assert.Equal(t, "USD", inv.PricingExchangeRate.SourceCurrencyCode)
	assert.Equal(t, "TRY", inv.PricingExchangeRate.TargetCurrencyCode)
	assert.Equal(t, "41.35", inv.PricingExchangeRate.CalculationRate.String())
	assert.Equal(t, "USD", inv.LegalMonetaryTotal.PayableAmount.CurrencyID)
	assert.Equal(t, "USD", inv.InvoiceLines[0].LineExtensionAmount.CurrencyID)
}

func TestBuildAutoUUID(t *testing.T) {
	b := testBuilder(WithUUID(""))
	b.AddLine(Line{Name: "X", Qty: D("1"), Unit: "C62", UnitPrice: D("10"), VATRate: D("20")})
	inv, err := b.Build()
	require.NoError(t, err)
	assert.Regexp(t, `^[0-9A-F]{8}-[0-9A-F]{4}-4[0-9A-F]{3}-[89AB][0-9A-F]{3}-[0-9A-F]{12}$`, inv.UUID)
}
