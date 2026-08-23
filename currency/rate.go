package currency

import "time"

// OfficialRate — официальный курс: BYNPerUnit белорусских рублей за Scale единиц валюты.
type OfficialRate struct {
	Code       Code
	Scale      int
	BYNPerUnit float64
	Date       time.Time
}
