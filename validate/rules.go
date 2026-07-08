package validate

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/YusufDrymz/go-efatura/ubltr"
	trvalidate "github.com/YusufDrymz/go-trvalidate"
	"github.com/shopspring/decimal"
)

// Kaynak kisaltmalari: Common = UBL-TR_Common_Schematron.xml,
// Main = UBL-TR_Main_Schematron.xml (satir numaralari o dosyalardan).

var (
	invoiceIDRe = regexp.MustCompile(`^[A-Z0-9]{3}20[0-9]{11}$`) // InvoiceIDCheck, Common:154
	uuidRe      = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)
	trIBANRe    = regexp.MustCompile(`^TR\d{7}[A-Z0-9]{17}$`) // KamuFaturaCheck, Common:520
)

// UBLVersionIDCheck Common:135, CustomizationIDCheck Common:141,
// ProfileIDCheck Common:146, InvoiceIDCheck Common:154,
// CopyIndicatorCheck Common:163, UUIDCheck Common:225
func checkHeader(inv *ubltr.Invoice, r *report) {
	if inv.UBLVersionID != "2.1" {
		r.errf("UBLVersionIDCheck", "UBLVersionID", "değer '2.1' olmalıdır, '%s' bulundu", inv.UBLVersionID)
	}
	if inv.CustomizationID != "TR1.2" && inv.CustomizationID != "TR1.2.1" {
		r.errf("CustomizationIDCheck", "CustomizationID", "değer 'TR1.2' veya 'TR1.2.1' olmalıdır, '%s' bulundu", inv.CustomizationID)
	}
	if !profileIDSet[inv.ProfileID] {
		r.errf("ProfileIDCheck", "ProfileID", "geçersiz profil '%s'", inv.ProfileID)
	}
	if !invoiceIDRe.MatchString(inv.ID) {
		r.errf("InvoiceIDCheck", "ID", "fatura no 'ABC2009123456789' formatında olmalıdır, '%s' bulundu", inv.ID)
	}
	if inv.CopyIndicator {
		r.errf("CopyIndicatorCheck", "CopyIndicator", "'false' olmalıdır (suret gönderilemez)")
	}
	if !uuidRe.MatchString(inv.UUID) {
		r.errf("UUIDCheck", "UUID", "ETTN UUID formatında olmalıdır, '%s' bulundu", inv.UUID)
	}
}

// TimeCheck Common:168 — bugünden ileri olamaz, 2005-01-01'den eski olamaz.
func checkIssueDate(inv *ubltr.Invoice, r *report) {
	d, err := time.Parse("2006-01-02", inv.IssueDate)
	if err != nil {
		r.errf("TimeCheck", "IssueDate", "geçersiz tarih '%s' (YYYY-AA-GG bekleniyor)", inv.IssueDate)
		return
	}
	if today := time.Now(); d.After(today) {
		r.errf("TimeCheck", "IssueDate", "fatura tarihi (%s) günün tarihinden ileri olamaz", inv.IssueDate)
	}
	if d.Before(time.Date(2005, 1, 1, 0, 0, 0, 0, time.UTC)) {
		r.errf("TimeCheck", "IssueDate", "fatura tarihi 01.01.2005'ten önce olamaz")
	}
}

// InvoiceTypeCodeCheck Common:174 — tip listede; IADE yalnız belirli
// profillerde; ENERJI ile SARJ/SARJANLIK karşılıklı bağlı;
// TEKNOLOJIDESTEK yalnız EARSIVFATURA.
func checkTypeAndProfile(inv *ubltr.Invoice, r *report) {
	tc, p := inv.InvoiceTypeCode, inv.ProfileID
	if !invoiceTypeCodeSet[tc] {
		r.errf("InvoiceTypeCodeCheck", "InvoiceTypeCode", "geçersiz fatura tipi '%s'", tc)
	}
	if tc == "IADE" && p != "TEMELFATURA" && p != "EARSIVFATURA" && p != "ILAC_TIBBICIHAZ" && p != "YATIRIMTESVIK" && p != "IDIS" {
		r.errf("InvoiceTypeCodeCheck", "InvoiceTypeCode", "IADE tipi yalnız TEMELFATURA, EARSIVFATURA, ILAC_TIBBICIHAZ, YATIRIMTESVIK veya IDIS profilinde kullanılabilir; profil '%s'", p)
	}
	sarj := tc == "SARJ" || tc == "SARJANLIK"
	if (p == "ENERJI") != sarj {
		r.errf("InvoiceTypeCodeCheck", "InvoiceTypeCode", "ENERJI profili ile SARJ/SARJANLIK tipi birlikte kullanılmalıdır (profil '%s', tip '%s')", p, tc)
	}
	if tc == "TEKNOLOJIDESTEK" && p != "EARSIVFATURA" {
		r.errf("InvoiceTypeCodeCheck", "InvoiceTypeCode", "TEKNOLOJIDESTEK tipinde profil EARSIVFATURA olmalıdır")
	}
}

