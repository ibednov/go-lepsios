package currency

import "time"

// OfficialRate — официальный курс: BasePerUnit единиц базовой валюты
// (BYN для NBRB, AZN для CBAR, …) за Scale единиц валюты Code.
type OfficialRate struct {
	Code        Code
	Scale       int
	BasePerUnit float64
	Date        time.Time
}
