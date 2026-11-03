package router

import (
	"io/fs"
	"net/http"

	"github.com/meshestra/go-playground/jp/internal/src-api/presentation/http/controller"
)

func New(c *controller.Controller) (http.Handler, error) {
	mux := http.NewServeMux()
	staticFiles := controller.StaticFiles()
	mux.Handle("/static/", http.FileServer(http.FS(staticFiles)))
	assets, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, err
	}
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets))))
	mux.HandleFunc("/api/auth/register", c.Register)
	mux.HandleFunc("/api/auth/login", c.Login)
	mux.HandleFunc("/api/auth/me", c.Me)
	mux.HandleFunc("/api/settings/topic", c.TopicSetting)
	mux.HandleFunc("/api/chat", c.ChatMessage)
	mux.HandleFunc("/api/chat/stream", c.ChatStream)
	mux.HandleFunc("/api/chat/history", c.ChatHistory)
	mux.HandleFunc("/api/learning-state", c.LearningState)
	mux.HandleFunc("/api/content/today", c.TodayContent)
	mux.HandleFunc("/api/content/", c.ContentByDate)
	mux.HandleFunc("/", c.Index)
	return securityHeaders(mux), nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'; script-src 'unsafe-inline'")
		next.ServeHTTP(writer, request)
	})
}