// CurrencyCodeCheck Common:183 — para birimi kodları listede; TRY dışı
// belgede kur zorunlu ve en fazla 6 ondalık haneli.
// GeneralCurrencyIDCheck Common:204 — tüm currencyID nitelikleri listede.
func checkCurrencies(inv *ubltr.Invoice, r *report) {
	codes := []struct{ path, v string }{
		{"DocumentCurrencyCode", inv.DocumentCurrencyCode},
		{"TaxCurrencyCode", inv.TaxCurrencyCode},
		{"PricingCurrencyCode", inv.PricingCurrencyCode},
		{"PaymentCurrencyCode", inv.PaymentCurrencyCode},
		{"PaymentAlternativeCurrencyCode", inv.PaymentAlternativeCurrencyCode},
	}
	for _, c := range codes {
		if c.v != "" && !currencyCodeSet[c.v] {
			r.errf("CurrencyCodeCheck", c.path, "geçersiz para birimi '%s'", c.v)
		}
	}
	if inv.DocumentCurrencyCode == "" {
		r.errf("CurrencyCodeCheck", "DocumentCurrencyCode", "belge para birimi zorunludur")
	}
	if inv.DocumentCurrencyCode != "TRY" && inv.DocumentCurrencyCode != "" {
		if inv.PricingExchangeRate == nil {
			r.errf("CurrencyCodeCheck", "PricingExchangeRate", "para birimi TRY olmayan belgelerde kur bilgisi zorunludur")
		} else if rate := inv.PricingExchangeRate.CalculationRate; rate.IsNegative() || rate.Exponent() < -6 {
			r.errf("CurrencyCodeCheck", "PricingExchangeRate/CalculationRate", "kur noktadan sonra en fazla 6 haneli ve negatif olmayan bir değer olmalıdır")
		}
	}
	for _, a := range amounts(inv) {
		if !currencyCodeSet[a.amt.CurrencyID] {
			r.errf("GeneralCurrencyIDCheck", a.path, "geçersiz currencyID '%s'", a.amt.CurrencyID)
		}
	}
}

type pathedAmount struct {
	path string
	amt  *ubltr.Amount
}

