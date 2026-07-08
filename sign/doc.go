// Package sign, UBL-TR belgelerini GIB'in bekledigi XAdES-BES yapisiyla
// imzalar ve imzali belgeleri dogrular.
//
// Imza serialize edilmis XML uzerinde uretilir (nesne modeli degil): once
// ubltr ile belgeyi kur ve XML() ile serialize et, sonra Sign'a ver. Sign,
// ExtensionContent icindeki placeholder'i gercek ds:Signature ile degistirir.
// Yapi, GIB'in resmi imzali ornekleriyle birebir ayni iskelettedir:
// SignedInfo (C14N 1.0 + rsa-sha256), URI="" enveloped referansi (schematron
// TransformCountCheck geregi tek transform), #SignedProperties referansi ve
// SigningTime + SigningCertificate tasiyan xades:QualifyingProperties.
//
// Verify, imzanin matematiksel gecerliligini (digest'ler + RSA) ve GIB'in
// yapisal kurallarini kontrol eder; sertifika ZINCIRINI dogrulamaz — mali
// muhur kokune (Kamu SM) guven karari cagirana aittir, VerifyResult icindeki
// sertifikayla verilir.
//
// Gercek mali muhur HSM'de yasar; bu paket dosya tabanli anahtarlarla
// (PEM veya Kamu SM test sistemindeki gibi PKCS#12/PFX) calisir. Signer
// crypto.Signer kabul ettigi icin HSM/PKCS#11 implementasyonu sonradan
// takilabilir. Bu katman GIB'in imza dogrulayicisina karsi test EDILMEMISTIR;
// canli kullanim oncesi kendi muhrunuzle smoke test sarttir.
package sign
