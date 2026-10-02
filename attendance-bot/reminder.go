package main

import (
	"log"
	"os"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func startReminderScheduler(bot *tgbotapi.BotAPI) {
	chatID, err := strconv.ParseInt(os.Getenv("REMINDER_CHAT_ID"), 10, 64)
	if err != nil {
		log.Println("REMINDER_CHAT_ID не задан, напоминания отключены")
		return
	}

	var lastSent string
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for now := range ticker.C {
		if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
			continue
		}
		if now.Hour() != 7 || now.Minute() != 0 {
			continue
		}
		day := now.Format("2006-01-02")
		if day == lastSent {
			continue
		}
		lastSent = day

		msg := tgbotapi.NewMessage(chatID, "⏰ Напоминание: не забудьте отметить явку в боте")
		if _, err := bot.Send(msg); err != nil {
			log.Printf("ошибка отправки напоминания: %v", err)
		}
	}
}