// amounts belgede dolasan tutarlari yol bilgisiyle toplar (currencyID ve
// hane denetimleri icin).
func amounts(inv *ubltr.Invoice) []pathedAmount {
	var out []pathedAmount
	add := func(path string, a *ubltr.Amount) {
		if a != nil {
			out = append(out, pathedAmount{path, a})
		}
	}
	t := &inv.LegalMonetaryTotal
	add("LegalMonetaryTotal/LineExtensionAmount", &t.LineExtensionAmount)
	add("LegalMonetaryTotal/TaxExclusiveAmount", &t.TaxExclusiveAmount)
	add("LegalMonetaryTotal/TaxInclusiveAmount", &t.TaxInclusiveAmount)
	add("LegalMonetaryTotal/AllowanceTotalAmount", t.AllowanceTotalAmount)
	add("LegalMonetaryTotal/ChargeTotalAmount", t.ChargeTotalAmount)
	add("LegalMonetaryTotal/PayableRoundingAmount", t.PayableRoundingAmount)
	add("LegalMonetaryTotal/PayableAmount", &t.PayableAmount)
	for i := range inv.TaxTotals {
		tt := &inv.TaxTotals[i]
		add(fmt.Sprintf("TaxTotal[%d]/TaxAmount", i+1), &tt.TaxAmount)
		for j := range tt.TaxSubtotals {
			st := &tt.TaxSubtotals[j]
			p := fmt.Sprintf("TaxTotal[%d]/TaxSubtotal[%d]", i+1, j+1)
			add(p+"/TaxableAmount", st.TaxableAmount)
			add(p+"/TaxAmount", &st.TaxAmount)
		}
	}
	for i := range inv.WithholdingTaxTotals {
		add(fmt.Sprintf("WithholdingTaxTotal[%d]/TaxAmount", i+1), &inv.WithholdingTaxTotals[i].TaxAmount)
	}
	for i := range inv.InvoiceLines {
		l := &inv.InvoiceLines[i]
		p := fmt.Sprintf("InvoiceLine[%d]", i+1)
		add(p+"/LineExtensionAmount", &l.LineExtensionAmount)
		add(p+"/Price/PriceAmount", &l.Price.PriceAmount)
		for j := range l.AllowanceCharges {
			add(fmt.Sprintf("%s/AllowanceCharge[%d]/Amount", p, j+1), &l.AllowanceCharges[j].Amount)
		}
		if l.TaxTotal != nil {
			add(p+"/TaxTotal/TaxAmount", &l.TaxTotal.TaxAmount)
		}
	}
	return out
}

// PartyIdentificationPartyNamePersonCheck Common:279,
// PartyIdentificationSchemeIDCheck Common:250,
// PartyIdentificationTCKNVKNCheck Common:255.
// GOEF-VKN-CHECKSUM / GOEF-TCKN-CHECKSUM go-efatura ek kuralı: schematron
// yalnız hane sayısına bakar, checksum'ı kimse denetlemez.
func checkParties(inv *ubltr.Invoice, r *report) {
	checkParty(&inv.AccountingSupplierParty.Party, "AccountingSupplierParty/Party", r)
	checkParty(&inv.AccountingCustomerParty.Party, "AccountingCustomerParty/Party", r)
}

func checkParty(p *ubltr.Party, path string, r *report) {
	var vkn, tckn int
	for i, pi := range p.PartyIdentifications {
		id := pi.ID
		ipath := fmt.Sprintf("%s/PartyIdentification[%d]", path, i+1)
		if !partySchemeIDSet[id.SchemeID] {
			r.errf("PartyIdentificationSchemeIDCheck", ipath, "geçersiz schemeID '%s'", id.SchemeID)
		}
		switch id.SchemeID {
		case "VKN":
			vkn++
			if len(id.Value) != 10 {
				r.errf("PartyIdentificationTCKNVKNCheck", ipath, "VKN 10 haneli olmalıdır, '%s' bulundu", id.Value)
			} else if !trvalidate.IsVKN(id.Value) {
				r.errf("GOEF-VKN-CHECKSUM", ipath, "VKN '%s' checksum doğrulamasından geçmiyor", id.Value)
			}
		case "TCKN":
			tckn++
			if len(id.Value) != 11 {
				r.errf("PartyIdentificationTCKNVKNCheck", ipath, "TCKN 11 haneli olmalıdır, '%s' bulundu", id.Value)
			} else if !trvalidate.IsTCKN(id.Value) {
				r.errf("GOEF-TCKN-CHECKSUM", ipath, "TCKN '%s' checksum doğrulamasından geçmiyor", id.Value)
			}
		}
	}
	switch {
	case vkn == 0 && tckn == 0:
		r.errf("PartyIdentificationPartyNamePersonCheck", path, "schemeID değeri VKN veya TCKN olan bir kimlik bulunmalıdır")
	case vkn > 0 && tckn > 0:
		r.errf("PartyIdentificationPartyNamePersonCheck", path, "hem VKN hem TCKN bulunamaz")
	case vkn > 0:
		if p.PartyName == nil || strings.TrimSpace(p.PartyName.Name) == "" {
			r.errf("PartyIdentificationPartyNamePersonCheck", path, "VKN'li tarafta boş olmayan PartyName/Name zorunludur")
		}
	case tckn > 0:
		if p.Person == nil || strings.TrimSpace(p.Person.FirstName) == "" || strings.TrimSpace(p.Person.FamilyName) == "" {
			r.errf("PartyIdentificationPartyNamePersonCheck", path, "TCKN'li tarafta boş olmayan FirstName/FamilyName ile Person zorunludur")
		}
	}
}

