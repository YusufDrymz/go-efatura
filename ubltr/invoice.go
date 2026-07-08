package ubltr

import "encoding/xml"

// Invoice is a UBL-TR 1.2 invoice. Field order mirrors the element order in
// the UBL-TR Fatura guide v1.0; the GIB XSD is sequence-based, so this order
// is what makes the output schema-valid. Do not reorder.
type Invoice struct {
	XMLName xml.Name `xml:"Invoice"`

	// xmlns attributes are filled by XML(); zero on parsed documents.
	XMLNS    string `xml:"xmlns,attr,omitempty"`
	XMLNSCAC string `xml:"xmlns:cac,attr,omitempty"`
	XMLNSCBC string `xml:"xmlns:cbc,attr,omitempty"`
	XMLNSEXT string `xml:"xmlns:ext,attr,omitempty"`

	UBLExtensions   UBLExtensions `xml:"ext:UBLExtensions"`
	UBLVersionID    string        `xml:"cbc:UBLVersionID"`    // "2.1"
	CustomizationID string        `xml:"cbc:CustomizationID"` // "TR1.2"
	ProfileID       string        `xml:"cbc:ProfileID"`
	ID              string        `xml:"cbc:ID"`
	CopyIndicator   bool          `xml:"cbc:CopyIndicator"`
	UUID            string        `xml:"cbc:UUID"` // ETTN
	IssueDate       string        `xml:"cbc:IssueDate"`
	IssueTime       string        `xml:"cbc:IssueTime,omitempty"`
	InvoiceTypeCode string        `xml:"cbc:InvoiceTypeCode"`
	Notes           []string      `xml:"cbc:Note,omitempty"`

	DocumentCurrencyCode           string `xml:"cbc:DocumentCurrencyCode"`
	TaxCurrencyCode                string `xml:"cbc:TaxCurrencyCode,omitempty"`
	PricingCurrencyCode            string `xml:"cbc:PricingCurrencyCode,omitempty"`
	PaymentCurrencyCode            string `xml:"cbc:PaymentCurrencyCode,omitempty"`
	PaymentAlternativeCurrencyCode string `xml:"cbc:PaymentAlternativeCurrencyCode,omitempty"`
	AccountingCost                 string `xml:"cbc:AccountingCost,omitempty"`
	LineCountNumeric               int    `xml:"cbc:LineCountNumeric"`

	InvoicePeriod                *Period             `xml:"cac:InvoicePeriod,omitempty"`
	OrderReference               *OrderReference     `xml:"cac:OrderReference,omitempty"`
	BillingReferences            []BillingReference  `xml:"cac:BillingReference,omitempty"`
	DespatchDocumentReferences   []DocumentReference `xml:"cac:DespatchDocumentReference,omitempty"`
	ReceiptDocumentReferences    []DocumentReference `xml:"cac:ReceiptDocumentReference,omitempty"`
	OriginatorDocumentReferences []DocumentReference `xml:"cac:OriginatorDocumentReference,omitempty"`
	ContractDocumentReferences   []DocumentReference `xml:"cac:ContractDocumentReference,omitempty"`
	AdditionalDocumentReferences []DocumentReference `xml:"cac:AdditionalDocumentReference,omitempty"`
	Signatures                   []Signature         `xml:"cac:Signature"`
	AccountingSupplierParty      SupplierParty       `xml:"cac:AccountingSupplierParty"`
	AccountingCustomerParty      CustomerParty       `xml:"cac:AccountingCustomerParty"`
	BuyerCustomerParty           *CustomerParty      `xml:"cac:BuyerCustomerParty,omitempty"`
	SellerSupplierParty          *SupplierParty      `xml:"cac:SellerSupplierParty,omitempty"`
	TaxRepresentativeParty       *Party              `xml:"cac:TaxRepresentativeParty,omitempty"`
	Deliveries                   []Delivery          `xml:"cac:Delivery,omitempty"`
	PaymentMeans                 []PaymentMeans      `xml:"cac:PaymentMeans,omitempty"`
	PaymentTerms                 *PaymentTerms       `xml:"cac:PaymentTerms,omitempty"`
	AllowanceCharges             []AllowanceCharge   `xml:"cac:AllowanceCharge,omitempty"`

	TaxExchangeRate                *ExchangeRate `xml:"cac:TaxExchangeRate,omitempty"`
	PricingExchangeRate            *ExchangeRate `xml:"cac:PricingExchangeRate,omitempty"`
	PaymentExchangeRate            *ExchangeRate `xml:"cac:PaymentExchangeRate,omitempty"`
	PaymentAlternativeExchangeRate *ExchangeRate `xml:"cac:PaymentAlternativeExchangeRate,omitempty"`

	TaxTotals            []TaxTotal    `xml:"cac:TaxTotal"`
	WithholdingTaxTotals []TaxTotal    `xml:"cac:WithholdingTaxTotal,omitempty"`
	LegalMonetaryTotal   MonetaryTotal `xml:"cac:LegalMonetaryTotal"`
	InvoiceLines         []InvoiceLine `xml:"cac:InvoiceLine"`
}

