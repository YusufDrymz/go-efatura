# go-efatura

GİB e-Fatura / e-Arşiv belgeleri (UBL-TR 1.2) için Go kütüphanesi. Belge
modeli, parse ve deterministik XML üretimi + GİB iş kuralı doğrulaması;
imza ve taşıma katmanları yolda.

[![Go Reference](https://pkg.go.dev/badge/github.com/YusufDrymz/go-efatura.svg)](https://pkg.go.dev/github.com/YusufDrymz/go-efatura)
[![CI](https://github.com/YusufDrymz/go-efatura/actions/workflows/ci.yml/badge.svg)](https://github.com/YusufDrymz/go-efatura/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

> Geliştirme sürüyor. v0.1 belge katmanını (builder + parse), v0.2
> doğrulama katmanını getirdi. İmza (XAdES) ve entegratör taşıması
> sonraki sürümlerde.

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

## Kullanım

Fatura kurma — satır tutarı, KDV dağılımı ve belge toplamları otomatik
hesaplanır, VKN/TCKN checksum'ları
[go-trvalidate](https://github.com/YusufDrymz/go-trvalidate) ile doğrulanır:

```go
import "github.com/YusufDrymz/go-efatura/ubltr"

b := ubltr.NewInvoice(
    ubltr.WithProfile(ubltr.ProfileTemelFatura),
    ubltr.WithType(ubltr.TypeSatis),
    ubltr.WithID("ABC2026000000001"),
    ubltr.WithIssueDate(time.Now()),
    ubltr.WithSupplier(ubltr.PartyInfo{VKN: "9990000005", Name: "Örnek A.Ş.", TaxOffice: "Beşiktaş",
        Address: ubltr.Address{CitySubdivisionName: "Beşiktaş", CityName: "İstanbul",
            Country: ubltr.Country{Name: "Türkiye"}}}),
    ubltr.WithCustomer(ubltr.PartyInfo{TCKN: "99900000074", FirstName: "Ali", FamilyName: "Yılmaz",
        Address: ubltr.Address{CitySubdivisionName: "Çankaya", CityName: "Ankara",
            Country: ubltr.Country{Name: "Türkiye"}}}),
)
b.AddLine(ubltr.Line{Name: "Danışmanlık", Qty: ubltr.D("2"), Unit: "C62",
    UnitPrice: ubltr.D("1500"), VATRate: ubltr.D("20")})

inv, err := b.Build() // hesap + dogrulama burada
if err != nil {
    return err // birden fazla hata errors.Join ile birlikte doner
}
out, err := inv.XML()
```

`Build` eksik/geçersiz her alanı ayrı raporlar (profil, fatura no biçimi,
checksum, adres, kur, istisna gerekçesi...) ve hepsini tek seferde döner —
tek tek düzeltip yeniden denemek gerekmez. Üretilen XML, testlerde GİB'in
resmi XSD'sine karşı `xmllint` ile doğrulanır (`ubltr/testdata/xsd/`).

Gelen faturayı parse etme:

```go
inv, err := ubltr.ParseInvoice(data)
fmt.Println(inv.ProfileID, inv.InvoiceTypeCode) // TEMELFATURA SATIS
fmt.Println(inv.LegalMonetaryTotal.PayableAmount.Value) // 17.88
```

Göndermeden (veya gelen belgeyi işlemeden) önce GİB iş kurallarıyla doğrulama:

```go
import "github.com/YusufDrymz/go-efatura/validate"

issues := validate.Invoice(inv)
for _, is := range issues {
    fmt.Println(is) // [hata] InvoicedQuantityCheck: unitCode niteliği zorunludur (InvoiceLine[1]/InvoicedQuantity)
}
if len(validate.Errors(issues)) == 0 {
    // kurallardan geçti
}

// opsiyonel XSD katmanı: xmllint gerektirir, şema seti pakete gömülü
if err := validate.XSD(xmlBytes); err != nil { ... }
```

Kuralların kaynağı GİB'in resmi schematron dosyalarıdır; her bulgu,
schematron'daki kural ID'siyle gelir (`UBLVersionIDCheck`, `decimalCheck`,
`WithholdingTaxTotalCheck`...). `GOEF-` önekli kurallar go-efatura'nın ek
kurallarıdır: GİB schematron'u aritmetik tutarlılığı ve VKN/TCKN
checksum'ını hiç denetlemez — toplam formülleri ve checksum'lar burada
doğrulanır. Kapsam kritik fatura alt kümesidir; zarf ve e-İrsaliye kuralları
sonraki fazlarda.

Çalışan örnek: [`examples/`](examples/main.go). Yuvarlama tercihi: satır ve
vergi tutarları 2 haneye half-up yuvarlanır, toplamlar yuvarlanmış
değerlerden türetilir — GİB hiçbir kılavuzda yöntem tanımlamadığı için bu
bilinçli ve dokümante bir tercihtir; resmi örneklerdeki değerlerle uyumludur.

Model, UBL-TR kılavuzlarındaki eleman sırasını birebir taşır (XSD sequence
tabanlı olduğu için alan sırası sözleşmenin parçası). Tutarlar
`shopspring/decimal` üzerine kurulu `Dec` tipiyle taşınır ve parse edilen
ölçek korunur: `18.0` geri yazarken `18` olmaz, `18.0` kalır.

## Yol haritası

| Sürüm | Katman | İçerik |
|---|---|---|
| v0.1 ✓ | `ubltr/` | belge modeli, builder, otomatik toplam/KDV hesabı, golden testler |
| v0.2 ✓ | `validate/` | GİB schematron kurallarının kritik alt kümesi (kural ID referanslı) + XSD katmanı |
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
  imzasız üretir, imzayı entegratör atar. GİB XSD'si `UBLExtensions`'ı
  zorunlu kıldığı için imzasız belgede şema-geçerli bir placeholder yazılır;
  imzacı bunu XAdES içeriğiyle değiştirir.

## English

Go library for Turkish electronic invoices (GİB e-Fatura / e-Arşiv,
UBL-TR 1.2 — a national customization of OASIS UBL 2.1). Ships the
document layer (build invoices with automatic VAT distribution and totals,
parse official documents, re-emit deterministic prefix-correct XML) and a
validation layer implementing the critical subset of GİB's official
schematron rules — every finding carries the schematron rule ID — plus an
optional XSD check backed by the embedded official schema set (requires
xmllint). Roadmap: XAdES signing (v0.3), SBDH envelopes (v0.4), integrator
transports (v0.5+). Docs are in Turkish on purpose — the domain, its
terminology and its regulator are Turkish.

## License

MIT — see [LICENSE](LICENSE).
