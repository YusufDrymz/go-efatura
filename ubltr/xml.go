package ubltr

import (
	"bytes"
	"encoding/xml"
)

// UBL 2.1 namespaces with the fixed prefixes GIB documents use.
const (
	nsInvoice = "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
	nsCBC     = "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"
	nsCAC     = "urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
	nsEXT     = "urn:oasis:names:specification:ubl:schema:xsd:CommonExtensionComponents-2"
)

var nsPrefix = map[string]string{
	nsInvoice: "",
	nsCBC:     "cbc",
	nsCAC:     "cac",
	nsEXT:     "ext",
}

// prefixer rewrites resolved namespace URIs back to the fixed prefixes used
// in struct tags. encoding/xml matches prefixed tag names literally on
// unmarshal, so without this rewrite a single struct set cannot both parse
// namespaced GIB documents and emit prefixed XML.
type prefixer struct{ d *xml.Decoder }

func (p prefixer) Token() (xml.Token, error) {
	t, err := p.d.Token()
	if err != nil {
		return t, err
	}
	switch e := t.(type) {
	case xml.StartElement:
		e.Name = prefixName(e.Name)
		attrs := e.Attr[:0]
		for _, a := range e.Attr {
			// xmlns bildirimlerini at: sabit prefix'ler zaten biliniyor
			if a.Name.Space == "xmlns" || a.Name.Local == "xmlns" {
				continue
			}
			attrs = append(attrs, a)
		}
		e.Attr = attrs
		return e, nil
	case xml.EndElement:
		e.Name = prefixName(e.Name)
		return e, nil
	}
	return t, nil
}

func prefixName(n xml.Name) xml.Name {
	p, ok := nsPrefix[n.Space]
	if !ok {
		return n
	}
	if p == "" {
		return xml.Name{Local: n.Local}
	}
	return xml.Name{Local: p + ":" + n.Local}
}

// ParseInvoice decodes a UBL-TR invoice document. Signature content inside
// UBLExtensions is not preserved; parse a signed document for its data, not
// to re-emit it signed.
func ParseInvoice(data []byte) (*Invoice, error) {
	var inv Invoice
	d := xml.NewTokenDecoder(prefixer{xml.NewDecoder(bytes.NewReader(data))})
	if err := d.Decode(&inv); err != nil {
		return nil, err
	}
	return &inv, nil
}

// XML serializes the invoice with GIB namespace prefixes and an XML
// declaration. Value receiver on purpose: namespace attributes are set on a
// copy so parse -> XML -> parse stays deep-equal on the original.
func (inv Invoice) XML() ([]byte, error) {
	inv.XMLNS = nsInvoice
	inv.XMLNSCAC = nsCAC
	inv.XMLNSCBC = nsCBC
	inv.XMLNSEXT = nsEXT
	out, err := xml.MarshalIndent(inv, "", "    ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}
