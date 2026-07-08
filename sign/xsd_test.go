package sign

import (
	"os/exec"
	"testing"

	"github.com/YusufDrymz/go-efatura/ubltr"
	"github.com/YusufDrymz/go-efatura/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Imzali belge XSD'den gecmeli: lax wildcard, ds/xades icerigini gomulu
// semalara karsi fiilen dogrular — yapisal hatalar burada patlar.
func TestSignedPassesXSD(t *testing.T) {
	if _, err := exec.LookPath("xmllint"); err != nil {
		t.Skip("xmllint yok")
	}
	signed, _ := signedInvoice(t)
	assert.NoError(t, validate.XSD(signed))
}

// Imzali belge is kurallarindan da gecmeli (parse imzayi dusurur, kalan
// icerik degismemistir).
func TestSignedPassesRules(t *testing.T) {
	signed, _ := signedInvoice(t)
	inv, err := ubltr.ParseInvoice(signed)
	require.NoError(t, err)
	assert.Empty(t, validate.Invoice(inv))
}
