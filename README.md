# go-efatura

GİB e-Fatura / e-Arşiv belgeleri (UBL-TR 1.2) için Go kütüphanesi. Belge
modeli, parse ve deterministik XML üretimi; doğrulama, imza ve taşıma
katmanları yolda.

[![Go Reference](https://pkg.go.dev/badge/github.com/YusufDrymz/go-efatura.svg)](https://pkg.go.dev/github.com/YusufDrymz/go-efatura)
[![CI](https://github.com/YusufDrymz/go-efatura/actions/workflows/ci.yml/badge.svg)](https://github.com/YusufDrymz/go-efatura/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

> Geliştirme sürüyor. İlk hedef (v0.1) belge katmanı: UBL-TR fatura üret,
> parse et, yeniden üret. Builder ve otomatik KDV/toplam hesabı bu sürümde
> gelecek; imza ve entegratör taşıması sonraki sürümlerde.

## Neden

Türkiye e-belge ekosisteminde ciddi kütüphaneler C# ve PHP tarafında;
Go'da UBL-TR üreten, GİB kurallarını bilen bir kütüphane yok. Backend'i Go
olan herkes ya entegratörün hazır paketine kilitleniyor ya da XML'i elle
kuruyor. Bu kütüphane o boşluk için: hangi entegratörü kullanırsanız
kullanın, doğru UBL-TR belgesini üretmek ve gelen belgeyi parse etmek
ortak ihtiyaç.

## Kurulum

```bash
go get github.com/YusufDrymz/go-efatura
```

## Kullanım (bugünkü yüzey)

```go
import "github.com/YusufDrymz/go-efatura/ubltr"

data, _ := os.ReadFile("fatura.xml")
inv, err := ubltr.ParseInvoice(data)
if err != nil {
    return err
}
fmt.Println(inv.ProfileID, inv.InvoiceTypeCode) // TEMELFATURA SATIS
fmt.Println(inv.AccountingSupplierParty.Party.PartyIdentifications[0].ID.Value)
fmt.Println(inv.LegalMonetaryTotal.PayableAmount.Value) // 17.88

// yeniden üretim: GİB prefix'leriyle, deterministik
out, err := inv.XML()
```

Model, UBL-TR kılavuzlarındaki eleman sırasını birebir taşır (XSD sequence
tabanlı olduğu için alan sırası sözleşmenin parçası). Tutarlar
`shopspring/decimal` üzerine kurulu `Dec` tipiyle taşınır ve parse edilen
ölçek korunur: `18.0` geri yazarken `18` olmaz, `18.0` kalır.

## Yol haritası

| Sürüm | Katman | İçerik |
|---|---|---|
| v0.1 | `ubltr/` | belge modeli, builder, otomatik toplam/KDV hesabı, golden testler |
| v0.2 | `validate/` | GİB schematron kurallarının kritik alt kümesi (kural ID referanslı) |
| v0.3 | `sign/` | XAdES imza, pluggable `Signer` |
| v0.4 | `envelope/` | SBDH zarf + sistem yanıtı / durum kodları |
| v0.5+ | `transport/`, `earsiv/` | entegratör adaptörleri, GİB doğrudan entegrasyon, e-Arşiv raporu |

## Kapsam ve duruş

- Yalnız resmi yollar hedeflenir: UBL-TR belge + özel entegratör veya GİB
  doğrudan entegrasyon. e-Arşiv portalının resmi olmayan JSON API'si
  (earsivportal) kapsam dışıdır: dokümante değil, sık kırılıyor.
- Test verileri GİB'in kamuya açık paketlerindeki resmi örneklerden gelir
  (`ubltr/testdata/gib/`). Gerçek mükellef verisi yoktur; sentetik
  fixture'larda VKN/TCKN değerleri uydurma ama checksum-geçerlidir.
- İmza katmanı çekirdeğe bulaşmaz: entegratör kullanan çoğunluk belgeyi
  imzasız üretir, imzayı entegratör atar.

## English

Go library for Turkish electronic invoices (GİB e-Fatura / e-Arşiv,
UBL-TR 1.2 — a national customization of OASIS UBL 2.1). Currently ships
the document layer: parse official invoices and re-emit deterministic,
prefix-correct XML. Roadmap: builder with tax math (v0.1), GİB business
rule validation (v0.2), XAdES signing (v0.3), SBDH envelopes (v0.4),
integrator transports (v0.5+). Docs are in Turkish on purpose — the
domain, its terminology and its regulator are Turkish.

## License

MIT — see [LICENSE](LICENSE).
