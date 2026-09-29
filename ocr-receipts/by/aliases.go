package by

import "strings"

// alias maps a normalized needle → display title.
type alias struct {
	needle string
	title  string
}

// storeAliases: OCR / legal variants → short spend title.
var storeAliases = []alias{
	{needle: "евроопт", title: "Евроопт"},
	{needle: "euroopt", title: "Евроопт"},
	{needle: "европочт", title: "Европочта"},
	{needle: "интернет-магазин евроопт", title: "Евроопт"},
	{needle: "днс электроника", title: "ДНС Электроника"},
	{needle: "дис электроника", title: "ДНС Электроника"},
	{needle: "dns", title: "ДНС Электроника"},
	{needle: "спортмастер", title: "Спортмастер"},
	{needle: "олимп см", title: "Спортмастер"},
	{needle: "мпр ритейл", title: "МПР Ритейл"},
	{needle: "эксмо аст", title: "Эксмо АСТ"},
	{needle: "ветер-логистик", title: "Ветер-Логистик"},
	{needle: "ветер логистик", title: "Ветер-Логистик"},
	{needle: "факт", title: "Факт"},
}

func applyAlias(normalized, raw string) string {
	for _, a := range storeAliases {
		if strings.Contains(normalized, a.needle) {
			return a.title
		}
	}
	return strings.TrimSpace(raw)
}
