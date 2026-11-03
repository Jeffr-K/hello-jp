package api

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/meshestra/go-playground/jp/internal/auth"
	"github.com/meshestra/go-playground/jp/internal/content"
	"github.com/meshestra/go-playground/jp/internal/llm"
	"github.com/meshestra/go-playground/jp/internal/memory"
)

//go:embed static
var staticFiles embed.FS

var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

type Server struct {
	server     *http.Server
	contentDir string
	page       *template.Template
	chat       ChatRunner
	auth       *auth.Service
	generator  UserContentGenerator
}

type ChatRunner interface {
	Run(context.Context, string, string, string) (llm.Message, error)
}

type ChatHistoryRunner interface {
	History(context.Context, string, string, int) ([]llm.Message, error)
}

type LearningStateRunner interface {
	LearningState(context.Context, string, string) (memory.LearningState, error)
	SaveLearningState(context.Context, string, memory.LearningState) error
}

type UserContentGenerator interface {
	RunForUser(context.Context, string, string) (*content.DailyContent, error)
}

func NewServer(addr, contentDir string, chat ...ChatRunner) (*Server, error) {
	var runner ChatRunner
	if len(chat) > 0 {
		runner = chat[0]
	}
	return newServer(addr, contentDir, runner, nil)
}

func NewServerWithAuth(addr, contentDir string, chat ChatRunner, authService *auth.Service) (*Server, error) {
	return newServer(addr, contentDir, chat, authService)
}

func NewServerWithAuthAndGenerator(addr, contentDir string, chat ChatRunner, authService *auth.Service, generator UserContentGenerator) (*Server, error) {
	server, err := newServer(addr, contentDir, chat, authService)
	if err != nil {
		return nil, err
	}
	server.generator = generator
	return server, nil
}

func newServer(addr, contentDir string, runner ChatRunner, authService *auth.Service) (*Server, error) {
	if strings.TrimSpace(addr) == "" {
		return nil, fmt.Errorf("web address is required")
	}
	if strings.TrimSpace(contentDir) == "" {
		return nil, fmt.Errorf("content directory is required")
	}
	page, err := template.New("index.html").ParseFS(staticFiles, "static/index.html")
	if err != nil {
		return nil, fmt.Errorf("parse web template: %w", err)
	}

	server := &Server{contentDir: contentDir, page: page, chat: runner, auth: authService}
	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServer(http.FS(staticFiles)))
	assetFiles, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, fmt.Errorf("prepare web assets: %w", err)
	}
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetFiles))))
	mux.HandleFunc("/api/auth/register", server.register)
	mux.HandleFunc("/api/auth/login", server.login)
	mux.HandleFunc("/api/auth/me", server.me)
	mux.HandleFunc("/api/settings/topic", server.topicSetting)
	mux.HandleFunc("/api/chat", server.chatMessage)
	mux.HandleFunc("/api/chat/stream", server.chatStream)
	mux.HandleFunc("/api/chat/history", server.chatHistory)
	mux.HandleFunc("/api/learning-state", server.learningState)
	mux.HandleFunc("/api/content/today", server.todayContent)
	mux.HandleFunc("/api/content/", server.contentByDate)
	mux.HandleFunc("/", server.index)

	server.server = &http.Server{
		Addr:              addr,
		Handler:           withSecurityHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	return server, nil
}

func (s *Server) Run(ctx context.Context) error {
	serverError := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverError <- err
		}
		close(serverError)
	}()

	select {
	case err := <-serverError:
		return err
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.server.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("shutdown web server: %w", err)
		}

		return nil
	}
}

func (s *Server) index(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" && request.URL.Path != "/today" {
		http.NotFound(writer, request)
		return
	}

	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	page := struct {
		Content *content.DailyContent
		Error   string
		Today   bool
	}{}
	if request.URL.Path == "/today" {
		page.Today = true
		daily, err := s.loadContent(time.Now().Format("2006-01-02"))
		if err != nil {
			page.Error = "오늘의 콘텐츠가 없습니다. 먼저 just generate를 실행하세요."
		} else {
			page.Content = daily
		}
	}
	if err := s.page.Execute(writer, page); err != nil {
		http.Error(writer, "web template unavailable", http.StatusInternalServerError)
	}
}

func (s *Server) register(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.auth == nil {
		http.Error(writer, "authentication unavailable", http.StatusServiceUnavailable)
		return
	}
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 16<<10)).Decode(&input); err != nil {
		http.Error(writer, "invalid request", http.StatusBadRequest)
		return
	}
	user, token, err := s.auth.Register(request.Context(), input.Email, input.Password, input.Name)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, auth.ErrEmailExists) {
			status = http.StatusConflict
		}
		http.Error(writer, err.Error(), status)
		return
	}
	s.writeAuthResponse(writer, user, token)
}

