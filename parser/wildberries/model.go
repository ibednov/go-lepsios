package wildberries

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
)

func extractProductInfo(data DetailResponseDTO, normalizedURL string) (title string, price float64, pics int, err error) {
	if len(data.Products) == 0 {
		return "", 0, 0, errors.New("wb detail response has empty products")
	}

	product := data.Products[0]
	title, pics, err = extractProductMetadata(data)
	if err != nil {
		return "", 0, 0, err
	}

	if len(product.Sizes) == 0 {
		return title, 0, pics, errors.New("wb detail response has empty sizes")
	}

	size := pickSize(product.Sizes, normalizedURL)
	// product is the showcase price; logistics is already inside it (BY: 63.49 shown, 6.94 logistics).
	price = float64(size.Price.Product) / 100
	if price <= 0 {
		return title, 0, pics, errors.New("wb detail response has zero price")
	}

	return title, price, pics, nil
}

func extractProductMetadata(data DetailResponseDTO) (title string, pics int, err error) {
	if len(data.Products) == 0 {
		return "", 0, errors.New("wb detail response has empty products")
	}

	product := data.Products[0]
	title = product.Name
	if product.Brand != "" {
		title = fmt.Sprintf("%s - Бренд %s", title, product.Brand)
	}
	if title == "" {
		return "", 0, errors.New("wb detail response has empty title")
	}

	return title, product.Pics, nil
}

func pickSize(sizes []DetailSizeDTO, normalizedURL string) DetailSizeDTO {
	if len(sizes) == 0 {
		return DetailSizeDTO{}
	}
	if optionID := sizeOptionIDFromURL(normalizedURL); optionID > 0 {
		for _, s := range sizes {
			if s.OptionID == optionID {
				return s
			}
		}
	}
	return sizes[0]
}

func sizeOptionIDFromURL(normalizedURL string) int64 {
	u, err := url.Parse(normalizedURL)
	if err != nil {
		return 0
	}
	raw := u.Query().Get("size")
	if raw == "" {
		return 0
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return id
}

type DetailResponseDTO struct {
	Products []DetailProductDTO `json:"products"`
}

type DetailProductDTO struct {
	ID              int              `json:"id"`
	Root            int              `json:"root"`
	KindID          int              `json:"kindId"`
	Brand           string           `json:"brand"`
	BrandID         int              `json:"brandId"`
	SiteBrandID     int              `json:"siteBrandId"`
	Colors          []DetailColorDTO `json:"colors"`
	SubjectID       int              `json:"subjectId"`
	SubjectParentID int              `json:"subjectParentId"`
	Name            string           `json:"name"`
	Entity          string           `json:"entity"`
	MatchID         int              `json:"matchId"`
	Supplier        string           `json:"supplier"`
	SupplierID      int              `json:"supplierId"`
	SupplierRating  float64          `json:"supplierRating"`
	SupplierFlags   int              `json:"supplierFlags"`
	Pics            int              `json:"pics"`
	Rating          float64          `json:"rating"`
	ReviewRating    float64          `json:"reviewRating"`
	NMReviewRating  float64          `json:"nmReviewRating"`
	Feedbacks       int              `json:"feedbacks"`
	NMFeedbacks     int              `json:"nmFeedbacks"`
	PanelPromoID    int              `json:"panelPromoId"`
	Volume          float64          `json:"volume"`
	Weight          float64          `json:"weight"`
	ViewFlags       int64            `json:"viewFlags"`
	Promotions      []int            `json:"promotions"`
	Sizes           []DetailSizeDTO  `json:"sizes"`
	TotalQuantity   int              `json:"totalQuantity"`
	Time1           int              `json:"time1"`
	Time2           int              `json:"time2"`
	WH              int              `json:"wh"`
	DType           int64            `json:"dtype"`
	Dist            int              `json:"dist"`
}

type DetailColorDTO struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}

type DetailSizeDTO struct {
	Name           string           `json:"name"`
	OrigName       string           `json:"origName"`
	Rank           int              `json:"rank"`
	OptionID       int64            `json:"optionId"`
	Stocks         []DetailStockDTO `json:"stocks"`
	Time1          int              `json:"time1"`
	Time2          int              `json:"time2"`
	WH             int              `json:"wh"`
	DType          int64            `json:"dtype"`
	Dist           int              `json:"dist"`
	Price          DetailPriceDTO   `json:"price"`
	SaleConditions int64            `json:"saleConditions"`
	Payload        string           `json:"payload"`
}

type DetailStockDTO struct {
	WH       int   `json:"wh"`
	DType    int64 `json:"dtype"`
	Dist     int   `json:"dist"`
	Qty      int   `json:"qty"`
	Priority int   `json:"priority"`
	Time1    int   `json:"time1"`
	Time2    int   `json:"time2"`
}

type DetailPriceDTO struct {
	Basic     int `json:"basic"`
	Product   int `json:"product"`
	Logistics int `json:"logistics"`
	Return    int `json:"return"`
}
