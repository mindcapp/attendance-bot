package handlers

import (
	"log"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/yourusername/attendance-bot/storage"
)

const studentsPerRow = 3

// Префиксы и коды callback-данных.
const (
	cbMenuAttendance = "menu:att"
	cbMenuReport     = "menu:rep"
	cbBackMenu       = "back:menu"
	cbCancel         = "cancel"

	cbPickPrefix   = "pick:"
	cbStatusPrefix = "st:"
	cbDelayPrefix  = "dl:"
	cbParaPrefix   = "pa:"

	cbReportTable = "rep:table"
	cbReportExcel = "rep:excel"
)

type sessionKey struct {
	ChatID int64
	UserID int64
}

// session хранит незавершённый диалог "отметить явку" для одного пользователя.
type session struct {
	Student      storage.Student
	StatusLabel  string
	DelayMinutes int
}

type Handler struct {
	bot        *tgbotapi.BotAPI
	students   *storage.Students
	attendance *storage.Attendance
	loc        *time.Location

	mu       sync.Mutex
	sessions map[sessionKey]*session
}

func New(bot *tgbotapi.BotAPI, students *storage.Students, attendance *storage.Attendance) *Handler {
	return &Handler{
		bot:        bot,
		students:   students,
		attendance: attendance,
		loc:        time.Local,
		sessions:   make(map[sessionKey]*session),
	}
}

// HandleMessage обрабатывает команды /start и /attendance.
func (h *Handler) HandleMessage(msg *tgbotapi.Message) {
	if !msg.IsCommand() {
		return
	}
	switch msg.Command() {
	case "start", "attendance":
		h.clearSession(msg.Chat.ID, msg.From.ID)
		h.sendMainMenu(msg.Chat.ID, "Что нужно сделать?")
	}
}

func (h *Handler) sendMainMenu(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = mainMenuKeyboard()
	h.send(msg)
}

func mainMenuKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Отметить явку", cbMenuAttendance),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Просмотреть отчёт", cbMenuReport),
		),
	)
}

// HandleCallback распределяет нажатия inline-кнопок по нужному сценарию.
func (h *Handler) HandleCallback(cq *tgbotapi.CallbackQuery) {
	if cq.Message == nil {
		h.answer(cq.ID, "")
		return
	}
	chatID := cq.Message.Chat.ID
	msgID := cq.Message.MessageID
	data := cq.Data

	switch {
	case data == cbMenuAttendance:
		h.answer(cq.ID, "")
		h.clearSession(chatID, cq.From.ID)
		h.edit(chatID, msgID, "Выбери свой номер:", h.numbersKeyboard())

	case data == cbMenuReport:
		h.answer(cq.ID, "")
		h.edit(chatID, msgID, "Что показать?", reportMenuKeyboard())

	case data == cbBackMenu:
		h.answer(cq.ID, "")
		h.clearSession(chatID, cq.From.ID)
		h.edit(chatID, msgID, "Что нужно сделать?", mainMenuKeyboard())

	case data == cbCancel:
		h.answer(cq.ID, "")
		h.clearSession(chatID, cq.From.ID)
		h.edit(chatID, msgID, "Выбери свой номер:", h.numbersKeyboard())

	case strings.HasPrefix(data, cbPickPrefix):
		h.handlePick(cq, strings.TrimPrefix(data, cbPickPrefix))

	case strings.HasPrefix(data, cbStatusPrefix):
		h.handleStatus(cq, strings.TrimPrefix(data, cbStatusPrefix))

	case strings.HasPrefix(data, cbDelayPrefix):
		h.handleDelay(cq, strings.TrimPrefix(data, cbDelayPrefix))

	case strings.HasPrefix(data, cbParaPrefix):
		h.handlePara(cq, strings.TrimPrefix(data, cbParaPrefix))

	case data == cbReportTable:
		h.handleReportTable(cq)

	case data == cbReportExcel:
		h.handleReportExcel(cq)

	default:
		h.answer(cq.ID, "")
	}
}

func (h *Handler) getSession(chatID, userID int64) *session {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sessions[sessionKey{chatID, userID}]
}

func (h *Handler) setSession(chatID, userID int64, s *session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessions[sessionKey{chatID, userID}] = s
}

func (h *Handler) clearSession(chatID, userID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.sessions, sessionKey{chatID, userID})
}

func (h *Handler) send(c tgbotapi.Chattable) {
	if _, err := h.bot.Request(c); err != nil {
		log.Printf("ошибка отправки: %v", err)
	}
}

func (h *Handler) edit(chatID int64, msgID int, text string, kb tgbotapi.InlineKeyboardMarkup) {
	h.send(tgbotapi.NewEditMessageTextAndMarkup(chatID, msgID, text, kb))
}

// answer подтверждает нажатие; непустой text показывается всплывающим тостом.
func (h *Handler) answer(id, text string) {
	h.send(tgbotapi.NewCallback(id, text))
}
