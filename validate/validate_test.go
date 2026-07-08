package validate

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/YusufDrymz/go-efatura/ubltr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sentetik, checksum-gecerli kimlikler (999 onekli)
func validInvoice(t *testing.T) *ubltr.Invoice {
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
	return inv
}

func hasRule(issues []Issue, rule string) bool {
	for _, i := range issues {
		if i.Rule == rule {
			return true
		}
	}
	return false
}

func TestValidInvoicePasses(t *testing.T) {
	issues := Invoice(validInvoice(t))
	assert.Empty(t, issues, "builder çıktısı kurallardan geçmeli: %v", issues)
}

func TestRuleViolations(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ubltr.Invoice)
		rule   string
	}{
		{"ubl versiyonu", func(i *ubltr.Invoice) { i.UBLVersionID = "2.0" }, "UBLVersionIDCheck"},
		{"customization", func(i *ubltr.Invoice) { i.CustomizationID = "TR9" }, "CustomizationIDCheck"},
		{"profil", func(i *ubltr.Invoice) { i.ProfileID = "FATURA" }, "ProfileIDCheck"},
		{"fatura no yil", func(i *ubltr.Invoice) { i.ID = "ABC1999000000001" }, "InvoiceIDCheck"},
		{"suret", func(i *ubltr.Invoice) { i.CopyIndicator = true }, "CopyIndicatorCheck"},
		{"ettn", func(i *ubltr.Invoice) { i.UUID = "kacak-uuid" }, "UUIDCheck"},
		{"ileri tarih", func(i *ubltr.Invoice) { i.IssueDate = "2099-01-01" }, "TimeCheck"},
		{"eski tarih", func(i *ubltr.Invoice) { i.IssueDate = "2004-12-31" }, "TimeCheck"},
		{"tip listede yok", func(i *ubltr.Invoice) { i.InvoiceTypeCode = "PROFORMA" }, "InvoiceTypeCodeCheck"},
		{"iade ticari profilde", func(i *ubltr.Invoice) { i.InvoiceTypeCode = "IADE"; i.ProfileID = "TICARIFATURA" }, "InvoiceTypeCodeCheck"},
		{"sarj enerji disi", func(i *ubltr.Invoice) { i.InvoiceTypeCode = "SARJ" }, "InvoiceTypeCodeCheck"},
		{"para birimi", func(i *ubltr.Invoice) { i.DocumentCurrencyCode = "TRL" }, "CurrencyCodeCheck"},
		{"doviz kursuz", func(i *ubltr.Invoice) { i.DocumentCurrencyCode = "USD" }, "CurrencyCodeCheck"},
		{"currencyID bozuk", func(i *ubltr.Invoice) { i.LegalMonetaryTotal.PayableAmount.CurrencyID = "XQZ" }, "GeneralCurrencyIDCheck"},
		{"scheme id bozuk", func(i *ubltr.Invoice) {
			i.AccountingSupplierParty.Party.PartyIdentifications[0].ID.SchemeID = "VERGINO"
		}, "PartyIdentificationSchemeIDCheck"},
		{"vkn 9 hane", func(i *ubltr.Invoice) {
			i.AccountingSupplierParty.Party.PartyIdentifications[0].ID.Value = "123456789"
		}, "PartyIdentificationTCKNVKNCheck"},
		{"vkn checksum", func(i *ubltr.Invoice) {
			i.AccountingSupplierParty.Party.PartyIdentifications[0].ID.Value = "9990000006"
		}, "GOEF-VKN-CHECKSUM"},
		{"tckn checksum", func(i *ubltr.Invoice) {
			i.AccountingCustomerParty.Party.PartyIdentifications[0].ID.Value = "99900000075"
		}, "GOEF-TCKN-CHECKSUM"},
		{"unvansiz kurum", func(i *ubltr.Invoice) { i.AccountingSupplierParty.Party.PartyName = nil }, "PartyIdentificationPartyNamePersonCheck"},
		{"persosuz sahis", func(i *ubltr.Invoice) { i.AccountingCustomerParty.Party.Person = nil }, "PartyIdentificationPartyNamePersonCheck"},
		{"imzasiz", func(i *ubltr.Invoice) { i.Signatures = nil }, "SignatureCheck"},
		{"cift imza", func(i *ubltr.Invoice) { i.Signatures = append(i.Signatures, i.Signatures[0]) }, "SignatureCountCheck"},
		{"vergi kodu bozuk", func(i *ubltr.Invoice) {
			i.TaxTotals[0].TaxSubtotals[0].TaxCategory.TaxScheme.TaxTypeCode = "9999"
		}, "TaxTypeCheck"},
		{"tevkifat satis tipinde", func(i *ubltr.Invoice) {
			i.WithholdingTaxTotals = []ubltr.TaxTotal{{TaxAmount: i.TaxTotals[0].TaxAmount}}
		}, "GeneralWithholdingTaxTotalCheck"},
		{"kdv sifir gerekcesiz", func(i *ubltr.Invoice) {
			i.TaxTotals[0].TaxSubtotals[0].TaxAmount.Value = ubltr.D("0")
		}, "TaxExemptionReasonCheck"},
		{"istisna kodu satis tipinde", func(i *ubltr.Invoice) {
			i.TaxTotals[0].TaxSubtotals[0].TaxCategory.TaxExemptionReason = "İhracat"
			i.TaxTotals[0].TaxSubtotals[0].TaxCategory.TaxExemptionReasonCode = "301"
		}, "TaxExemptionReasonCodeCheck"},
		{"iade referanssiz", func(i *ubltr.Invoice) {
			i.ProfileID = "TEMELFATURA"
			i.InvoiceTypeCode = "IADE"
		}, "IADEInvioceCheck"},
		{"kalem sayisi", func(i *ubltr.Invoice) { i.LineCountNumeric = 5 }, "GOEF-COUNT-1"},
		{"unitcode yok", func(i *ubltr.Invoice) { i.InvoiceLines[0].InvoicedQuantity.UnitCode = "" }, "InvoicedQuantityCheck"},
		{"unitcode bozuk", func(i *ubltr.Invoice) { i.InvoiceLines[0].InvoicedQuantity.UnitCode = "ADET" }, "GeneralUnitCodeCheck"},
		{"kalem adi bos", func(i *ubltr.Invoice) { i.InvoiceLines[0].Item.Name = " " }, "ItemNameCheck"},
		{"odeme kodu bozuk", func(i *ubltr.Invoice) {
			i.PaymentMeans = []ubltr.PaymentMeans{{PaymentMeansCode: "999"}}
		}, "PaymentMeansCodeCheck"},
		{"kamu ibansiz", func(i *ubltr.Invoice) { i.ProfileID = "KAMU" }, "KamuFaturaCheck"},
		{"uc haneli kurus", func(i *ubltr.Invoice) { i.LegalMonetaryTotal.PayableAmount.Value = ubltr.D("100.555") }, "decimalCheck"},
		{"toplam tutmuyor", func(i *ubltr.Invoice) { i.LegalMonetaryTotal.LineExtensionAmount.Value = ubltr.D("1") }, "GOEF-TOTALS-1"},
		{"vergili toplam tutmuyor", func(i *ubltr.Invoice) { i.LegalMonetaryTotal.TaxInclusiveAmount.Value = ubltr.D("9999") }, "GOEF-TOTALS-3"},
		{"odenecek tutmuyor", func(i *ubltr.Invoice) { i.LegalMonetaryTotal.PayableAmount.Value = ubltr.D("1") }, "GOEF-TOTALS-4"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inv := validInvoice(t)
			tc.mutate(inv)
			issues := Invoice(inv)
			assert.True(t, hasRule(issues, tc.rule), "beklenen kural %s tetiklenmedi; bulgular: %v", tc.rule, issues)
		})
	}
}

