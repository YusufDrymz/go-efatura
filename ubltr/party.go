package ubltr

type SupplierParty struct {
	Party           Party    `xml:"cac:Party"`
	DespatchContact *Contact `xml:"cac:DespatchContact,omitempty"`
}

type CustomerParty struct {
	Party           Party    `xml:"cac:Party"`
	DeliveryContact *Contact `xml:"cac:DeliveryContact,omitempty"`
}

// Party is a taraf. Kurum: VKN scheme'li kimlik + PartyName zorunlu.
// Sahis: TCKN + Person zorunlu. Bu kurallar schematron'da, XSD'de degil;
// burada zorlanmaz (validate katmaninin isi).
type Party struct {
	WebsiteURI                 string                `xml:"cbc:WebsiteURI,omitempty"`
	IndustryClassificationCode string                `xml:"cbc:IndustryClassificationCode,omitempty"` // NACE
	PartyIdentifications       []PartyIdentification `xml:"cac:PartyIdentification"`
	PartyName                  *PartyName            `xml:"cac:PartyName,omitempty"`
	PostalAddress              Address               `xml:"cac:PostalAddress"`
	PartyTaxScheme             *PartyTaxScheme       `xml:"cac:PartyTaxScheme,omitempty"`
	Contact                    *Contact              `xml:"cac:Contact,omitempty"`
	Person                     *Person               `xml:"cac:Person,omitempty"`
	// TODO: PhysicalLocation, PartyLegalEntity, AgentParty (sube)
}

type PartyIdentification struct {
	ID ID `xml:"cbc:ID"`
}

type PartyName struct {
	Name string `xml:"cbc:Name"`
}

type Address struct {
	ID                  string  `xml:"cbc:ID,omitempty"` // TUIK adres kodu
	Postbox             string  `xml:"cbc:Postbox,omitempty"`
	Room                string  `xml:"cbc:Room,omitempty"`
	StreetName          string  `xml:"cbc:StreetName,omitempty"`
	BlockName           string  `xml:"cbc:BlockName,omitempty"`
	BuildingName        string  `xml:"cbc:BuildingName,omitempty"`
	BuildingNumber      string  `xml:"cbc:BuildingNumber,omitempty"`
	CitySubdivisionName string  `xml:"cbc:CitySubdivisionName"` // ilce
	CityName            string  `xml:"cbc:CityName"`            // il
	PostalZone          string  `xml:"cbc:PostalZone,omitempty"`
	Region              string  `xml:"cbc:Region,omitempty"`
	District            string  `xml:"cbc:District,omitempty"`
	Country             Country `xml:"cac:Country"`
}

type Country struct {
	IdentificationCode string `xml:"cbc:IdentificationCode,omitempty"`
	Name               string `xml:"cbc:Name"`
}

type PartyTaxScheme struct {
	RegistrationName string    `xml:"cbc:RegistrationName,omitempty"` // ihracat: yabanci unvan
	CompanyID        string    `xml:"cbc:CompanyID,omitempty"`        // ihracat: yabanci vergi no
	TaxScheme        TaxScheme `xml:"cac:TaxScheme"`                  // Name = vergi dairesi
}

type Contact struct {
	Telephone      string `xml:"cbc:Telephone,omitempty"`
	Telefax        string `xml:"cbc:Telefax,omitempty"`
	ElectronicMail string `xml:"cbc:ElectronicMail,omitempty"`
}

type Person struct {
	FirstName  string `xml:"cbc:FirstName"`
	FamilyName string `xml:"cbc:FamilyName"`
	Title      string `xml:"cbc:Title,omitempty"`
	MiddleName string `xml:"cbc:MiddleName,omitempty"`
	NameSuffix string `xml:"cbc:NameSuffix,omitempty"`
}
