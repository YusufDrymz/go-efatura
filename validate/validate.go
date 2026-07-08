package validate

import (
	"fmt"

	"github.com/YusufDrymz/go-efatura/ubltr"
)

type Severity int

const (
	Error Severity = iota
	Warning
)

func (s Severity) String() string {
	if s == Warning {
		return "uyari"
	}
	return "hata"
}

// Issue tek bir kural ihlalidir. Rule, GIB schematron'undaki abstract rule
// ID'si ya da GOEF- onekli go-efatura ek kuralidir.
type Issue struct {
	Rule     string
	Severity Severity
	Path     string
	Message  string
}

func (i Issue) String() string {
	return fmt.Sprintf("[%s] %s: %s (%s)", i.Severity, i.Rule, i.Message, i.Path)
}

// Errors yalnizca Error seviyesindeki bulgulari suzer.
func Errors(issues []Issue) []Issue {
	var out []Issue
	for _, i := range issues {
		if i.Severity == Error {
			out = append(out, i)
		}
	}
	return out
}

type report struct{ issues []Issue }

func (r *report) errf(rule, path, format string, a ...any) {
	r.issues = append(r.issues, Issue{Rule: rule, Severity: Error, Path: path, Message: fmt.Sprintf(format, a...)})
}

func (r *report) warnf(rule, path, format string, a ...any) {
	r.issues = append(r.issues, Issue{Rule: rule, Severity: Warning, Path: path, Message: fmt.Sprintf(format, a...)})
}

// Invoice faturaya kritik GIB is kurallarini uygular ve bulgulari dondurur.
// Bos sonuc "kurallardan gecti" demektir; schematron'un tamamina degil,
// buradaki alt kumeye gore (bkz. paket dokumantasyonu).
func Invoice(inv *ubltr.Invoice) []Issue {
	r := &report{}
	for _, check := range invoiceRules {
		check(inv, r)
	}
	return r.issues
}

var invoiceRules = []func(*ubltr.Invoice, *report){
	checkHeader,
	checkIssueDate,
	checkTypeAndProfile,
	checkCurrencies,
	checkParties,
	checkSignatures,
	checkTaxes,
	checkWithholding,
	checkExemptions,
	checkIadeReferences,
	checkLines,
	checkPaymentMeans,
	checkDecimals,
	checkTotals,
}
