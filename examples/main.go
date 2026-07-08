// Sifirdan bir SATIS faturasi kurar ve UBL-TR XML'ini stdout'a yazar.
// Kimlikler sentetiktir (checksum-gecerli, gercek mukellefle iliskisiz).
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/YusufDrymz/go-efatura/ubltr"
	"github.com/YusufDrymz/go-efatura/validate"
)

func main() {
	b := ubltr.NewInvoice(
		ubltr.WithProfile(ubltr.ProfileTemelFatura),
		ubltr.WithType(ubltr.TypeSatis),
		ubltr.WithID("ABC2026000000001"),
		ubltr.WithIssueDate(time.Now()),
		ubltr.WithSupplier(ubltr.PartyInfo{
			VKN:       "9990000005",
			Name:      "Örnek Yazılım A.Ş.",
			TaxOffice: "Beşiktaş",
			Address: ubltr.Address{
				StreetName:          "Örnek Cad.",
				BuildingNumber:      "1",
				CitySubdivisionName: "Beşiktaş",
				CityName:            "İstanbul",
				Country:             ubltr.Country{Name: "Türkiye"},
			},
		}),
		ubltr.WithCustomer(ubltr.PartyInfo{
			TCKN:       "99900000074",
			FirstName:  "Ali",
			FamilyName: "Yılmaz",
			Address: ubltr.Address{
				CitySubdivisionName: "Çankaya",
				CityName:            "Ankara",
				Country:             ubltr.Country{Name: "Türkiye"},
			},
		}),
	)

	b.AddLine(ubltr.Line{
		Name:      "Danışmanlık hizmeti",
		Qty:       ubltr.D("2"),
		Unit:      "C62", // adet
		UnitPrice: ubltr.D("1500"),
		VATRate:   ubltr.D("20"),
	})
	b.AddLine(ubltr.Line{
		Name:            "Lisans yenileme",
		Qty:             ubltr.D("1"),
		Unit:            "C62",
		UnitPrice:       ubltr.D("850.50"),
		VATRate:         ubltr.D("20"),
		DiscountPercent: ubltr.D("10"),
	})

	inv, err := b.Build()
	if err != nil {
		log.Fatal(err)
	}

	// GIB is kurallari: bos donmesi beklenir, builder tutarli uretir
	for _, issue := range validate.Invoice(inv) {
		fmt.Fprintln(os.Stderr, issue)
	}

	out, err := inv.XML()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(out))
}
