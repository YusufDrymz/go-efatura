package validate

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// GIB UBL-TR 1.2.1 paketindeki XSD seti (xsdrt) pakete gomulu; XSD()
// calisma zamaninda bunu gecici dizine cikarip xmllint'e verir.
//
//go:embed xsd
var xsdFS embed.FS

// ErrXmllintNotFound, PATH'te xmllint olmadiginda doner. XSD dogrulamasi
// opsiyonel bir katmandir: cgo/libxml2 bagimliligina girmemek icin dis
// araca dayanir (macOS'ta hazir; debian/ubuntu: apt install libxml2-utils).
var ErrXmllintNotFound = errors.New("xmllint bulunamadi: XSD dogrulamasi icin libxml2-utils kurun")

var extractXSD = sync.OnceValues(func() (string, error) {
	dir, err := os.MkdirTemp("", "goefatura-xsd-*")
	if err != nil {
		return "", err
	}
	err = fs.WalkDir(xsdFS, "xsd", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dir, path)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := xsdFS.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "xsd", "maindoc", "UBL-Invoice-2.1.xsd"), nil
})

// XSD belgeyi GIB'in resmi UBL-TR Invoice semasina karsi dogrular.
// Sema hatalarinda xmllint ciktisiyla hata doner; belge gecerliyse nil.
func XSD(doc []byte) error {
	if _, err := exec.LookPath("xmllint"); err != nil {
		return ErrXmllintNotFound
	}
	schema, err := extractXSD()
	if err != nil {
		return fmt.Errorf("xsd seti cikarilamadi: %w", err)
	}
	f, err := os.CreateTemp("", "goefatura-doc-*.xml")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(doc); err != nil {
		f.Close()
		return err
	}
	f.Close()

	out, err := exec.Command("xmllint", "--noout", "--schema", schema, f.Name()).CombinedOutput()
	if err != nil {
		msg := strings.ReplaceAll(strings.TrimSpace(string(out)), f.Name()+":", "satir ")
		return fmt.Errorf("xsd dogrulamasi gecmedi:\n%s", msg)
	}
	return nil
}