func (s *Server) login(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.auth == nil {
		http.Error(writer, "authentication unavailable", http.StatusServiceUnavailable)
		return
	}
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 16<<10)).Decode(&input); err != nil {
		http.Error(writer, "invalid request", http.StatusBadRequest)
		return
	}
	user, token, err := s.auth.Login(request.Context(), input.Email, input.Password)
	if err != nil {
		http.Error(writer, "invalid credentials", http.StatusUnauthorized)
		return
	}
	s.writeAuthResponse(writer, user, token)
}

func (s *Server) writeAuthResponse(writer http.ResponseWriter, user auth.User, token string) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]any{"token": token, "user": user})
}

func (s *Server) me(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID, ok := s.requireUser(writer, request)
	if !ok {
		return
	}
	user, err := s.auth.User(request.Context(), userID)
	if err != nil {
		http.Error(writer, "user not found", http.StatusNotFound)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(user)
}

func (s *Server) topicSetting(writer http.ResponseWriter, request *http.Request) {
	userID, ok := s.requireUser(writer, request)
	if !ok {
		return
	}
	switch request.Method {
	case http.MethodGet:
		date := request.URL.Query().Get("date")
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}
		setting, err := s.auth.Topic(request.Context(), userID, date)
		if err != nil {
			http.Error(writer, "topic setting unavailable", http.StatusBadGateway)
			return
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(writer).Encode(setting)
	case http.MethodPut:
		var input struct {
			Topic     string `json:"topic"`
			ApplyDate string `json:"apply_date"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 8<<10)).Decode(&input); err != nil {
			http.Error(writer, "invalid request", http.StatusBadRequest)
			return
		}
		if err := s.auth.SaveTopic(request.Context(), auth.TopicSetting{UserID: userID, Topic: input.Topic, ApplyDate: input.ApplyDate}); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		if input.ApplyDate == time.Now().Format("2006-01-02") && s.generator != nil {
			if _, err := s.generator.RunForUser(request.Context(), userID, input.Topic); err != nil {
				http.Error(writer, "generate today's content failed", http.StatusBadGateway)
				return
			}
		}
		writer.WriteHeader(http.StatusNoContent)
	default:
		writer.Header().Set("Allow", "GET, PUT")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) requireUser(writer http.ResponseWriter, request *http.Request) (string, bool) {
	if s.auth == nil {
		http.Error(writer, "authentication unavailable", http.StatusServiceUnavailable)
		return "", false
	}
	userID, err := s.auth.Authenticate(request.Header.Get("Authorization"))
	if err != nil {
		http.Error(writer, "authentication required", http.StatusUnauthorized)
		return "", false
	}
	return userID, true
}

func (s *Server) chatMessage(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.chat == nil {
		http.Error(writer, "chat runtime unavailable", http.StatusServiceUnavailable)
		return
	}
	userID, ok := s.requireUser(writer, request)
	if !ok {
		return
	}

	var input struct {
		UserID    string `json:"user_id"`
		SessionID string `json:"session_id"`
		Input     string `json:"input"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 32<<10)).Decode(&input); err != nil {
		http.Error(writer, "invalid request", http.StatusBadRequest)
		return
	}
	input.UserID = userID
	input.SessionID = strings.TrimSpace(input.SessionID)
	input.Input = strings.TrimSpace(input.Input)
	if input.SessionID == "" {
		input.SessionID = "default"
	}
	if input.Input == "" {
		http.Error(writer, "input is required", http.StatusBadRequest)
		return
	}

	response, err := s.chat.Run(request.Context(), input.UserID, input.SessionID, input.Input)
	if err != nil {
		http.Error(writer, "chat unavailable", http.StatusBadGateway)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]string{"content": response.Content})
}