// SignatureCountCheck Common:233 (en fazla 1), SignatureCheck Common:242,
// SignatoryPartyPartyIdentificationCheck Common:577. Kılavuz cac:Signature'ı
// zorunlu (1..n) sayar; schematron üst sınırı 1'e çeker → tam 1 beklenir.
func checkSignatures(inv *ubltr.Invoice, r *report) {
	if len(inv.Signatures) == 0 {
		r.errf("SignatureCheck", "Signature", "cac:Signature zorunludur (UBL-TR Fatura kılavuzu)")
		return
	}
	if len(inv.Signatures) > 1 {
		r.errf("SignatureCountCheck", "Signature", "en fazla bir cac:Signature bulunmalıdır, %d bulundu", len(inv.Signatures))
	}
	for i := range inv.Signatures {
		s := &inv.Signatures[i]
		path := fmt.Sprintf("Signature[%d]", i+1)
		if s.ID.SchemeID != "VKN_TCKN" {
			r.errf("SignatureCheck", path, "cbc:ID schemeID değeri 'VKN_TCKN' olmalıdır")
		} else if n := len(s.ID.Value); n != 10 && n != 11 {
			r.errf("SignatureCheck", path, "VKN_TCKN değeri 10 veya 11 haneli olmalıdır")
		}
		ok := false
		for _, pi := range s.SignatoryParty.PartyIdentifications {
			if pi.ID.SchemeID == "VKN" || pi.ID.SchemeID == "TCKN" {
				ok = true
			}
		}
		if !ok {
			r.errf("SignatoryPartyPartyIdentificationCheck", path+"/SignatoryParty", "schemeID değeri VKN veya TCKN olan bir PartyIdentification zorunludur")
		}
	}
}

// TaxTypeCheck Common:386 — belge ve satır seviyesindeki vergi tip kodları
// kod listesinde olmalı. CountryCodeCheck Common:194 de burada (adresler).
func checkTaxes(inv *ubltr.Invoice, r *report) {
	for i := range inv.TaxTotals {
		for j := range inv.TaxTotals[i].TaxSubtotals {
			code := inv.TaxTotals[i].TaxSubtotals[j].TaxCategory.TaxScheme.TaxTypeCode
			if code != "" && !taxTypeCodeSet[code] {
				r.errf("TaxTypeCheck", fmt.Sprintf("TaxTotal[%d]/TaxSubtotal[%d]", i+1, j+1), "geçersiz vergi tipi kodu '%s'", code)
			}
		}
	}
	for i := range inv.InvoiceLines {
		l := &inv.InvoiceLines[i]
		if l.TaxTotal == nil {
			continue
		}
		for j := range l.TaxTotal.TaxSubtotals {
			code := l.TaxTotal.TaxSubtotals[j].TaxCategory.TaxScheme.TaxTypeCode
			if code != "" && !taxTypeCodeSet[code] {
				r.errf("TaxTypeCheck", fmt.Sprintf("InvoiceLine[%d]/TaxTotal/TaxSubtotal[%d]", i+1, j+1), "geçersiz vergi tipi kodu '%s'", code)
			}
		}
	}
	for _, pc := range []struct {
		path string
		code string
	}{
		{"AccountingSupplierParty/Party/PostalAddress/Country", inv.AccountingSupplierParty.Party.PostalAddress.Country.IdentificationCode},
		{"AccountingCustomerParty/Party/PostalAddress/Country", inv.AccountingCustomerParty.Party.PostalAddress.Country.IdentificationCode},
	} {
		if pc.code != "" && !countryCodeSet[pc.code] {
			r.errf("CountryCodeCheck", pc.path, "geçersiz ülke kodu '%s'", pc.code)
		}
	}
}

