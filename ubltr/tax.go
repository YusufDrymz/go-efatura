package ubltr

// TaxTotal is used in three places: invoice level (mandatory), line level
// (optional) and as WithholdingTaxTotal on tevkifat invoices.
type TaxTotal struct {
	TaxAmount    Amount        `xml:"cbc:TaxAmount"`
	TaxSubtotals []TaxSubtotal `xml:"cac:TaxSubtotal"`
}

type TaxSubtotal struct {
	TaxableAmount                *Amount     `xml:"cbc:TaxableAmount,omitempty"` // matrah
	TaxAmount                    Amount      `xml:"cbc:TaxAmount"`
	CalculationSequenceNumeric   *Dec        `xml:"cbc:CalculationSequenceNumeric,omitempty"`
	TransactionCurrencyTaxAmount *Amount     `xml:"cbc:TransactionCurrencyTaxAmount,omitempty"`
	Percent                      *Dec        `xml:"cbc:Percent,omitempty"`
	BaseUnitMeasure              *Quantity   `xml:"cbc:BaseUnitMeasure,omitempty"`
	PerUnitAmount                *Amount     `xml:"cbc:PerUnitAmount,omitempty"` // maktu vergi
	TaxCategory                  TaxCategory `xml:"cac:TaxCategory"`
}

type TaxCategory struct {
	Name                   string    `xml:"cbc:Name,omitempty"`
	TaxExemptionReasonCode string    `xml:"cbc:TaxExemptionReasonCode,omitempty"` // istisna kodu (orn. 301)
	TaxExemptionReason     string    `xml:"cbc:TaxExemptionReason,omitempty"`
	TaxScheme              TaxScheme `xml:"cac:TaxScheme"`
}

type TaxScheme struct {
	ID          string `xml:"cbc:ID,omitempty"`
	Name        string `xml:"cbc:Name,omitempty"`
	TaxTypeCode string `xml:"cbc:TaxTypeCode,omitempty"` // KDV=0015; UBL-TR Kod Listeleri
}
