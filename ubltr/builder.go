package ubltr

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"time"

	trvalidate "github.com/YusufDrymz/go-trvalidate"
	"github.com/shopspring/decimal"
)

// Profil ve fatura tip kodlari (UBL-TR Kod Listeleri).
const (
	ProfileTemelFatura  = "TEMELFATURA"
	ProfileTicariFatura = "TICARIFATURA"
	ProfileEArsivFatura = "EARSIVFATURA"

	TypeSatis        = "SATIS"
	TypeIade         = "IADE"
	TypeTevkifat     = "TEVKIFAT"
	TypeIstisna      = "ISTISNA"
	TypeOzelMatrah   = "OZELMATRAH"
	TypeIhracKayitli = "IHRACKAYITLI"
)

// D bir decimal literal'i Dec'e cevirir; gecersiz girdide panicler.
// Sabit degerler icin: ubltr.D("20"). Calisma zamani verisi icin ParseDec.
func D(s string) Dec { return Dec{decimal.RequireFromString(s)} }

// ParseDec calisma zamani verisini Dec'e cevirir.
func ParseDec(s string) (Dec, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return Dec{}, err
	}
	return Dec{d}, nil
}

// PartyInfo builder'a taraf girisi. VKN veya TCKN'den tam biri dolu olmali;
// kurumda Name, sahista FirstName/FamilyName zorunlu (UBL-TR Ortak Elemanlar).
type PartyInfo struct {
	VKN        string
	TCKN       string
	Name       string // kurum unvani
	FirstName  string
	FamilyName string
	TaxOffice  string // vergi dairesi
	Address    Address
	Contact    *Contact
	WebsiteURI string
}

func (p PartyInfo) validate(role string) error {
	switch {
	case p.VKN != "" && p.TCKN != "":
		return fmt.Errorf("%s: VKN ve TCKN birlikte verilemez", role)
	case p.VKN != "":
		if !trvalidate.IsVKN(p.VKN) {
			return fmt.Errorf("%s: gecersiz VKN %q", role, p.VKN)
		}
		if p.Name == "" {
			return fmt.Errorf("%s: kurum icin Name zorunlu", role)
		}
	case p.TCKN != "":
		if !trvalidate.IsTCKN(p.TCKN) {
			return fmt.Errorf("%s: gecersiz TCKN %q", role, p.TCKN)
		}
		if p.FirstName == "" || p.FamilyName == "" {
			return fmt.Errorf("%s: sahis icin FirstName/FamilyName zorunlu", role)
		}
	default:
		return fmt.Errorf("%s: VKN veya TCKN zorunlu", role)
	}
	if p.Address.CitySubdivisionName == "" || p.Address.CityName == "" || p.Address.Country.Name == "" {
		return fmt.Errorf("%s: adreste ilce, il ve ulke zorunlu", role)
	}
	return nil
}

func (p PartyInfo) id() ID {
	if p.VKN != "" {
		return ID{Value: p.VKN, SchemeID: "VKN"}
	}
	return ID{Value: p.TCKN, SchemeID: "TCKN"}
}

func (p PartyInfo) toParty() Party {
	party := Party{
		WebsiteURI:           p.WebsiteURI,
		PartyIdentifications: []PartyIdentification{{ID: p.id()}},
		PostalAddress:        p.Address,
		Contact:              p.Contact,
	}
	if p.Name != "" {
		party.PartyName = &PartyName{Name: p.Name}
	}
	if p.TCKN != "" {
		party.Person = &Person{FirstName: p.FirstName, FamilyName: p.FamilyName}
	}
	if p.TaxOffice != "" {
		party.PartyTaxScheme = &PartyTaxScheme{TaxScheme: TaxScheme{Name: p.TaxOffice}}
	}
	return party
}

