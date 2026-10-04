package az_cbar

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ibednov/go-lepsios/currency"
)

type xmlValCurs struct {
	ValTypes []xmlValType `xml:"ValType"`
}

type xmlValType struct {
	Type    string      `xml:"Type,attr"`
	Valutes []xmlValute `xml:"Valute"`
}

type xmlValute struct {
	Code    string `xml:"Code,attr"`
	Nominal string `xml:"Nominal"`
	Value   string `xml:"Value"`
	valType string // filled while parsing
}

func parseXMLValutes(data []byte) ([]xmlValute, error) {
	var root xmlValCurs
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("cbar: xml unmarshal: %w", err)
	}
	out := make([]xmlValute, 0)
	for _, vt := range root.ValTypes {
		for _, v := range vt.Valutes {
			v.valType = vt.Type
			out = append(out, v)
		}
	}
	return out, nil
}

func parseNominal(s string) (int, error) {
	parts := strings.Fields(strings.TrimSpace(s))
	if len(parts) == 0 {
		return 0, fmt.Errorf("empty nominal")
	}
	return strconv.Atoi(parts[0])
}

func ratesFromXML(items []xmlValute, rateDate time.Time) []currency.OfficialRate {
	out := make([]currency.OfficialRate, 0, len(items)+1)
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.valType), "metal") {
			continue
		}
		code, err := currency.Parse(strings.TrimSpace(item.Code))
		if err != nil || !code.IsValid() {
			continue
		}
		scale, err := parseNominal(item.Nominal)
		if err != nil || scale <= 0 {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(item.Value), 64)
		if err != nil || value <= 0 {
			continue
		}
		out = append(out, currency.OfficialRate{
			Code:        code,
			Scale:       scale,
			BasePerUnit: value,
			Date:        rateDate,
		})
	}
	out = append(out, currency.OfficialRate{
		Code:        currency.AZN,
		Scale:       1,
		BasePerUnit: 1,
		Date:        rateDate,
	})
	return out
}
