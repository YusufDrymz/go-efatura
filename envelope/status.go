package envelope

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
)

// Zarf durum kodlari — Ek-2 Sistem Yaniti Sema Yapisi v1.5, s.17-18.
// Aciklamalar kilavuzdaki metinlerin birebir kendisidir.
var statusText = map[int]string{
	1000: "ZARF KUYRUGA EKLENDI",
	1100: "ZARF ISLENIYOR",
	1110: "ZIP DOSYASI DEGIL",
	1111: "ZARF ID UZUNLUGU GECERSIZ",
	1120: "ZARF ARSIVDEN KOPYALANAMADI",
	1130: "ZIP ACILAMADI",
	1131: "ZIP BIR DOSYA ICERMELI",
	1132: "XML DOSYASI DEGIL",
	1133: "ZARF ID VE XML DOSYASININ ADI AYNI OLMALI",
	1140: "DOKUMAN AYRISTIRILAMADI",
	1141: "ZARF ID YOK",
	1142: "ZARF ID VE ZIP DOSYASI ADI AYNI OLMALI",
	1143: "GECERSIZ VERSIYON",
	1150: "SCHEMATRON KONTROL SONUCU HATALI",
	1160: "XML SEMA KONTROLUNDEN GECEMEDI",
	1161: "IMZA SAHIBI TCKN VKN ALINAMADI",
	1162: "IMZA KAYDEDILEMEDI",
	1163: "GONDERILEN ZARF SISTEMDE DAHA ONCE KAYITLI OLAN BIR FATURAYI ICERMEKTEDIR",
	1164: "GONDERILEN ZARF SISTEMDE DAHA ONCE KAYITLI OLAN BIR BELGEYI ICERMEKTEDIR",
	1170: "YETKI KONTROL EDILEMEDI",
	1171: "GONDERICI BIRIM YETKISI YOK",
	1172: "POSTA KUTUSU YETKISI YOK",
	1175: "IMZA YETKISI KONTROL EDILEMEDI",
	1176: "IMZA SAHIBI YETKISIZ",
	1177: "GECERSIZ IMZA",
	1180: "ADRES KONTROL EDILEMEDI",
	1181: "ADRES BULUNAMADI",
	1182: "KULLANICI EKLENEMEDI",
	1183: "KULLANICI SILENEMEDI",
	1190: "SISTEM YANITI HAZIRLANAMADI",
	1195: "SISTEM HATASI",
	1200: "ZARF BASARIYLA ISLENDI",
	1210: "DOKUMAN BULUNAN ADRESE GONDERILEMEDI",
	1215: "DOKUMAN GONDERIMI BASARISIZ. TEKRAR GONDERME SONLANDI",
	1220: "HEDEFTEN SISTEM YANITI GELMEDI",
	1230: "HEDEFTEN SISTEM YANITI BASARISIZ GELDI",
	1235: "FATURA IPTAL'E KONU EDILDI",
	1300: "BASARIYLA TAMAMLANDI",
}

// StatusText durum kodunun kilavuzdaki aciklamasini dondurur.
func StatusText(code int) string {
	if s, ok := statusText[code]; ok {
		return s
	}
	return fmt.Sprintf("BILINMEYEN DURUM KODU (%d)", code)
}

// StatusSucceeded: zarf islendi (1200 merkezde, 1300 uctan uca tamamlandi).
func StatusSucceeded(code int) bool { return code == 1200 || code == 1300 }

// StatusFailed: kalici hata. Kilavuz: islenme hatalari 1100-1200 araligindadir;
// 1215 tekrar gonderim sonlandi, 1230 hedef basarisiz. 1163/1215/1230 sonrasi
// faturalar ayni fatura ID'siyle yeni zarfta gonderilebilir.
func StatusFailed(code int) bool {
	return (code > 1100 && code < 1200) || code == 1215 || code == 1230
}

// StatusPending: islem suruyor, beklemek gerekir (kuyruk, isleme, gonderim
// denemesi, hedeften yanit bekleme).
func StatusPending(code int) bool {
	return code == 1000 || code == 1100 || code == 1210 || code == 1220
}

// Response bir sistem yanitinin ozeti: hangi zarfa hangi durum kodu dondu.
type Response struct {
	ID           string // sistem yaniti numarasi
	UUID         string
	IssueDate    string
	EnvelopeID   string // yanitlanan zarfin ID'si
	EnvelopeType string // SENDERENVELOPE / POSTBOXENVELOPE
	Code         int    // zarf durum kodu (LineResponse icindeki)
	Description  string // belgedeki aciklama metni
}

type appResponse struct {
	ID                string `xml:"ID"`
	UUID              string `xml:"UUID"`
	IssueDate         string `xml:"IssueDate"`
	DocumentResponses []struct {
		DocumentReference struct {
			ID               string `xml:"ID"`
			DocumentTypeCode string `xml:"DocumentTypeCode"`
		} `xml:"DocumentReference"`
		LineResponses []struct {
			Responses []struct {
				ResponseCode string   `xml:"ResponseCode"`
				Descriptions []string `xml:"Description"`
			} `xml:"Response"`
		} `xml:"LineResponse"`
	} `xml:"DocumentResponse"`
}

// ParseResponse sistem yaniti belgesini (ApplicationResponse) ozetler.
// Zarftan cikan APPLICATIONRESPONSE baytlari dogrudan verilebilir.
func ParseResponse(doc []byte) (*Response, error) {
	var ar appResponse
	if err := xml.NewDecoder(bytes.NewReader(doc)).Decode(&ar); err != nil {
		return nil, fmt.Errorf("envelope: sistem yaniti parse edilemedi: %w", err)
	}
	r := &Response{ID: ar.ID, UUID: ar.UUID, IssueDate: ar.IssueDate}
	if len(ar.DocumentResponses) == 0 {
		return nil, fmt.Errorf("envelope: DocumentResponse yok")
	}
	dr := ar.DocumentResponses[0]
	r.EnvelopeID = dr.DocumentReference.ID
	r.EnvelopeType = dr.DocumentReference.DocumentTypeCode
	for _, lr := range dr.LineResponses {
		for _, resp := range lr.Responses {
			if code, err := strconv.Atoi(resp.ResponseCode); err == nil {
				r.Code = code
				if len(resp.Descriptions) > 0 {
					r.Description = resp.Descriptions[0]
				}
				return r, nil
			}
		}
	}
	return r, nil
}
