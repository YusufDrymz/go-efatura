package ubltr

import (
	"encoding/xml"

	"github.com/shopspring/decimal"
)

// Dec is a decimal that keeps its scale through XML round-trips: a parsed
// "18.0" marshals back as "18.0", not "18". GIB samples carry meaningful
// trailing zeros and decimal.Decimal.String() strips them.
type Dec struct{ decimal.Decimal }

func (d Dec) MarshalText() ([]byte, error) {
	if exp := d.Exponent(); exp < 0 {
		return []byte(d.StringFixed(-exp)), nil
	}
	return []byte(d.String()), nil
}

// Amount is a monetary value with the currencyID attribute. currencyID is
// emitted even when empty so a missing currency shows up in output instead
// of silently producing invalid XML.
type Amount struct {
	Value      Dec
	CurrencyID string
}

func (a Amount) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "currencyID"}, Value: a.CurrencyID})
	return e.EncodeElement(a.Value, start)
}

func (a *Amount) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, at := range start.Attr {
		if at.Name.Local == "currencyID" {
			a.CurrencyID = at.Value
		}
	}
	return d.DecodeElement(&a.Value, &start)
}

// Quantity is a measured value with a unitCode attribute (UN/ECE Rec 20).
// unitCode is required by GIB schematron but some official samples omit it,
// so it is only emitted when set.
type Quantity struct {
	Value    Dec
	UnitCode string
}

func (q Quantity) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if q.UnitCode != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "unitCode"}, Value: q.UnitCode})
	}
	return e.EncodeElement(q.Value, start)
}

func (q *Quantity) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, at := range start.Attr {
		if at.Name.Local == "unitCode" {
			q.UnitCode = at.Value
		}
	}
	return d.DecodeElement(&q.Value, &start)
}

// ID is an identifier with an optional schemeID (VKN, TCKN, ...).
type ID struct {
	Value    string `xml:",chardata"`
	SchemeID string `xml:"schemeID,attr,omitempty"`
}