func (s *Server) chatStream(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	streamer, ok := s.chat.(interface {
		RunStream(context.Context, string, string, string, func(string) error) (llm.Message, error)
	})
	if !ok {
		http.Error(writer, "streaming chat unavailable", http.StatusServiceUnavailable)
		return
	}
	userID, ok := s.requireUser(writer, request)
	if !ok {
		return
	}
	var input struct {
		UserID    string `json:"user_id"`
		SessionID string `json:"session_id"`
		Input     string `json:"input"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 32<<10)).Decode(&input); err != nil {
		http.Error(writer, "invalid request", http.StatusBadRequest)
		return
	}
	input.UserID = userID
	input.SessionID = strings.TrimSpace(input.SessionID)
	input.Input = strings.TrimSpace(input.Input)
	if input.SessionID == "" {
		input.SessionID = "default"
	}
	if input.Input == "" {
		http.Error(writer, "input is required", http.StatusBadRequest)
		return
	}
	flusher, ok := writer.(http.Flusher)
	if !ok {
		http.Error(writer, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	startedAt := time.Now()
	send := func(event string, payload any) error {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event, data); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	_, err := streamer.RunStream(request.Context(), input.UserID, input.SessionID, input.Input, func(delta string) error {
		return send("delta", map[string]string{"content": delta})
	})
	if err != nil {
		_ = send("error", map[string]string{"message": "chat unavailable"})
		return
	}
	_ = send("done", map[string]any{"ok": true, "elapsed_ms": time.Since(startedAt).Milliseconds()})
}

func (s *Server) chatHistory(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	historyRunner, ok := s.chat.(ChatHistoryRunner)
	if !ok {
		http.Error(writer, "chat history unavailable", http.StatusServiceUnavailable)
		return
	}
	userID, ok := s.requireUser(writer, request)
	if !ok {
		return
	}
	sessionID := strings.TrimSpace(request.URL.Query().Get("session_id"))
	if sessionID == "" {
		http.Error(writer, "session_id is required", http.StatusBadRequest)
		return
	}
	history, err := historyRunner.History(request.Context(), userID, sessionID, 100)
	if err != nil {
		http.Error(writer, "chat history unavailable", http.StatusBadGateway)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(history)
}

func (s *Server) learningState(writer http.ResponseWriter, request *http.Request) {
	userID, ok := s.requireUser(writer, request)
	if !ok {
		return
	}
	runner, ok := s.chat.(LearningStateRunner)
	if !ok {
		http.Error(writer, "learning state unavailable", http.StatusServiceUnavailable)
		return
	}
	lessonID := strings.TrimSpace(request.URL.Query().Get("lesson_id"))
	if lessonID == "" {
		http.Error(writer, "lesson_id is required", http.StatusBadRequest)
		return
	}
	switch request.Method {
	case http.MethodGet:
		state, err := runner.LearningState(request.Context(), userID, lessonID)
		if err != nil {
			http.Error(writer, "learning state unavailable", http.StatusBadGateway)
			return
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(writer).Encode(state)
	case http.MethodPut:
		var state memory.LearningState
		if err := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 16<<10)).Decode(&state); err != nil {
			http.Error(writer, "invalid request", http.StatusBadRequest)
			return
		}
		state.LessonID = lessonID
		if err := runner.SaveLearningState(request.Context(), userID, state); err != nil {
			http.Error(writer, "save learning state failed", http.StatusBadGateway)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	default:
		writer.Header().Set("Allow", "GET, PUT")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) todayContent(writer http.ResponseWriter, request *http.Request) {
	if s.auth != nil {
		if _, ok := s.requireUser(writer, request); !ok {
			return
		}
	}
	s.content(writer, request, time.Now().Format("2006-01-02"))
}

func (s *Server) contentByDate(writer http.ResponseWriter, request *http.Request) {
	if s.auth != nil {
		if _, ok := s.requireUser(writer, request); !ok {
			return
		}
	}
	date := strings.TrimPrefix(request.URL.Path, "/api/content/")
	if !datePattern.MatchString(date) {
		http.Error(writer, "invalid date", http.StatusBadRequest)
		return
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		http.Error(writer, "invalid date", http.StatusBadRequest)
		return
	}

	s.content(writer, request, date)
}

func (s *Server) content(writer http.ResponseWriter, request *http.Request, date string) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := filepath.Join(s.contentDir, date+".json")
	if s.auth != nil {
		if userID, ok := s.requireUser(writer, request); ok {
			userPath := filepath.Join(s.contentDir, "users", userID, date+".json")
			if _, err := os.Stat(userPath); err == nil {
				path = userPath
			}
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(writer, "content not found", http.StatusNotFound)
			return
		}
		http.Error(writer, "content unavailable", http.StatusInternalServerError)
		return
	}

	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		http.Error(writer, "content is invalid JSON", http.StatusInternalServerError)
		return
	}

	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(payload)
}

func (s *Server) loadContent(date string) (*content.DailyContent, error) {
	data, err := os.ReadFile(filepath.Join(s.contentDir, date+".json"))
	if err != nil {
		return nil, err
	}
	var daily content.DailyContent
	if err := json.Unmarshal(data, &daily); err != nil {
		return nil, err
	}
	return &daily, nil
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'; script-src 'unsafe-inline'")
		next.ServeHTTP(writer, request)
	})
}
