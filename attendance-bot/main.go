package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/mindcapp/attendance-bot/config"
	"github.com/mindcapp/attendance-bot/handlers"
	"github.com/mindcapp/attendance-bot/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Ошибка конфигурации: %v", err)
	}

	students, err := storage.LoadStudents(cfg.StudentsFile)
	if err != nil {
		log.Fatalf("Ошибка загрузки студентов: %v", err)
	}

	// NewBotAPI calls getMe, so an invalid token fails here.
	bot, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		log.Fatalf("Не удалось подключиться к Telegram — проверь TELEGRAM_BOT_TOKEN: %v", err)
	}
	log.Printf("Бот @%s запущен, студентов: %d, явки пишутся в %s",
		bot.Self.UserName, len(students.All()), cfg.AttendanceFile)

	h := handlers.New(bot, students, storage.NewAttendance(cfg.AttendanceFile))

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(":9090", nil); err != nil {
			log.Printf("Метрики недоступны на :9090: %v", err)
		}
	}()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)
	go startReminderScheduler(bot)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-stop:
			log.Println("Остановка бота")
			bot.StopReceivingUpdates()
			return
		case update := <-updates:
			switch {
			case update.CallbackQuery != nil:
				h.HandleCallback(update.CallbackQuery)
			case update.Message != nil:
				h.HandleMessage(update.Message)
			}
		}
	}
}
