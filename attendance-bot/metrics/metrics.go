package metrics

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
)

var (
	TotalCheckins atomic.Int64
	ErrorsCount   atomic.Int64

	usersMu sync.Mutex
	users   = map[int64]struct{}{}
)

func SeenUser(id int64) {
	usersMu.Lock()
	users[id] = struct{}{}
	usersMu.Unlock()
}

func totalUsers() int {
	usersMu.Lock()
	defer usersMu.Unlock()
	return len(users)
}

func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "# TYPE total_users gauge\ntotal_users %d\n", totalUsers())
		fmt.Fprintf(w, "# TYPE total_checkins counter\ntotal_checkins %d\n", TotalCheckins.Load())
		fmt.Fprintf(w, "# TYPE errors_count counter\nerrors_count %d\n", ErrorsCount.Load())
	})
	return mux
}