// UBLExtensions imza tasiyicisidir ve GIB XSD'sinde zorunludur; ustelik
// ExtensionContent en az bir yabanci-namespace cocuk ister (lax wildcard —
// bos ds:Signature bile gecmez, sema sette oldugu icin dogrulanir). Bu
// yuzden marshal her zaman asagidaki placeholder'i yazar; imza katmani onu
// gercek XAdES icerigiyle degistirir. Parse icerigi atlar: imza baytlari
// nesne modeli uzerinden korunamaz (yeniden uretim imzayi bozar).
type UBLExtensions struct{}

const placeholderNS = "urn:go-efatura:signature-placeholder"

func (UBLExtensions) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	els := []xml.StartElement{
		{Name: start.Name},
		{Name: xml.Name{Local: "ext:UBLExtension"}},
		{Name: xml.Name{Local: "ext:ExtensionContent"}},
		{Name: xml.Name{Local: "sp:SignaturePlaceholder"}, Attr: []xml.Attr{
			{Name: xml.Name{Local: "xmlns:sp"}, Value: placeholderNS},
		}},
	}
	for _, el := range els {
		if err := e.EncodeToken(el); err != nil {
			return err
		}
	}
	for i := len(els) - 1; i >= 0; i-- {
		if err := e.EncodeToken(xml.EndElement{Name: els[i].Name}); err != nil {
			return err
		}
	}
	return nil
}

func (*UBLExtensions) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	return d.Skip()
}

type Period struct {
	StartDate       string    `xml:"cbc:StartDate,omitempty"`
	StartTime       string    `xml:"cbc:StartTime,omitempty"`
	EndDate         string    `xml:"cbc:EndDate,omitempty"`
	EndTime         string    `xml:"cbc:EndTime,omitempty"`
	DurationMeasure *Quantity `xml:"cbc:DurationMeasure,omitempty"`
	Description     string    `xml:"cbc:Description,omitempty"`
}

type OrderReference struct {
	ID        string `xml:"cbc:ID"`
	IssueDate string `xml:"cbc:IssueDate"`
}

type BillingReference struct {
	InvoiceDocumentReference *DocumentReference `xml:"cac:InvoiceDocumentReference,omitempty"`
	// TODO: OKC bilgi fisi / EFT-POS referanslari (433 ve 483 no.lu teblig)
}

type DocumentReference struct {
	ID                  string `xml:"cbc:ID"`
	IssueDate           string `xml:"cbc:IssueDate"`
	DocumentTypeCode    string `xml:"cbc:DocumentTypeCode,omitempty"`
	DocumentType        string `xml:"cbc:DocumentType,omitempty"`
	DocumentDescription string `xml:"cbc:DocumentDescription,omitempty"`
	IssuerParty         *Party `xml:"cac:IssuerParty,omitempty"` // orn. istisna belgesini veren kurum
	// TODO: Attachment (base64 gomulu belge)
}

type Signature struct {
	ID                         ID         `xml:"cbc:ID"`
	SignatoryParty             Party      `xml:"cac:SignatoryParty"`
	DigitalSignatureAttachment Attachment `xml:"cac:DigitalSignatureAttachment"`
}

type Attachment struct {
	ExternalReference *ExternalReference `xml:"cac:ExternalReference,omitempty"`
}

type ExternalReference struct {
	URI string `xml:"cbc:URI"`
}

type Delivery struct {
	ID                 *ID             `xml:"cbc:ID,omitempty"`
	ActualDeliveryDate string          `xml:"cbc:ActualDeliveryDate,omitempty"`
	ActualDeliveryTime string          `xml:"cbc:ActualDeliveryTime,omitempty"`
	DeliveryAddress    *Address        `xml:"cac:DeliveryAddress,omitempty"`
	CarrierParty       *Party          `xml:"cac:CarrierParty,omitempty"`
	DeliveryTerms      []DeliveryTerms `xml:"cac:DeliveryTerms,omitempty"`
	Shipment           *Shipment       `xml:"cac:Shipment,omitempty"`
}

