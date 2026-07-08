package ubltr

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Uretilen XML, GIB'in resmi XSD'sine karsi dogrulanir (testdata/xsd,
// UBL-TR 1.2.1 paketinden). xmllint PATH'te yoksa test atlanir; CI kurar.
func TestXSDValidation(t *testing.T) {
	if _, err := exec.LookPath("xmllint"); err != nil {
		t.Skip("xmllint yok, xsd dogrulamasi atlandi")
	}
	xsd, err := filepath.Abs("testdata/xsd/maindoc/UBL-Invoice-2.1.xsd")
	require.NoError(t, err)

	validate := func(t *testing.T, doc []byte) {
		t.Helper()
		f := filepath.Join(t.TempDir(), "doc.xml")
		require.NoError(t, os.WriteFile(f, doc, 0o644))
		out, err := exec.Command("xmllint", "--noout", "--schema", xsd, f).CombinedOutput()
		if err != nil {
			t.Fatalf("xsd dogrulamasi gecmedi:\n%s", out)
		}
	}

	t.Run("builder", func(t *testing.T) {
		b := NewInvoice(
			WithProfile(ProfileTemelFatura),
			WithType(TypeSatis),
			WithID("ABC2026000000001"),
			WithIssueDate(time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)),
			WithSupplier(testSupplier()),
			WithCustomer(testCustomer()),
		)
		b.AddLine(Line{Name: "Danışmanlık", Qty: D("2"), Unit: "C62", UnitPrice: D("1500"), VATRate: D("20")})
		b.AddLine(Line{Name: "Lisans", Qty: D("1"), Unit: "C62", UnitPrice: D("850.50"), VATRate: D("20"), DiscountPercent: D("10")})
		inv, err := b.Build()
		require.NoError(t, err)
		out, err := inv.XML()
		require.NoError(t, err)
		validate(t, out)
	})

	// resmi orneklerin yeniden uretimi de sema-gecerli olmali
	for _, name := range gibSamples {
		t.Run("reemit-"+name, func(t *testing.T) {
			inv, err := ParseInvoice(readFixture(t, name))
			require.NoError(t, err)
			out, err := inv.XML()
			require.NoError(t, err)
			validate(t, out)
		})
	}
}
