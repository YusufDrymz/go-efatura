package envelope

import (
	"bytes"
	"crypto/rand"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

// Zarf turleri (schematron EnvelopeType listesi).
const (
	TypeSender  = "SENDERENVELOPE"  // fatura tasiyan zarf
	TypePostbox = "POSTBOXENVELOPE" // uygulama yaniti tasiyan zarf
	TypeSystem  = "SYSTEMENVELOPE"  // sistem yaniti tasiyan zarf
	TypeUser    = "USERENVELOPE"    // kullanici islemleri
)

// Belge turleri (schematron ElementType listesi).
const (
	ElementInvoice             = "INVOICE"
	ElementApplicationResponse = "APPLICATIONRESPONSE"
	ElementDespatchAdvice      = "DESPATCHADVICE"
	ElementReceiptAdvice       = "RECEIPTADVICE"
)

// Party zarf gonderen/alan birimdir. Alias GIB'de kayitli posta kutusu /
// gonderici birim etiketi (urn:mail:...), VKN 10 haneli vergi kimlik no
// (sahis icin 11 haneli TCKN), Title unvan.
type Party struct {
	Alias string
	VKN   string
	Title string
}

// Envelope bir SBDH zarfinin icerigi. Documents ham belge baytlaridir.
type Envelope struct {
	ID          string // zarf ID (UUID). Build'de bos birakilirsa uretilir
	Type        string // TypeSender vb. Bos ise TypeSender
	ElementType string // ElementInvoice vb. Bos ise TypeSender icin INVOICE
	Sender      Party
	Receiver    Party
	Created     time.Time // bos ise Build aninda simdi
	Documents   [][]byte
}

var uuidRe = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

// Build zarfi serialize eder. Belgeler oldugu gibi (bayt bayt) gomulur;
// imzalari bozulmaz. Yapisal sinirlar schematron'dan: tek Elements grubu,
// en fazla 1000 belge, INVOICE icin en fazla 100 fatura.
func Build(e Envelope) ([]byte, error) {
	if e.Type == "" {
		e.Type = TypeSender
	}
	if e.ElementType == "" {
		switch e.Type {
		case TypeSender:
			e.ElementType = ElementInvoice
		case TypePostbox, TypeSystem:
			e.ElementType = ElementApplicationResponse
		}
	}
	if e.ID == "" {
		id, err := newUUID()
		if err != nil {
			return nil, err
		}
		e.ID = id
	}
	if e.Created.IsZero() {
		e.Created = time.Now()
	}

	var errs []error
	if !uuidRe.MatchString(e.ID) {
		errs = append(errs, fmt.Errorf("zarf ID UUID formatinda olmali: %q", e.ID))
	}
	if e.Type != TypeSender && e.Type != TypePostbox && e.Type != TypeSystem && e.Type != TypeUser {
		errs = append(errs, fmt.Errorf("gecersiz zarf turu %q", e.Type))
	}
	for who, p := range map[string]Party{"gonderen": e.Sender, "alici": e.Receiver} {
		if p.Alias == "" || p.VKN == "" {
			errs = append(errs, fmt.Errorf("%s icin Alias ve VKN zorunlu", who))
		}
		if n := len(p.VKN); p.VKN != "" && n != 10 && n != 11 {
			errs = append(errs, fmt.Errorf("%s VKN/TCKN 10 veya 11 haneli olmali", who))
		}
	}
	if len(e.Documents) == 0 {
		errs = append(errs, errors.New("zarf en az bir belge icermeli"))
	}
	if len(e.Documents) > 1000 {
		errs = append(errs, fmt.Errorf("zarf en fazla 1000 belge tasiyabilir (%d verildi)", len(e.Documents)))
	}
	if e.ElementType == ElementInvoice && len(e.Documents) > 100 {
		errs = append(errs, fmt.Errorf("zarf en fazla 100 fatura tasiyabilir (%d verildi)", len(e.Documents)))
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	var b strings.Builder
	b.WriteString(xml.Header)
	b.WriteString(`<sh:StandardBusinessDocument xmlns:sh="http://www.unece.org/cefact/namespaces/StandardBusinessDocumentHeader" xmlns:ef="http://www.efatura.gov.tr/package-namespace" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.unece.org/cefact/namespaces/StandardBusinessDocumentHeader PackageProxy_1_2.xsd">`)
	b.WriteString("\n<sh:StandardBusinessDocumentHeader>")
	b.WriteString("\n<sh:HeaderVersion>1.2</sh:HeaderVersion>")
	writeParty(&b, "Sender", e.Sender)
	writeParty(&b, "Receiver", e.Receiver)
	b.WriteString("\n<sh:DocumentIdentification>")
	b.WriteString("\n<sh:Standard/>")
	// TypeVersionCheck: schematron 1.2 ister
	b.WriteString("\n<sh:TypeVersion>1.2</sh:TypeVersion>")
	fmt.Fprintf(&b, "\n<sh:InstanceIdentifier>%s</sh:InstanceIdentifier>", esc(e.ID))
	fmt.Fprintf(&b, "\n<sh:Type>%s</sh:Type>", esc(e.Type))
	fmt.Fprintf(&b, "\n<sh:CreationDateAndTime>%s</sh:CreationDateAndTime>", e.Created.Format("2006-01-02T15:04:05"))
	b.WriteString("\n</sh:DocumentIdentification>")
	b.WriteString("\n</sh:StandardBusinessDocumentHeader>")
	b.WriteString("\n<ef:Package>")
	b.WriteString("\n<Elements>")
	fmt.Fprintf(&b, "\n<ElementType>%s</ElementType>", esc(e.ElementType))
	fmt.Fprintf(&b, "\n<ElementCount>%d</ElementCount>", len(e.Documents))
	b.WriteString("\n<ElementList>")
	for _, doc := range e.Documents {
		b.WriteString("\n")
		b.Write(stripXMLDecl(doc))
	}
	b.WriteString("\n</ElementList>")
	b.WriteString("\n</Elements>")
	b.WriteString("\n</ef:Package>")
	b.WriteString("\n</sh:StandardBusinessDocument>")
	return []byte(b.String()), nil
}

func writeParty(b *strings.Builder, role string, p Party) {
	fmt.Fprintf(b, "\n<sh:%s>", role)
	fmt.Fprintf(b, "\n<sh:Identifier>%s</sh:Identifier>", esc(p.Alias))
	if p.Title != "" {
		fmt.Fprintf(b, "\n<sh:ContactInformation>\n<sh:Contact>%s</sh:Contact>\n<sh:ContactTypeIdentifier>UNVAN</sh:ContactTypeIdentifier>\n</sh:ContactInformation>", esc(p.Title))
	}
	fmt.Fprintf(b, "\n<sh:ContactInformation>\n<sh:Contact>%s</sh:Contact>\n<sh:ContactTypeIdentifier>VKN_TCKN</sh:ContactTypeIdentifier>\n</sh:ContactInformation>", esc(p.VKN))
	fmt.Fprintf(b, "\n</sh:%s>", role)
}

func esc(s string) string {
	var buf bytes.Buffer
	xml.EscapeText(&buf, []byte(s)) // hata donmez
	return buf.String()
}

var xmlDeclRe = regexp.MustCompile(`^\s*<\?xml[^?]*\?>\s*`)

func stripXMLDecl(doc []byte) []byte {
	return xmlDeclRe.ReplaceAll(doc, nil)
}

func newUUID() (string, error) {
	var u [16]byte
	if _, err := io.ReadFull(rand.Reader, u[:]); err != nil {
		return "", err
	}
	u[6] = (u[6] & 0x0f) | 0x40
	u[8] = (u[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16]), nil
}
