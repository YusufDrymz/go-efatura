package envelope

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// Zip zarfi GIB'in beklediği pakete koyar: tek dosyali zip, dosya adi
// zarf ID'si (aksi 1131/1133 durum kodlariyla reddedilir).
func Zip(envelopeXML []byte, envelopeID string) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create(envelopeID + ".xml")
	if err != nil {
		return nil, err
	}
	if _, err := f.Write(envelopeXML); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// OpenZip zip paketinden zarfi cikarip acar. GIB kurali uygulanir:
// zip tam bir .xml dosyasi icermelidir.
func OpenZip(data []byte) (*Envelope, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("envelope: zip acilamadi: %w", err)
	}
	if len(zr.File) != 1 {
		return nil, fmt.Errorf("envelope: zip tam bir dosya icermeli (%d bulundu)", len(zr.File))
	}
	f := zr.File[0]
	if !strings.HasSuffix(strings.ToLower(f.Name), ".xml") {
		return nil, fmt.Errorf("envelope: zip icindeki dosya xml degil: %q", f.Name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	raw, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	env, err := Open(raw)
	if err != nil {
		return nil, err
	}
	if base := strings.TrimSuffix(f.Name, ".xml"); env.ID != "" && base != env.ID {
		return nil, fmt.Errorf("envelope: zip dosya adi (%s) zarf ID'siyle (%s) ayni olmali", base, env.ID)
	}
	return env, nil
}