type DeliveryTerms struct {
	ID ID `xml:"cbc:ID"` // teslim sekli (INCOTERMS, orn. CIF)
}

// Shipment tasima bilgisi. Ihracat/istisna faturalarinda navlun
// (DeclaredForCarriageValueAmount) ve sigorta (InsuranceValueAmount) burada
// tasinir ve GIB ornekleri bunlari belge toplamina dahil eder.
type Shipment struct {
	ID                                 ID        `xml:"cbc:ID"`
	GrossWeightMeasure                 *Quantity `xml:"cbc:GrossWeightMeasure,omitempty"`
	NetWeightMeasure                   *Quantity `xml:"cbc:NetWeightMeasure,omitempty"`
	TotalTransportHandlingUnitQuantity *Dec      `xml:"cbc:TotalTransportHandlingUnitQuantity,omitempty"`
	InsuranceValueAmount               *Amount   `xml:"cbc:InsuranceValueAmount,omitempty"`
	DeclaredCustomsValueAmount         *Amount   `xml:"cbc:DeclaredCustomsValueAmount,omitempty"`
	DeclaredForCarriageValueAmount     *Amount   `xml:"cbc:DeclaredForCarriageValueAmount,omitempty"`
	// TODO: GoodsItem, ShipmentStage, TransportHandlingUnit (e-irsaliye isleri)
}

type PaymentMeans struct {
	PaymentMeansCode      string            `xml:"cbc:PaymentMeansCode"`
	PaymentDueDate        string            `xml:"cbc:PaymentDueDate,omitempty"`
	PaymentChannelCode    string            `xml:"cbc:PaymentChannelCode,omitempty"`
	InstructionNote       string            `xml:"cbc:InstructionNote,omitempty"`
	PayeeFinancialAccount *FinancialAccount `xml:"cac:PayeeFinancialAccount,omitempty"`
}

type FinancialAccount struct {
	ID                         string                      `xml:"cbc:ID"` // IBAN
	CurrencyCode               string                      `xml:"cbc:CurrencyCode,omitempty"`
	PaymentNote                string                      `xml:"cbc:PaymentNote,omitempty"`
	FinancialInstitutionBranch *FinancialInstitutionBranch `xml:"cac:FinancialInstitutionBranch,omitempty"`
}

type FinancialInstitutionBranch struct {
	Name                 string                `xml:"cbc:Name,omitempty"` // sube
	FinancialInstitution *FinancialInstitution `xml:"cac:FinancialInstitution,omitempty"`
}

type FinancialInstitution struct {
	Name string `xml:"cbc:Name,omitempty"` // banka
}

type PaymentTerms struct {
	Note                    string  `xml:"cbc:Note,omitempty"`
	PenaltySurchargePercent *Dec    `xml:"cbc:PenaltySurchargePercent,omitempty"`
	Amount                  *Amount `xml:"cbc:Amount,omitempty"`
	PenaltyAmount           *Amount `xml:"cbc:PenaltyAmount,omitempty"`
	PaymentDueDate          string  `xml:"cbc:PaymentDueDate,omitempty"`
}

type ExchangeRate struct {
	SourceCurrencyCode string `xml:"cbc:SourceCurrencyCode"`
	TargetCurrencyCode string `xml:"cbc:TargetCurrencyCode"`
	CalculationRate    Dec    `xml:"cbc:CalculationRate"`
	Date               string `xml:"cbc:Date,omitempty"`
}

type MonetaryTotal struct {
	LineExtensionAmount   Amount  `xml:"cbc:LineExtensionAmount"`
	TaxExclusiveAmount    Amount  `xml:"cbc:TaxExclusiveAmount"`
	TaxInclusiveAmount    Amount  `xml:"cbc:TaxInclusiveAmount"`
	AllowanceTotalAmount  *Amount `xml:"cbc:AllowanceTotalAmount,omitempty"`
	ChargeTotalAmount     *Amount `xml:"cbc:ChargeTotalAmount,omitempty"`
	PayableRoundingAmount *Amount `xml:"cbc:PayableRoundingAmount,omitempty"`
	PayableAmount         Amount  `xml:"cbc:PayableAmount"`
}
