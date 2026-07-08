package validate

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/YusufDrymz/go-efatura/ubltr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func skipWithoutXmllint(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("xmllint"); err != nil {
		t.Skip("xmllint yok, xsd dogrulamasi atlandi")
	}
}

func TestXSDBuilderOutput(t *testing.T) {
	skipWithoutXmllint(t)
	out, err := validInvoice(t).XML()
	require.NoError(t, err)
	assert.NoError(t, XSD(out))
}

// Resmi orneklerin yeniden uretimi de sema-gecerli olmali.
func TestXSDReemittedSamples(t *testing.T) {
	skipWithoutXmllint(t)
	files, err := os.ReadDir("../ubltr/testdata/gib")
	require.NoError(t, err)
	for _, f := range files {
		t.Run(f.Name(), func(t *testing.T) {
			data, err := os.ReadFile("../ubltr/testdata/gib/" + f.Name())
			require.NoError(t, err)
			inv, err := ubltr.ParseInvoice(data)
			require.NoError(t, err)
			out, err := inv.XML()
			require.NoError(t, err)
			assert.NoError(t, XSD(out))
		})
	}
}

func TestXSDRejectsInvalid(t *testing.T) {
	skipWithoutXmllint(t)
	out, err := validInvoice(t).XML()
	require.NoError(t, err)
	// eleman sirasini bozan bilinmeyen bir eleman sema hatasi vermeli
	broken := strings.Replace(string(out), "<cbc:ID>", "<cbc:Bilinmeyen>x</cbc:Bilinmeyen><cbc:ID>", 1)
	assert.Error(t, XSD([]byte(broken)))
}
