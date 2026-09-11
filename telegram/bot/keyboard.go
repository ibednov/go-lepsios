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
