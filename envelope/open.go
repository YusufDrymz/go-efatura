package envelope

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"time"
)

// Open zarfi ayristirir. Belgeler orijinal baytlariyla dilimlenir
// (yeniden serialize edilmez), boylece iclerindeki imzalar dogrulanabilir
// kalir. Baslik alanlarinda esnek davranir; yapisal dogrulama gonderme
// tarafinin isidir.
func Open(data []byte) (*Envelope, error) {
	e := &Envelope{}
	d := xml.NewDecoder(bytes.NewReader(data))

	var path []string
	var text bytes.Buffer
	var contact, contactType string
	var inElementList bool

	at := func(names ...string) bool {
		if len(path) != len(names) {
			return false
		}
		for i, n := range names {
			if path[i] != n {
				return false
			}
		}
		return true
	}

	for {
		start := d.InputOffset()
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("envelope: zarf parse edilemedi: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			// ElementList'in dogrudan cocugu: ham baytlari dilimle
			if inElementList {
				end, err := skipElement(d)
				if err != nil {
					return nil, err
				}
				e.Documents = append(e.Documents, data[start:end])
				continue
			}
			path = append(path, t.Name.Local)
			text.Reset()
			if at("StandardBusinessDocument", "Package", "Elements", "ElementList") {
				inElementList = true
			}
		case xml.CharData:
			text.Write(t)
		case xml.EndElement:
			val := string(bytes.TrimSpace(text.Bytes()))
			switch {
			case at("StandardBusinessDocument", "StandardBusinessDocumentHeader", "Sender", "Identifier"):
				e.Sender.Alias = val
			case at("StandardBusinessDocument", "StandardBusinessDocumentHeader", "Receiver", "Identifier"):
				e.Receiver.Alias = val
			case at("StandardBusinessDocument", "StandardBusinessDocumentHeader", "Sender", "ContactInformation", "Contact"),
				at("StandardBusinessDocument", "StandardBusinessDocumentHeader", "Receiver", "ContactInformation", "Contact"):
				contact = val
			case at("StandardBusinessDocument", "StandardBusinessDocumentHeader", "Sender", "ContactInformation", "ContactTypeIdentifier"),
				at("StandardBusinessDocument", "StandardBusinessDocumentHeader", "Receiver", "ContactInformation", "ContactTypeIdentifier"):
				contactType = val
			case at("StandardBusinessDocument", "StandardBusinessDocumentHeader", "Sender", "ContactInformation"),
				at("StandardBusinessDocument", "StandardBusinessDocumentHeader", "Receiver", "ContactInformation"):
				p := &e.Sender
				if path[2] == "Receiver" {
					p = &e.Receiver
				}
				switch contactType {
				case "VKN_TCKN":
					p.VKN = contact
				case "UNVAN":
					p.Title = contact
				}
				contact, contactType = "", ""
			case at("StandardBusinessDocument", "StandardBusinessDocumentHeader", "DocumentIdentification", "InstanceIdentifier"):
				e.ID = val
			case at("StandardBusinessDocument", "StandardBusinessDocumentHeader", "DocumentIdentification", "Type"):
				e.Type = val
			case at("StandardBusinessDocument", "StandardBusinessDocumentHeader", "DocumentIdentification", "CreationDateAndTime"):
				e.Created, _ = time.Parse("2006-01-02T15:04:05", val)
			case at("StandardBusinessDocument", "Package", "Elements", "ElementType"):
				if e.ElementType == "" {
					e.ElementType = val
				}
			case at("StandardBusinessDocument", "Package", "Elements", "ElementList"):
				inElementList = false
			}
			path = path[:len(path)-1]
			text.Reset()
		}
	}
	if e.ID == "" && len(e.Documents) == 0 {
		return nil, errors.New("envelope: SBDH zarfi degil (InstanceIdentifier ve belge yok)")
	}
	return e, nil
}

// skipElement acilmis elemanin sonuna kadar ilerler ve bitis offset'ini
// dondurur (kapanis tag'i dahil).
func skipElement(d *xml.Decoder) (int64, error) {
	depth := 1
	for depth > 0 {
		tok, err := d.Token()
		if err != nil {
			return 0, fmt.Errorf("envelope: belge sinirlari okunamadi: %w", err)
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return d.InputOffset(), nil
}
