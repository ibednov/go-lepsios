package bot

import (
	tgmodels "github.com/go-telegram/bot/models"
)

// Btn builds an inline keyboard button with callback data.
func Btn(text, data string) tgmodels.InlineKeyboardButton {
	return tgmodels.InlineKeyboardButton{Text: text, CallbackData: data}
}

// Markup builds an inline keyboard from rows of buttons.
func Markup(rows ...[]tgmodels.InlineKeyboardButton) *tgmodels.InlineKeyboardMarkup {
	mk := &tgmodels.InlineKeyboardMarkup{InlineKeyboard: make([][]tgmodels.InlineKeyboardButton, 0, len(rows))}
	mk.InlineKeyboard = append(mk.InlineKeyboard, rows...)
	return mk
}

// Row is a convenience alias for a button row.
func Row(buttons ...tgmodels.InlineKeyboardButton) []tgmodels.InlineKeyboardButton {
	return buttons
}

// Grid lays buttons in cols columns and appends footer rows as-is.
// cols < 1 is treated as 1.
func Grid(cols int, buttons []tgmodels.InlineKeyboardButton, footer ...[]tgmodels.InlineKeyboardButton) *tgmodels.InlineKeyboardMarkup {
	if cols < 1 {
		cols = 1
	}
	rows := make([][]tgmodels.InlineKeyboardButton, 0, len(buttons)/cols+len(footer))
	var row []tgmodels.InlineKeyboardButton
	for _, button := range buttons {
		row = append(row, button)
		if len(row) == cols {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	rows = append(rows, footer...)
	return Markup(rows...)
}
