// Package validate, UBL-TR faturalara GIB is kurallarini uygular.
//
// Kurallarin kaynagi GIB'in e-Fatura paketiyle dagittigi resmi schematron
// dosyalaridir (UBL-TR_Main_Schematron.xml, UBL-TR_Common_Schematron.xml,
// UBL-TR_Codelist.xml); her kuralin ID'si schematron'daki abstract rule
// ID'sidir ve kod icinde dosya/satir referansi verilir. Schematron'un
// tamamini kapsamak hedef degildir (zarf, e-irsaliye, kullanici islemleri
// disarida); kritik fatura alt kumesi buradadir.
//
// GOEF- onekli kurallar go-efatura'nin ek kurallaridir: GIB schematron'u
// aritmetik tutarliligi hic denetlemez (hicbir toplam-esitlik assert'i
// yoktur), bu kurallar kilavuzdaki toplam formullerini dogrular.
//
// XSD dogrulamasi icin ayrica XSD fonksiyonu vardir; PATH'te xmllint
// gerektirir, sema seti pakete gomuludur.
package validate