// GeneralWithholdingTaxTotalCheck Common:288, WithholdingTaxTotalCheck
// Common:302 — tevkifat tip kısıtı ve kod+yüzde uyumu.
func checkWithholding(inv *ubltr.Invoice, r *report) {
	tc := inv.InvoiceTypeCode
	whtAllowed := map[string]bool{"TEVKIFAT": true, "YTBTEVKIFAT": true, "IADE": true, "YTBIADE": true, "SGK": true, "SARJ": true, "SARJANLIK": true}
	if len(inv.WithholdingTaxTotals) > 0 && !whtAllowed[tc] {
		r.errf("GeneralWithholdingTaxTotalCheck", "WithholdingTaxTotal", "cac:WithholdingTaxTotal varken fatura tipi TEVKIFAT, YTBTEVKIFAT, IADE, YTBIADE, SGK, SARJ veya SARJANLIK olabilir; '%s' bulundu", tc)
	}
	for i := range inv.TaxTotals {
		for j := range inv.TaxTotals[i].TaxSubtotals {
			if inv.TaxTotals[i].TaxSubtotals[j].TaxCategory.TaxScheme.TaxTypeCode == "4171" &&
				tc != "TEVKIFAT" && tc != "IADE" && tc != "SGK" && tc != "YTBIADE" {
				r.errf("GeneralWithholdingTaxTotalCheck", fmt.Sprintf("TaxTotal[%d]/TaxSubtotal[%d]", i+1, j+1), "vergi tipi 4171 için fatura tipi TEVKIFAT, IADE, YTBIADE veya SGK olabilir; '%s' bulundu", tc)
			}
		}
	}
	check := func(path string, subs []ubltr.TaxSubtotal) {
		for j := range subs {
			st := &subs[j]
			p := fmt.Sprintf("%s/TaxSubtotal[%d]", path, j+1)
			code := strings.TrimSpace(st.TaxCategory.TaxScheme.TaxTypeCode)
			if code == "" || st.Percent == nil {
				r.errf("WithholdingTaxTotalCheck", p, "tevkifat satırında TaxTypeCode ve Percent dolu olmalıdır")
				continue
			}
			if !withholdingTaxTypeSet[code] {
				r.errf("WithholdingTaxTotalCheck", p, "geçersiz tevkifat kodu '%s'", code)
				continue
			}
			if !withholdingWithPctSet[code+st.Percent.String()] {
				r.errf("WithholdingTaxTotalCheck", p, "'%s' tevkifat kodunun yüzdesi '%s' olamaz", code, st.Percent.String())
			}
		}
	}
	for i := range inv.WithholdingTaxTotals {
		check(fmt.Sprintf("WithholdingTaxTotal[%d]", i+1), inv.WithholdingTaxTotals[i].TaxSubtotals)
	}
	for i := range inv.InvoiceLines {
		for k := range inv.InvoiceLines[i].WithholdingTaxTotals {
			check(fmt.Sprintf("InvoiceLine[%d]/WithholdingTaxTotal[%d]", i+1, k+1), inv.InvoiceLines[i].WithholdingTaxTotals[k].TaxSubtotals)
		}
	}
}