// Resmi GIB ornekleri uzerinde bilinen bulgular: orneklerin bir kismi
// guncel schematron'u gecemiyor — dogrulayicinin bunlari yakalamasi beklenir.
func TestKnownFindingsInGIBSamples(t *testing.T) {
	cases := []struct {
		fixture string
		rule    string
		reason  string
	}{
		{"tevkifat-ticarifatura.xml", "InvoicedQuantityCheck", "resmi ornekte unitCode yok"},
		{"istisna-usd-ticarifatura.xml", "CurrencyCodeCheck", "USD ornekte kur bilgisi yok"},
		{"iade-ticarifatura.xml", "InvoiceTypeCodeCheck", "2009 ornegi IADE+TICARIFATURA; guncel kural yasaklar"},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			data, err := os.ReadFile("../ubltr/testdata/gib/" + tc.fixture)
			require.NoError(t, err)
			inv, err := ubltr.ParseInvoice(data)
			require.NoError(t, err)
			issues := Invoice(inv)
			assert.True(t, hasRule(issues, tc.rule), "%s: %s bekleniyordu (%s); bulgular: %v", tc.fixture, tc.rule, tc.reason, issues)
		})
	}
}

func TestWithholdingPercentPair(t *testing.T) {
	inv := validInvoice(t)
	inv.InvoiceTypeCode = "TEVKIFAT"
	pct := ubltr.D("90")
	inv.WithholdingTaxTotals = []ubltr.TaxTotal{{
		TaxAmount: ubltr.Amount{Value: ubltr.D("3240"), CurrencyID: "TRY"},
		TaxSubtotals: []ubltr.TaxSubtotal{{
			TaxAmount:   ubltr.Amount{Value: ubltr.D("3240"), CurrencyID: "TRY"},
			Percent:     &pct,
			TaxCategory: ubltr.TaxCategory{TaxScheme: ubltr.TaxScheme{TaxTypeCode: "606"}},
		}},
	}}
	// 606 + %90 = "60690" listede: kod-yuzde uyumsuzlugu URETMEMELI
	issues := Invoice(inv)
	for _, i := range issues {
		assert.NotEqual(t, "WithholdingTaxTotalCheck", i.Rule, "60690 gecerli cift: %v", i)
	}

	// %50, 606 icin gecersiz
	bad := ubltr.D("50")
	inv.WithholdingTaxTotals[0].TaxSubtotals[0].Percent = &bad
	assert.True(t, hasRule(Invoice(inv), "WithholdingTaxTotalCheck"))
}

func TestErrorsFilter(t *testing.T) {
	inv := validInvoice(t)
	// GOEF-LINE-1 uyari uretir: satir tutari hesapla uyusmasin ama belge
	// toplamlariyla tutarli kalsin diye tum toplamlari da oynatmiyoruz —
	// sadece satirin birim fiyatini bozuyoruz.
	inv.InvoiceLines[0].Price.PriceAmount.Value = ubltr.D("9")
	issues := Invoice(inv)
	require.True(t, hasRule(issues, "GOEF-LINE-1"))
	for _, i := range issues {
		if i.Rule == "GOEF-LINE-1" {
			assert.Equal(t, Warning, i.Severity)
		}
	}
	assert.Less(t, len(Errors(issues)), len(issues), "uyarilar Errors() disinda kalmali")
}

func TestIssueString(t *testing.T) {
	s := Issue{Rule: "TimeCheck", Severity: Error, Path: "IssueDate", Message: "x"}.String()
	assert.True(t, strings.Contains(s, "TimeCheck") && strings.Contains(s, "IssueDate"))
}
