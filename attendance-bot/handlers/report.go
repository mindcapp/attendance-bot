package handlers

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/mindcapp/attendance-bot/excel"
	"github.com/mindcapp/attendance-bot/metrics"
	"github.com/mindcapp/attendance-bot/storage"
)

func reportMenuKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Таблица в чате", cbReportTable),
			tgbotapi.NewInlineKeyboardButtonData("📁 Скачать Excel", cbReportExcel),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ В меню", cbBackMenu),
		),
	)
}

func (h *Handler) handleReportTable(cq *tgbotapi.CallbackQuery) {
	metrics.CommandsTotal.WithLabelValues("report_table").Inc()
	chatID := cq.Message.Chat.ID
	msgID := cq.Message.MessageID
	h.answer(cq.ID, "")

	records, err := h.attendance.Today(time.Now().In(h.loc))
	if err != nil {
		metrics.ErrorsTotal.WithLabelValues("read_attendance").Inc()
		log.Printf("ошибка чтения явок: %v", err)
		h.edit(chatID, msgID, "Не удалось прочитать явки", reportMenuKeyboard())
		return
	}
	if len(records) == 0 {
		h.edit(chatID, msgID, "Явок за сегодня пока нет.", reportMenuKeyboard())
		return
	}

	text := "```\n" + formatTable(records) + "\n```"
	msg := tgbotapi.NewEditMessageText(chatID, msgID, text)
	msg.ParseMode = "Markdown"
	kb := reportMenuKeyboard()
	msg.ReplyMarkup = &kb
	h.send(msg)
}

func (h *Handler) handleReportExcel(cq *tgbotapi.CallbackQuery) {
	metrics.CommandsTotal.WithLabelValues("report_excel").Inc()
	chatID := cq.Message.Chat.ID
	h.answer(cq.ID, "")

	records, err := h.attendance.Today(time.Now().In(h.loc))
	if err != nil {
		metrics.ErrorsTotal.WithLabelValues("read_attendance").Inc()
		log.Printf("ошибка чтения явок: %v", err)
		h.send(tgbotapi.NewMessage(chatID, "Ошибка при создании файла"))
		return
	}
	if len(records) == 0 {
		h.send(tgbotapi.NewMessage(chatID, "Явок за сегодня пока нет."))
		return
	}

	path, err := excel.Build(records)
	if err != nil {
		metrics.ErrorsTotal.WithLabelValues("excel_build").Inc()
		log.Printf("ошибка генерации excel: %v", err)
		h.send(tgbotapi.NewMessage(chatID, "Ошибка при создании файла"))
		return
	}
	defer os.Remove(path)

	doc := tgbotapi.NewDocument(chatID, tgbotapi.FilePath(path))
	doc.Caption = fmt.Sprintf("Явки на %s", time.Now().In(h.loc).Format("02.01.2006"))
	if _, err := h.bot.Send(doc); err != nil {
		metrics.ErrorsTotal.WithLabelValues("excel_send").Inc()
		log.Printf("ошибка отправки excel: %v", err)
		h.send(tgbotapi.NewMessage(chatID, "Ошибка при создании файла"))
	}
}

// formatTable рисует моноширинную таблицу для отправки в чат.
func formatTable(records []storage.Record) string {
	sorted := make([]storage.Record, len(records))
	copy(sorted, records)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Number < sorted[j].Number })

	headers := []string{"№", "ФИ", "Статус", "Время", "Пара"}
	rows := make([][]string, 0, len(sorted))
	for _, r := range sorted {
		rows = append(rows, []string{
			strconv.Itoa(r.Number),
			r.Name,
			shortStatus(r),
			r.Time,
			r.Para,
		})
	}

	widths := make([]int, len(headers))
	for i, hd := range headers {
		widths[i] = utf8.RuneCountInString(hd)
	}
	for _, row := range rows {
		for i, cell := range row {
			if n := utf8.RuneCountInString(cell); n > widths[i] {
				widths[i] = n
			}
		}
	}

	var b strings.Builder
	writeRow := func(cells []string) {
		for i, cell := range cells {
			b.WriteString(padRight(cell, widths[i]))
			if i != len(cells)-1 {
				b.WriteString(" | ")
			}
		}
		b.WriteString("\n")
	}
	writeRow(headers)
	sep := make([]string, len(widths))
	for i, w := range widths {
		sep[i] = strings.Repeat("-", w)
	}
	writeRow(sep)
	for _, row := range rows {
		writeRow(row)
	}
	return strings.TrimRight(b.String(), "\n")
}

func padRight(s string, w int) string {
	n := utf8.RuneCountInString(s)
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}

// shortStatus сокращает "Задержался на N минут" до "Задержусь на Nм" для таблицы.
func shortStatus(r storage.Record) string {
	if strings.HasPrefix(r.Status, "Задержался") && r.DelayMinutes > 0 {
		return fmt.Sprintf("Задержусь на %dм", r.DelayMinutes)
	}
	return r.Status
}