// TaxExemptionReasonCheck Common:390 — KDV (0015) tutarı 0 ise gerekçe
// zorunlu (belirli tipler hariç). TaxExemptionReasonCodeCheck Common:311 —
// gerekçe varsa kod dolu/listede; kod gruplarına göre tip kısıtı.
// Schematron bunları yalnız belge seviyesi TaxSubtotal'a uygular (Main:211,
// satır seviyesi Main'de yorumda) — aynısı yapılır.
func checkExemptions(inv *ubltr.Invoice, r *report) {
	tc := inv.InvoiceTypeCode
	reasonExempt := map[string]bool{"IADE": true, "YTBIADE": true, "IHRACKAYITLI": true, "OZELMATRAH": true, "SGK": true, "KONAKLAMAVERGISI": true}
	for i := range inv.TaxTotals {
		for j := range inv.TaxTotals[i].TaxSubtotals {
			st := &inv.TaxTotals[i].TaxSubtotals[j]
			p := fmt.Sprintf("TaxTotal[%d]/TaxSubtotal[%d]", i+1, j+1)
			cat := &st.TaxCategory
			if !reasonExempt[tc] && cat.TaxScheme.TaxTypeCode == "0015" && st.TaxAmount.Value.IsZero() &&
				strings.TrimSpace(cat.TaxExemptionReason) == "" {
				r.errf("TaxExemptionReasonCheck", p, "vergi tutarı 0 olan 0015 kodlu KDV için TaxExemptionReason zorunludur")
			}
			if strings.TrimSpace(cat.TaxExemptionReason) != "" {
				code := strings.TrimSpace(cat.TaxExemptionReasonCode)
				if code == "" || !exemptionCodeSet[code] {
					r.errf("TaxExemptionReasonCodeCheck", p, "TaxExemptionReason varken TaxExemptionReasonCode dolu ve geçerli olmalıdır ('%s' bulundu)", code)
				}
			}
			if code := strings.TrimSpace(cat.TaxExemptionReasonCode); code != "" {
				switch {
				case code != "555" && istisnaExemptionSet[code] &&
					tc != "ISTISNA" && tc != "IADE" && tc != "IHRACKAYITLI" && tc != "SGK" && tc != "YTBISTISNA" && tc != "YTBIADE":
					r.errf("TaxExemptionReasonCodeCheck", p, "istisna kodu '%s' için fatura tipi ISTISNA, IADE, IHRACKAYITLI, SGK, YTBISTISNA veya YTBIADE olabilir; '%s' bulundu", code, tc)
				case ozelMatrahExemptionSet[code] && tc != "OZELMATRAH" && tc != "IADE" && tc != "SGK":
					r.errf("TaxExemptionReasonCodeCheck", p, "özel matrah kodu '%s' için fatura tipi OZELMATRAH, IADE veya SGK olabilir; '%s' bulundu", code, tc)
				case ihracExemptionSet[code] && tc != "IHRACKAYITLI" && tc != "IADE" && tc != "SGK":
					r.errf("TaxExemptionReasonCodeCheck", p, "ihraç kayıtlı kodu '%s' için fatura tipi IHRACKAYITLI, IADE veya SGK olabilir; '%s' bulundu", code, tc)
				}
			}
		}
	}
}

// IADEInvioceCheck Common:357 — iade ailesinde, iade edilen faturaya
// DocumentTypeCode'u IADE ve 16 haneli ID'li InvoiceDocumentReference şart.
func checkIadeReferences(inv *ubltr.Invoice, r *report) {
	tc := inv.InvoiceTypeCode
	if tc != "IADE" && tc != "TEVKIFATIADE" && tc != "YTBIADE" && tc != "YTBTEVKIFATIADE" {
		return
	}
	var refs []*ubltr.DocumentReference
	for i := range inv.BillingReferences {
		if ref := inv.BillingReferences[i].InvoiceDocumentReference; ref != nil {
			refs = append(refs, ref)
		}
	}
	if len(refs) == 0 {
		r.errf("IADEInvioceCheck", "BillingReference", "%s tipinde iade edilen faturayı gösteren BillingReference/InvoiceDocumentReference zorunludur", tc)
		return
	}
	for i, ref := range refs {
		if (ref.DocumentTypeCode != "IADE" && ref.DocumentTypeCode != "İADE") || len(strings.TrimSpace(ref.ID)) != 16 {
			r.errf("IADEInvioceCheck", fmt.Sprintf("BillingReference[%d]/InvoiceDocumentReference", i+1), "DocumentTypeCode 'IADE' ve 16 haneli fatura no zorunludur")
		}
	}
}

// InvoicedQuantityCheck Common:399, ItemNameCheck Common:713,
// GOEF-COUNT-1 (kılavuz: LineCountNumeric kalem sayısıdır).
func checkLines(inv *ubltr.Invoice, r *report) {
	if inv.LineCountNumeric != len(inv.InvoiceLines) {
		r.errf("GOEF-COUNT-1", "LineCountNumeric", "LineCountNumeric (%d) kalem sayısına (%d) eşit olmalıdır", inv.LineCountNumeric, len(inv.InvoiceLines))
	}
	for i := range inv.InvoiceLines {
		l := &inv.InvoiceLines[i]
		p := fmt.Sprintf("InvoiceLine[%d]", i+1)
		if strings.TrimSpace(l.InvoicedQuantity.UnitCode) == "" {
			r.errf("InvoicedQuantityCheck", p+"/InvoicedQuantity", "unitCode niteliği zorunludur")
		} else if !unitCodeSet[l.InvoicedQuantity.UnitCode] {
			r.errf("GeneralUnitCodeCheck", p+"/InvoicedQuantity", "geçersiz unitCode '%s'", l.InvoicedQuantity.UnitCode)
		}
		if strings.TrimSpace(l.Item.Name) == "" {
			r.errf("ItemNameCheck", p+"/Item/Name", "mal/hizmet adı boş olamaz")
		}
	}
}

