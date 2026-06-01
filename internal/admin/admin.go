package admin

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ggwpgoend/devin-proxy/internal/checker"
	"github.com/ggwpgoend/devin-proxy/internal/keypool"
)

type Server struct {
	pool *keypool.Pool
	tmpl *template.Template
}

func NewServer(pool *keypool.Pool) *Server {
	s := &Server{pool: pool}
	s.tmpl = template.Must(template.New("").Funcs(template.FuncMap{
		"statusClass": func(code int) string {
			if code >= 200 && code < 300 {
				return "success"
			}
			if code >= 400 {
				return "error"
			}
			return "warning"
		},
		"stateClass": func(state string) string {
			switch state {
			case "active":
				return "state-active"
			case "cooldown":
				return "state-cooldown"
			case "disabled":
				return "state-disabled"
			case "revoked":
				return "state-revoked"
			default:
				return ""
			}
		},
		"truncate": func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "…"
		},
	}).Parse(dashboardHTML))
	return s
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", s.handleDashboard)
	r.Post("/keys", s.handleAddKey)
	r.Post("/keys/bulk", s.handleBulkAdd)
	r.Post("/keys/{id}/disable", s.handleDisable)
	r.Post("/keys/{id}/enable", s.handleEnable)
	r.Delete("/keys/{id}", s.handleDelete)
	r.Get("/api/stats", s.handleAPIStats)
	r.Get("/api/keys", s.handleAPIKeys)
	r.Get("/api/logs", s.handleAPILogs)
	r.Post("/keys/{id}/check", s.handleCheckKey)
	r.Post("/keys/check-all", s.handleCheckAll)

	return r
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats, _ := s.pool.GetStats(ctx)
	keys, _ := s.pool.ListKeys(ctx)
	recent, _ := s.pool.RecentRequests(ctx, 50)

	data := map[string]any{
		"Stats":   stats,
		"Keys":    keys,
		"Logs":    recent,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "dashboard", data); err != nil {
		log.Printf("[admin] template error: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) handleAddKey(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	apiKey := strings.TrimSpace(r.FormValue("api_key"))
	label := strings.TrimSpace(r.FormValue("label"))
	planType := strings.TrimSpace(r.FormValue("plan_type"))
	if apiKey == "" {
		http.Error(w, "api_key is required", http.StatusBadRequest)
		return
	}
	id, err := s.pool.AddKey(r.Context(), apiKey, label, planType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("[admin] added key %s", id[:8])
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleBulkAdd(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	raw := strings.TrimSpace(r.FormValue("keys"))
	planType := strings.TrimSpace(r.FormValue("plan_type"))
	lines := strings.FieldsFunc(raw, func(c rune) bool { return c == '\n' || c == ',' })
	added, err := s.pool.BulkAdd(r.Context(), lines, planType)
	if err != nil {
		http.Error(w, fmt.Sprintf("added %d, error: %v", added, err), http.StatusInternalServerError)
		return
	}
	log.Printf("[admin] bulk added %d keys", added)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleDisable(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.pool.DisableKey(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleEnable(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.pool.EnableKey(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.pool.RemoveKey(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAPIStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.pool.GetStats(r.Context())
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, stats)
}

func (s *Server) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.pool.ListKeys(r.Context())
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, keys)
}

func (s *Server) handleAPILogs(w http.ResponseWriter, r *http.Request) {
	logs, err := s.pool.RecentRequests(r.Context(), 100)
	if err != nil {
		jsonErr(w, err)
		return
	}
	jsonOK(w, logs)
}

func (s *Server) handleCheckKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	apiKey, err := s.pool.DecryptKeyByID(ctx, id)
	if err != nil {
		jsonErr(w, err)
		return
	}

	result := checker.Check(ctx, apiKey)
	log.Printf("[checker] key %s: %s (HTTP %d, %dms)", id[:8], result.Status, result.HTTPStatus, result.Latency)

	switch result.Status {
	case checker.StatusValid:
		_ = s.pool.UpdateKeyState(ctx, id, "active", "")
	case checker.StatusUnauthorized:
		_ = s.pool.UpdateKeyState(ctx, id, "revoked", result.Error)
	case checker.StatusQuotaExhausted:
		_ = s.pool.UpdateKeyState(ctx, id, "cooldown", result.Error)
	case checker.StatusRateLimited:
		_ = s.pool.UpdateKeyState(ctx, id, "cooldown", result.Error)
	}

	jsonOK(w, result)
}

func (s *Server) handleCheckAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	keys, err := s.pool.ListKeys(ctx)
	if err != nil {
		jsonErr(w, err)
		return
	}

	type keyResult struct {
		ID     string         `json:"id"`
		Label  string         `json:"label"`
		Result checker.Result `json:"result"`
	}

	results := make([]keyResult, len(keys))
	var wg sync.WaitGroup
	for i, k := range keys {
		wg.Add(1)
		go func(idx int, key keypool.Key) {
			defer wg.Done()
			apiKey, err := s.pool.DecryptKeyByID(ctx, key.ID)
			if err != nil {
				results[idx] = keyResult{ID: key.ID, Label: key.Label, Result: checker.Result{Status: checker.StatusNetworkError, Error: err.Error()}}
				return
			}
			res := checker.Check(ctx, apiKey)
			results[idx] = keyResult{ID: key.ID, Label: key.Label, Result: res}

			switch res.Status {
			case checker.StatusValid:
				_ = s.pool.UpdateKeyState(ctx, key.ID, "active", "")
			case checker.StatusUnauthorized:
				_ = s.pool.UpdateKeyState(ctx, key.ID, "revoked", res.Error)
			case checker.StatusQuotaExhausted:
				_ = s.pool.UpdateKeyState(ctx, key.ID, "cooldown", res.Error)
			case checker.StatusRateLimited:
				_ = s.pool.UpdateKeyState(ctx, key.ID, "cooldown", res.Error)
			}
		}(i, k)
	}
	wg.Wait()

	log.Printf("[checker] checked %d keys", len(results))
	jsonOK(w, results)
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
