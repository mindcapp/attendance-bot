package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golang.org/x/net/proxy"

	"github.com/yourusername/attendance-bot/config"
	"github.com/yourusername/attendance-bot/handlers"
	"github.com/yourusername/attendance-bot/metrics"
	"github.com/yourusername/attendance-bot/storage"
)

var proxyAddrs = []string{
	"127.0.0.1:1080",
	"127.0.0.1:1081",
	"127.0.0.1:7890",
	"127.0.0.1:55009",
}

func newProxyClient() (*http.Client, string, error) {
	for _, addr := range proxyAddrs {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err != nil {
			log.Printf("Прокси %s недоступен: %v", addr, err)
			continue
		}
		conn.Close()

		dialer, err := proxy.SOCKS5("tcp", addr, nil, proxy.Direct)
		if err != nil {
			log.Printf("Не удалось создать SOCKS5 для %s: %v", addr, err)
			continue
		}
		contextDialer, ok := dialer.(proxy.ContextDialer)
		if !ok {
			log.Printf("SOCKS5 dialer для %s не поддерживает контекст", addr)
			continue
		}

		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				return contextDialer.DialContext(ctx, network, address)
			},
		}
		return &http.Client{Transport: transport, Timeout: 150 * time.Second}, addr, nil
	}
	return nil, "", fmt.Errorf("ни один SOCKS5 прокси не доступен: %v", proxyAddrs)
}

const logFile = "logs/bot.log"

// filterWriter скрывает "EOF"/"unexpected EOF" от getUpdates (обычный разрыв
// long-poll) и считает остальные ошибки получения обновлений.
type filterWriter struct {
	mu       sync.Mutex
	out      io.Writer
	skipNext bool
}

func (f *filterWriter) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch {
	case bytes.Contains(p, []byte("EOF")):
		f.skipNext = true
		return len(p), nil
	case bytes.Contains(p, []byte("Failed to get updates")):
		if f.skipNext {
			f.skipNext = false
			return len(p), nil
		}
		metrics.ErrorsCount.Add(1)
	}
	return f.out.Write(p)
}

func setupLogging() (*os.File, error) {
	if err := os.MkdirAll("logs", 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	log.SetOutput(&filterWriter{out: io.MultiWriter(os.Stderr, f)})
	return f, nil
}

func formatUptime(d time.Duration) string {
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

func main() {
	lf, err := setupLogging()
	if err != nil {
		log.Fatalf("Не удалось открыть %s: %v", logFile, err)
	}
	defer lf.Close()
	start := time.Now()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Ошибка конфигурации: %v", err)
	}

	students, err := storage.LoadStudents(cfg.StudentsFile)
	if err != nil {
		log.Fatalf("Ошибка загрузки студентов: %v", err)
	}

	// NewBotAPI calls getMe, so an invalid token fails here.
	client, proxyAddr, err := newProxyClient()
	if err != nil {
		log.Fatalf("Ошибка прокси: %v", err)
	}
	log.Printf("Используется SOCKS5 прокси %s", proxyAddr)

	bot, err := tgbotapi.NewBotAPIWithClient(cfg.Token, tgbotapi.APIEndpoint, client)
	if err != nil {
		log.Fatalf("Не удалось подключиться к Telegram — проверь TELEGRAM_BOT_TOKEN: %v", err)
	}
	log.Printf("Бот @%s запущен, студентов: %d, явки пишутся в %s",
		bot.Self.UserName, len(students.All()), cfg.AttendanceFile)

	h := handlers.New(bot, students, storage.NewAttendance(cfg.AttendanceFile))

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 120
	updates := bot.GetUpdatesChan(u)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	metricsSrv := &http.Server{Addr: ":9090", Handler: metrics.Handler()}
	go func() {
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Метрики недоступны на :9090: %v", err)
		}
	}()

	health := time.NewTicker(5 * time.Minute)
	defer health.Stop()

	for {
		select {
		case <-stop:
			log.Println("Остановка бота...")
			bot.StopReceivingUpdates()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			metricsSrv.Shutdown(ctx)
			cancel()
			log.Printf("Бот остановлен, uptime: %s", formatUptime(time.Since(start)))
			return
		case <-health.C:
			log.Printf("Bot is running, uptime: %s", formatUptime(time.Since(start)))
		case update := <-updates:
			switch {
			case update.CallbackQuery != nil:
				metrics.SeenUser(update.CallbackQuery.From.ID)
			case update.Message != nil && update.Message.From != nil:
				metrics.SeenUser(update.Message.From.ID)
			}
			switch {
			case update.CallbackQuery != nil:
				h.HandleCallback(update.CallbackQuery)
			case update.Message != nil:
				h.HandleMessage(update.Message)
			}
		}
	}
}