// PaymentMeansCodeCheck Common:395, KamuFaturaCheck Common:519.
func checkPaymentMeans(inv *ubltr.Invoice, r *report) {
	for i := range inv.PaymentMeans {
		if code := inv.PaymentMeans[i].PaymentMeansCode; code != "" && !paymentMeansCodeSet[code] {
			r.errf("PaymentMeansCodeCheck", fmt.Sprintf("PaymentMeans[%d]", i+1), "geçersiz ödeme şekli kodu '%s'", code)
		}
	}
	if inv.ProfileID != "KAMU" {
		return
	}
	for i := range inv.PaymentMeans {
		if acc := inv.PaymentMeans[i].PayeeFinancialAccount; acc != nil && trIBANRe.MatchString(acc.ID) {
			return
		}
	}
	r.errf("KamuFaturaCheck", "PaymentMeans/PayeeFinancialAccount", "KAMU profilinde geçerli bir Türkiye IBAN'ı zorunludur")
}

// decimalCheck Common:229 — Main:260-287'de bağlanan alanlar: belge
// toplamları ve belge seviyesi TaxTotal/TaxAmount. Kural serileştirilmiş
// değere bakar; modelde eşdeğeri: negatif olamaz, en fazla 2 ondalık hane,
// noktadan önce en fazla 15 hane (hata mesajındaki sınır; regex fiilen 17'ye
// izin verir — mesaj esas alındı).
func checkDecimals(inv *ubltr.Invoice, r *report) {
	max15 := decimal.New(1, 15) // 10^15
	check := func(path string, d ubltr.Dec) {
		switch {
		case d.IsNegative():
			r.errf("decimalCheck", path, "tutar negatif olamaz")
		case d.Exponent() < -2:
			r.errf("decimalCheck", path, "tutar noktadan sonra en fazla 2 haneli olmalıdır ('%s')", d.String())
		case d.Abs().Cmp(max15) >= 0:
			r.errf("decimalCheck", path, "tutar noktadan önce en fazla 15 haneli olmalıdır")
		}
	}
	t := &inv.LegalMonetaryTotal
	check("LegalMonetaryTotal/LineExtensionAmount", t.LineExtensionAmount.Value)
	check("LegalMonetaryTotal/TaxExclusiveAmount", t.TaxExclusiveAmount.Value)
	check("LegalMonetaryTotal/TaxInclusiveAmount", t.TaxInclusiveAmount.Value)
	if t.AllowanceTotalAmount != nil {
		check("LegalMonetaryTotal/AllowanceTotalAmount", t.AllowanceTotalAmount.Value)
	}
	check("LegalMonetaryTotal/PayableAmount", t.PayableAmount.Value)
	for i := range inv.TaxTotals {
		check(fmt.Sprintf("TaxTotal[%d]/TaxAmount", i+1), inv.TaxTotals[i].TaxAmount.Value)
	}
}

