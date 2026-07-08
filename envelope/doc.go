// Package envelope, GIB e-Fatura zarflarini (SBDH — StandardBusinessDocument)
// olusturur ve acar.
//
// Belgeler zarfa HAM BAYT olarak konur ve acilirken ham bayt olarak cikar:
// imzali bir fatura zarflanip acildiginda baytlari degismez, imzasi bozulmaz.
// Bu yuzden zarf govdesi nesne modelinden degil dogrudan metinden kurulur ve
// acma tarafinda belgeler offset tabanli dilimlenir.
//
// Durum kodlari ve sistem yaniti modeli Ek-2 "Sistem Yaniti Sema Yapisi"
// kilavuzundan (v1.5) alinmistir; zarf yapisal kurallari resmi schematron'la
// uyumludur (TypeVersion 1.2, en fazla 100 fatura, UUID zarf ID'si...).
//
// Zarflar GIB'e ZIP olarak gonderilir: zip tam bir dosya icermeli ve dosya
// adi zarf ID'si olmalidir (durum kodlari 1131/1133 bu kurallarin ihlalidir);
// Zip ve OpenZip bu kurali uygular.
package envelope
