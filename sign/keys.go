package sign

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// NewFromPEM, PEM kodlu sertifika ve (sifresiz) ozel anahtarla imzalayici
// kurar. PKCS#1 ve PKCS#8 anahtarlar desteklenir.
func NewFromPEM(certPEM, keyPEM []byte, opts ...Option) (*XAdESSigner, error) {
	cb, _ := pem.Decode(certPEM)
	if cb == nil || cb.Type != "CERTIFICATE" {
		return nil, errors.New("sign: gecerli CERTIFICATE PEM blogu yok")
	}
	cert, err := x509.ParseCertificate(cb.Bytes)
	if err != nil {
		return nil, fmt.Errorf("sign: sertifika parse: %w", err)
	}
	kb, _ := pem.Decode(keyPEM)
	if kb == nil {
		return nil, errors.New("sign: gecerli anahtar PEM blogu yok")
	}
	var key any
	switch kb.Type {
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(kb.Bytes)
	case "PRIVATE KEY":
		key, err = x509.ParsePKCS8PrivateKey(kb.Bytes)
	default:
		return nil, fmt.Errorf("sign: desteklenmeyen PEM tipi %q", kb.Type)
	}
	if err != nil {
		return nil, fmt.Errorf("sign: anahtar parse: %w", err)
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("sign: anahtar imzalayamiyor (%T)", key)
	}
	return New(cert, signer, opts...)
}

// NewFromPKCS12, PFX/P12 dosya iceriginden imzalayici kurar. Kamu SM test
// sistemi sertifikalari bu formatta dagitilir (sifre: dosya adinin son 6
// karakteri).
func NewFromPKCS12(pfx []byte, password string, opts ...Option) (*XAdESSigner, error) {
	key, cert, err := pkcs12.Decode(pfx, password)
	if err != nil {
		return nil, fmt.Errorf("sign: pkcs12 cozulemedi: %w", err)
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("sign: pkcs12 anahtari imzalayamiyor (%T)", key)
	}
	return New(cert, signer, opts...)
}
