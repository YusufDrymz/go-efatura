package ubltr

type InvoiceLine struct {
	ID                   string            `xml:"cbc:ID"`
	Note                 string            `xml:"cbc:Note,omitempty"`
	InvoicedQuantity     Quantity          `xml:"cbc:InvoicedQuantity"`
	LineExtensionAmount  Amount            `xml:"cbc:LineExtensionAmount"` // miktar x birim fiyat - iskonto
	AllowanceCharges     []AllowanceCharge `xml:"cac:AllowanceCharge,omitempty"`
	TaxTotal             *TaxTotal         `xml:"cac:TaxTotal,omitempty"`
	WithholdingTaxTotals []TaxTotal        `xml:"cac:WithholdingTaxTotal,omitempty"`
	Item                 Item              `xml:"cac:Item"`
	Price                Price             `xml:"cac:Price"`
	// TODO: OrderLineReference, DespatchLineReference, Delivery, SubInvoiceLine
}

type Item struct {
	Description string `xml:"cbc:Description,omitempty"`
	Name        string `xml:"cbc:Name"`
	BrandName   string `xml:"cbc:BrandName,omitempty"`
	ModelName   string `xml:"cbc:ModelName,omitempty"`
	// TODO: Sellers/Manufacturers/BuyersItemIdentification
}

type Price struct {
	PriceAmount Amount `xml:"cbc:PriceAmount"`
}

type AllowanceCharge struct {
	ChargeIndicator         bool    `xml:"cbc:ChargeIndicator"` // false=iskonto, true=artirim
	AllowanceChargeReason   string  `xml:"cbc:AllowanceChargeReason,omitempty"`
	MultiplierFactorNumeric *Dec    `xml:"cbc:MultiplierFactorNumeric,omitempty"`
	Amount                  Amount  `xml:"cbc:Amount"`
	BaseAmount              *Amount `xml:"cbc:BaseAmount,omitempty"`
}
