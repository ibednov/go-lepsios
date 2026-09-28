package bot

import (
	"testing"

	tgmodels "github.com/go-telegram/bot/models"
)

func TestGrid(t *testing.T) {
	mk := Grid(2,
		[]tgmodels.InlineKeyboardButton{
			Btn("a", "1"),
			Btn("b", "2"),
			Btn("c", "3"),
		},
		Row(Btn("back", "back")),
	)
	rows := mk.InlineKeyboard
	if len(rows) != 3 {
		t.Fatalf("rows: got %d", len(rows))
	}
	if len(rows[0]) != 2 || rows[0][0].CallbackData != "1" || rows[0][1].CallbackData != "2" {
		t.Fatalf("row0: %+v", rows[0])
	}
	if len(rows[1]) != 1 || rows[1][0].CallbackData != "3" {
		t.Fatalf("row1: %+v", rows[1])
	}
	if rows[2][0].CallbackData != "back" {
		t.Fatalf("footer: %+v", rows[2])
	}
}