// Line builder'a satir girisi. VATRate yuzdedir (20 = %20). VATRate 0 ise
// ExemptionReason zorunludur (GIB schematron kurali). Iskonto tutar VEYA
// yuzde olarak verilir, ikisi birden degil.
type Line struct {
	Name            string
	Description     string
	Qty             Dec
	Unit            string // birim kodu, orn. C62 (adet)
	UnitPrice       Dec
	VATRate         Dec
	DiscountPercent Dec
	DiscountAmount  Dec
	ExemptionCode   string // UBL-TR istisna kod listesi (orn. 301)
	ExemptionReason string
}

// InvoiceBuilder UBL-TR faturayi kurar; hesap ve tutarlilik Build'de.
type InvoiceBuilder struct {
	inv      Invoice
	supplier PartyInfo
	customer PartyInfo
	lines    []Line
	xr       *ExchangeRate
}

type InvoiceOption func(*InvoiceBuilder)

func NewInvoice(opts ...InvoiceOption) *InvoiceBuilder {
	b := &InvoiceBuilder{}
	b.inv.UBLVersionID = "2.1"
	b.inv.CustomizationID = "TR1.2"
	b.inv.DocumentCurrencyCode = "TRY"
	for _, o := range opts {
		o(b)
	}
	return b
}

func WithProfile(p string) InvoiceOption { return func(b *InvoiceBuilder) { b.inv.ProfileID = p } }
func WithType(t string) InvoiceOption    { return func(b *InvoiceBuilder) { b.inv.InvoiceTypeCode = t } }
func WithID(id string) InvoiceOption     { return func(b *InvoiceBuilder) { b.inv.ID = id } }
func WithUUID(u string) InvoiceOption    { return func(b *InvoiceBuilder) { b.inv.UUID = u } }
func WithCurrency(c string) InvoiceOption {
	return func(b *InvoiceBuilder) { b.inv.DocumentCurrencyCode = c }
}
func WithIssueDate(t time.Time) InvoiceOption {
	return func(b *InvoiceBuilder) { b.inv.IssueDate = t.Format("2006-01-02") }
}
func WithIssueTime(t time.Time) InvoiceOption {
	return func(b *InvoiceBuilder) { b.inv.IssueTime = t.Format("15:04:05") }
}
func WithNote(n string) InvoiceOption {
	return func(b *InvoiceBuilder) { b.inv.Notes = append(b.inv.Notes, n) }
}
func WithSupplier(p PartyInfo) InvoiceOption { return func(b *InvoiceBuilder) { b.supplier = p } }
func WithCustomer(p PartyInfo) InvoiceOption { return func(b *InvoiceBuilder) { b.customer = p } }

// WithExchangeRate belge para birimi TRY degilse zorunlu kur bilgisini ekler
// (PricingExchangeRate; schematron kurali).
func WithExchangeRate(rate Dec, date string) InvoiceOption {
	return func(b *InvoiceBuilder) {
		b.xr = &ExchangeRate{TargetCurrencyCode: "TRY", CalculationRate: rate, Date: date}
	}
}

func (b *InvoiceBuilder) AddLine(l Line) { b.lines = append(b.lines, l) }

// GIB fatura no: 3 haneli birim kodu + 13 haneli muteselsil (ilk 4'u yil).
var invoiceIDRe = regexp.MustCompile(`^[A-Z0-9]{3}[0-9]{13}$`)

var hundred = decimal.NewFromInt(100)

// round2 tutar/vergi yuvarlamasi: 2 haneye half-up. GIB hicbir kaynakta
// yontem tanimlamiyor (schematron yalniz formati zorlar); half-up bilinçli
// tercihtir ve resmi orneklerdeki degerlerle uyumludur.
func round2(d decimal.Decimal) decimal.Decimal { return d.Round(2) }

