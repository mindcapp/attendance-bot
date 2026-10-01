package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	CommandsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "commands_total",
		Help: "Количество обработанных команд по типам.",
	}, []string{"command"})

	ErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "errors_total",
		Help: "Количество ошибок при обработке по типам.",
	}, []string{"type"})

	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "request_duration_seconds",
		Help:    "Время обработки сообщения или нажатия кнопки.",
		Buckets: prometheus.DefBuckets,
	}, []string{"kind"})

	usersMu sync.Mutex
	users   = map[int64]struct{}{}

	_ = promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "users_seen",
		Help: "Количество уникальных пользователей с момента запуска.",
	}, func() float64 {
		usersMu.Lock()
		defer usersMu.Unlock()
		return float64(len(users))
	})
)

func SeenUser(id int64) {
	usersMu.Lock()
	users[id] = struct{}{}
	usersMu.Unlock()
}