// GOEF-TOTALS-* go-efatura ek kuralları: kılavuzdaki toplam formülleri
// (UBL-TR Ortak Elemanlar s.47). GIB schematron'u aritmetiği hiç
// denetlemez. OZELMATRAH genel formülden sapar (vergi toplama eklenmez,
// resmi örnek kanıtı) → o tipte 3/4 atlanır. GOEF-LINE-1 yeniden hesap
// içerdiğinden (yuvarlama yöntemi resmî olarak tanımsız) uyarı seviyesinde
// ve ±0.01 toleranslıdır.
func checkTotals(inv *ubltr.Invoice, r *report) {
	sumLEA := decimal.Zero
	for i := range inv.InvoiceLines {
		l := &inv.InvoiceLines[i]
		sumLEA = sumLEA.Add(l.LineExtensionAmount.Value.Decimal)

		raw := l.InvoicedQuantity.Value.Mul(l.Price.PriceAmount.Value.Decimal)
		for j := range l.AllowanceCharges {
			ac := &l.AllowanceCharges[j]
			if ac.ChargeIndicator {
				raw = raw.Add(ac.Amount.Value.Decimal)
			} else {
				raw = raw.Sub(ac.Amount.Value.Decimal)
			}
		}
		if raw.Sub(l.LineExtensionAmount.Value.Decimal).Abs().GreaterThan(decimal.New(1, -2)) {
			r.warnf("GOEF-LINE-1", fmt.Sprintf("InvoiceLine[%d]/LineExtensionAmount", i+1),
				"satır tutarı (%s) miktar × birim fiyat ∓ iskonto/artırım (%s) ile uyuşmuyor",
				l.LineExtensionAmount.Value.String(), raw.String())
		}
	}
	t := &inv.LegalMonetaryTotal
	if !t.LineExtensionAmount.Value.Equal(sumLEA) {
		r.errf("GOEF-TOTALS-1", "LegalMonetaryTotal/LineExtensionAmount", "belge satır toplamı (%s) satırların toplamına (%s) eşit olmalıdır", t.LineExtensionAmount.Value.String(), sumLEA.String())
	}
	expectedExcl := t.LineExtensionAmount.Value.Decimal
	if t.AllowanceTotalAmount != nil {
		expectedExcl = expectedExcl.Sub(t.AllowanceTotalAmount.Value.Decimal)
	}
	if t.ChargeTotalAmount != nil {
		expectedExcl = expectedExcl.Add(t.ChargeTotalAmount.Value.Decimal)
	}
	// Ihracat örneğinde navlun/sigorta Shipment üzerinden matraha girer
	// (resmi örnek, AllowanceCharge kullanmaz) — o durumda kural atlanır.
	shipmentExtras := false
	for i := range inv.Deliveries {
		if inv.Deliveries[i].Shipment != nil {
			s := inv.Deliveries[i].Shipment
			if s.InsuranceValueAmount != nil || s.DeclaredForCarriageValueAmount != nil {
				shipmentExtras = true
			}
		}
	}
	if !shipmentExtras && !t.TaxExclusiveAmount.Value.Equal(expectedExcl) {
		r.errf("GOEF-TOTALS-2", "LegalMonetaryTotal/TaxExclusiveAmount", "vergi matrahı (%s) satır toplamı − iskonto + artırım (%s) olmalıdır", t.TaxExclusiveAmount.Value.String(), expectedExcl.String())
	}
	if inv.InvoiceTypeCode == "OZELMATRAH" || shipmentExtras {
		return
	}
	sumTax := decimal.Zero
	for i := range inv.TaxTotals {
		for j := range inv.TaxTotals[i].TaxSubtotals {
			sumTax = sumTax.Add(inv.TaxTotals[i].TaxSubtotals[j].TaxAmount.Value.Decimal)
		}
	}
	if !t.TaxInclusiveAmount.Value.Equal(t.TaxExclusiveAmount.Value.Add(sumTax)) {
		r.errf("GOEF-TOTALS-3", "LegalMonetaryTotal/TaxInclusiveAmount", "vergiler dahil toplam (%s) matrah + vergi (%s) olmalıdır", t.TaxInclusiveAmount.Value.String(), t.TaxExclusiveAmount.Value.Add(sumTax).String())
	}
	expectedPay := t.TaxInclusiveAmount.Value.Decimal
	if t.PayableRoundingAmount != nil {
		expectedPay = expectedPay.Add(t.PayableRoundingAmount.Value.Decimal)
	}
	for i := range inv.WithholdingTaxTotals {
		expectedPay = expectedPay.Sub(inv.WithholdingTaxTotals[i].TaxAmount.Value.Decimal)
	}
	if !t.PayableAmount.Value.Equal(expectedPay) {
		r.errf("GOEF-TOTALS-4", "LegalMonetaryTotal/PayableAmount", "ödenecek tutar (%s) vergiler dahil toplam + yuvarlama − tevkifat (%s) olmalıdır", t.PayableAmount.Value.String(), expectedPay.String())
	}
}