// Build hesaplari yapar, tutarlari doldurur ve zorunlulari dogrular.
// Toplamlar yuvarlanmis satir degerlerinden turetilir; boylece satirlar ve
// belge toplamlari kurusuna kadar tutarlidir.
func (b *InvoiceBuilder) Build() (*Invoice, error) {
	inv := b.inv // kopya: Build tekrarlanabilir kalsin

	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	if inv.ProfileID == "" {
		fail("profil zorunlu (WithProfile)")
	}
	if inv.InvoiceTypeCode == "" {
		fail("fatura tipi zorunlu (WithType)")
	}
	if inv.ID == "" {
		fail("fatura no zorunlu (WithID)")
	} else if !invoiceIDRe.MatchString(inv.ID) {
		fail("fatura no %q bicimi gecersiz (3 haneli birim kodu + 13 haneli numara)", inv.ID)
	}
	if inv.IssueDate == "" {
		fail("fatura tarihi zorunlu (WithIssueDate)")
	}
	if err := b.supplier.validate("satici"); err != nil {
		errs = append(errs, err)
	}
	if err := b.customer.validate("alici"); err != nil {
		errs = append(errs, err)
	}
	if len(b.lines) == 0 {
		fail("en az bir satir zorunlu (AddLine)")
	}
	cur := inv.DocumentCurrencyCode
	if cur != "TRY" && b.xr == nil {
		fail("belge para birimi %s: kur zorunlu (WithExchangeRate)", cur)
	}

	amt := func(d decimal.Decimal) Amount { return Amount{Value: Dec{d}, CurrencyID: cur} }

	// satirlar + vergi dagilimi (oran+istisna koduna gore grupla)
	type taxKey struct{ rate, code string }
	subIdx := map[taxKey]int{}
	var subs []TaxSubtotal
	totalLEA := decimal.Zero
	totalVAT := decimal.Zero

	for i, l := range b.lines {
		n := i + 1
		if l.Name == "" {
			fail("satir %d: Name zorunlu", n)
		}
		if l.Qty.IsZero() || l.Qty.IsNegative() {
			fail("satir %d: Qty pozitif olmali", n)
		}
		if l.UnitPrice.IsNegative() {
			fail("satir %d: UnitPrice negatif olamaz", n)
		}
		if l.VATRate.IsZero() && l.ExemptionReason == "" {
			fail("satir %d: KDV 0 ise ExemptionReason zorunlu (schematron)", n)
		}
		if !l.DiscountAmount.IsZero() && !l.DiscountPercent.IsZero() {
			fail("satir %d: DiscountAmount ve DiscountPercent birlikte verilemez", n)
		}

		raw := l.Qty.Mul(l.UnitPrice.Decimal)
		discount := decimal.Zero
		switch {
		case !l.DiscountAmount.IsZero():
			discount = round2(l.DiscountAmount.Decimal)
		case !l.DiscountPercent.IsZero():
			discount = round2(raw.Mul(l.DiscountPercent.Div(hundred)))
		}
		lea := round2(raw.Sub(discount))
		if lea.IsNegative() {
			fail("satir %d: iskonto satir tutarini asiyor", n)
		}
		vat := round2(lea.Mul(l.VATRate.Div(hundred)))

		cat := TaxCategory{
			TaxExemptionReasonCode: l.ExemptionCode,
			TaxExemptionReason:     l.ExemptionReason,
			TaxScheme:              TaxScheme{Name: "KDV", TaxTypeCode: "0015"},
		}
		il := InvoiceLine{
			ID:                  strconv.Itoa(n),
			InvoicedQuantity:    Quantity{Value: l.Qty, UnitCode: l.Unit},
			LineExtensionAmount: amt(lea),
			TaxTotal: &TaxTotal{
				TaxAmount: amt(vat),
				TaxSubtotals: []TaxSubtotal{{
					TaxableAmount: &Amount{Value: Dec{lea}, CurrencyID: cur},
					TaxAmount:     amt(vat),
					Percent:       &l.VATRate,
					TaxCategory:   cat,
				}},
			},
			Item:  Item{Name: l.Name, Description: l.Description},
			Price: Price{PriceAmount: amt(l.UnitPrice.Decimal)},
		}
		if !discount.IsZero() {
			ac := AllowanceCharge{
				ChargeIndicator: false,
				Amount:          amt(discount),
				BaseAmount:      &Amount{Value: Dec{round2(raw)}, CurrencyID: cur},
			}
			if !l.DiscountPercent.IsZero() {
				mult := Dec{l.DiscountPercent.Div(hundred)}
				ac.MultiplierFactorNumeric = &mult
			}
			il.AllowanceCharges = []AllowanceCharge{ac}
		}
		inv.InvoiceLines = append(inv.InvoiceLines, il)

		key := taxKey{l.VATRate.String(), l.ExemptionCode}
		if j, ok := subIdx[key]; ok {
			subs[j].TaxableAmount.Value.Decimal = subs[j].TaxableAmount.Value.Add(lea)
			subs[j].TaxAmount.Value.Decimal = subs[j].TaxAmount.Value.Add(vat)
		} else {
			rate := l.VATRate
			subIdx[key] = len(subs)
			subs = append(subs, TaxSubtotal{
				TaxableAmount: &Amount{Value: Dec{lea}, CurrencyID: cur},
				TaxAmount:     amt(vat),
				Percent:       &rate,
				TaxCategory:   cat,
			})
		}
		totalLEA = totalLEA.Add(lea)
		totalVAT = totalVAT.Add(vat)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	inv.LineCountNumeric = len(inv.InvoiceLines)
	inv.TaxTotals = []TaxTotal{{TaxAmount: amt(totalVAT), TaxSubtotals: subs}}
	inv.LegalMonetaryTotal = MonetaryTotal{
		LineExtensionAmount: amt(totalLEA),
		TaxExclusiveAmount:  amt(totalLEA), // belge seviyesi iskonto v0.1'de yok
		TaxInclusiveAmount:  amt(totalLEA.Add(totalVAT)),
		PayableAmount:       amt(totalLEA.Add(totalVAT)),
	}

	inv.AccountingSupplierParty = SupplierParty{Party: b.supplier.toParty()}
	inv.AccountingCustomerParty = CustomerParty{Party: b.customer.toParty()}

	// UBLExtensions kilavuzda zorunlu (1..n): XAdES buraya yazilir. Imzasiz
	// uretimde GIB ornekleri gibi bos placeholder birakilir, imzaci doldurur.
	inv.UBLExtensions = &UBLExtensions{Extensions: []UBLExtension{{}}}

	// imza blogu: belge imzasiz uretilse de cac:Signature zorunlu (1..n);
	// GIB ornekleriyle ayni desen, imzanin kendisi sign katmaninin isi.
	inv.Signatures = []Signature{{
		ID:             ID{Value: b.supplier.id().Value, SchemeID: "VKN_TCKN"},
		SignatoryParty: Party{PartyIdentifications: []PartyIdentification{{ID: b.supplier.id()}}, PostalAddress: b.supplier.Address},
		DigitalSignatureAttachment: Attachment{
			ExternalReference: &ExternalReference{URI: "#Signature"},
		},
	}}

	if cur != "TRY" && b.xr != nil {
		xr := *b.xr
		xr.SourceCurrencyCode = cur
		inv.PricingExchangeRate = &xr
	}

	if inv.UUID == "" {
		u, err := newUUID()
		if err != nil {
			return nil, fmt.Errorf("ettn uretilemedi: %w", err)
		}
		inv.UUID = u
	}

	return &inv, nil
}

// newUUID ETTN icin v4 UUID uretir (GIB ornekleri buyuk harfli).
func newUUID() (string, error) {
	var u [16]byte
	if _, err := io.ReadFull(rand.Reader, u[:]); err != nil {
		return "", err
	}
	u[6] = (u[6] & 0x0f) | 0x40
	u[8] = (u[8] & 0x3f) | 0x80
	return fmt.Sprintf("%X-%X-%X-%X-%X", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16]), nil
}
