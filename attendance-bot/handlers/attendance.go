package handlers

import (
	"fmt"
	"log"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/yourusername/attendance-bot/storage"
)

const notFoundText = "Номер не найден, попробуй ещё"

// Коды статусов (используются в callback-данных st:<код>).
const (
	statusCame    = "came"
	statusLate    = "late"
	statusAbsent  = "absent"
	statusExcused = "excused"
	statusDelay   = "delay"
)

var delayOptions = []int{5, 10, 15, 20, 30, 60}

func (h *Handler) numbersKeyboard() tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton
	for _, st := range h.students.All() {
		n := strconv.Itoa(st.Number)
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(n, cbPickPrefix+n))
		if len(row) == studentsPerRow {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⬅️ В меню", cbBackMenu),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func statusKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("✓ Пришёл", cbStatusPrefix+statusCame)),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⏰ Опоздаю", cbStatusPrefix+statusLate)),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("✗ Не приду", cbStatusPrefix+statusAbsent)),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("📋 По уважительной", cbStatusPrefix+statusExcused)),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⏳ Задержусь", cbStatusPrefix+statusDelay)),
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", cbCancel)),
	)
}

func delayKeyboard() tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton
	for _, m := range delayOptions {
		label := strconv.Itoa(m)
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, cbDelayPrefix+label))
		if len(row) == studentsPerRow {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", cbCancel),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func paraKeyboard() tgbotapi.InlineKeyboardMarkup {
	var row []tgbotapi.InlineKeyboardButton
	for n := 1; n <= 5; n++ {
		label := strconv.Itoa(n)
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, cbParaPrefix+label))
	}
	return tgbotapi.NewInlineKeyboardMarkup(
		row,
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", cbCancel)),
	)
}

func (h *Handler) handlePick(cq *tgbotapi.CallbackQuery, raw string) {
	chatID := cq.Message.Chat.ID
	msgID := cq.Message.MessageID

	n, err := strconv.Atoi(raw)
	if err != nil {
		h.answer(cq.ID, notFoundText)
		return
	}
	st, ok := h.students.Get(n)
	if !ok {
		h.answer(cq.ID, notFoundText)
		h.edit(chatID, msgID, notFoundText, h.numbersKeyboard())
		return
	}

	h.setSession(chatID, cq.From.ID, &session{Student: st})
	h.answer(cq.ID, "")
	text := fmt.Sprintf("%s (№%d)\nВыбери статус:", st.Name, st.Number)
	h.edit(chatID, msgID, text, statusKeyboard())
}

func (h *Handler) handleStatus(cq *tgbotapi.CallbackQuery, code string) {
	chatID := cq.Message.Chat.ID
	msgID := cq.Message.MessageID
	h.answer(cq.ID, "")

	s := h.getSession(chatID, cq.From.ID)
	if s == nil {
		h.edit(chatID, msgID, "Сессия истекла, начни заново", h.numbersKeyboard())
		return
	}

	switch code {
	case statusCame:
		s.StatusLabel = "Пришёл"
		h.finalize(cq, s, 0)
	case statusAbsent:
		s.StatusLabel = "Не приду"
		h.finalize(cq, s, 0)
	case statusExcused:
		s.StatusLabel = "По уважительной"
		h.finalize(cq, s, 0)
	case statusLate:
		s.StatusLabel = "Опоздаю"
		text := fmt.Sprintf("%s (№%d)\nКо скольки паре?", s.Student.Name, s.Student.Number)
		h.edit(chatID, msgID, text, paraKeyboard())
	case statusDelay:
		text := fmt.Sprintf("%s (№%d)\nНа сколько минут?", s.Student.Name, s.Student.Number)
		h.edit(chatID, msgID, text, delayKeyboard())
	default:
		h.edit(chatID, msgID, notFoundText, statusKeyboard())
	}
}

func (h *Handler) handleDelay(cq *tgbotapi.CallbackQuery, raw string) {
	chatID := cq.Message.Chat.ID
	msgID := cq.Message.MessageID
	h.answer(cq.ID, "")

	s := h.getSession(chatID, cq.From.ID)
	if s == nil {
		h.edit(chatID, msgID, "Сессия истекла, начни заново", h.numbersKeyboard())
		return
	}
	m, err := strconv.Atoi(raw)
	if err != nil {
		return
	}
	s.DelayMinutes = m
	s.StatusLabel = fmt.Sprintf("Задержался на %d минут", m)

	text := fmt.Sprintf("%s (№%d)\nКо скольки паре?", s.Student.Name, s.Student.Number)
	h.edit(chatID, msgID, text, paraKeyboard())
}

func (h *Handler) handlePara(cq *tgbotapi.CallbackQuery, raw string) {
	chatID := cq.Message.Chat.ID
	msgID := cq.Message.MessageID
	h.answer(cq.ID, "")

	s := h.getSession(chatID, cq.From.ID)
	if s == nil {
		h.edit(chatID, msgID, "Сессия истекла, начни заново", h.numbersKeyboard())
		return
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return
	}
	h.finalize(cq, s, n)
}

// finalize сохраняет явку и выводит итоговое сообщение. para == 0 значит "без пары".
func (h *Handler) finalize(cq *tgbotapi.CallbackQuery, s *session, para int) {
	chatID := cq.Message.Chat.ID
	msgID := cq.Message.MessageID
	now := time.Now().In(h.loc)

	paraStr := "-"
	if para > 0 {
		paraStr = strconv.Itoa(para)
	}

	rec := storage.Record{
		Date:         now.Format("2006-01-02"),
		Time:         now.Format("15:04"),
		Number:       s.Student.Number,
		Name:         s.Student.Name,
		Status:       s.StatusLabel,
		Para:         paraStr,
		DelayMinutes: s.DelayMinutes,
	}

	if err := h.attendance.Save(rec); err != nil {
		log.Printf("ошибка записи явки №%d: %v", s.Student.Number, err)
		h.edit(chatID, msgID, "Не удалось сохранить, попробуй ещё раз", statusKeyboard())
		return
	}
	h.clearSession(chatID, cq.From.ID)

	text := fmt.Sprintf("✓ Статус записан: %s%s [%s]", s.StatusLabel, paraPhrase(para), rec.Time)
	h.edit(chatID, msgID, text, tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⬅️ В меню", cbBackMenu)),
	))
	log.Printf("отмечен(а): №%d %s — %s%s [%s]", rec.Number, rec.Name, s.StatusLabel, paraPhrase(para), rec.Time)
}

// paraPhrase возвращает " к 1 паре" / " ко 2 паре" и т.д., или "" если пара не указана.
func paraPhrase(n int) string {
	if n <= 0 {
		return ""
	}
	prep := "к"
	if n == 2 {
		prep = "ко"
	}
	return fmt.Sprintf(" %s %d паре", prep, n)
}
