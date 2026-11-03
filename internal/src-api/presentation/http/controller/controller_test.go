package controller

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/meshestra/go-playground/jp/internal/src-api/presentation/http/adapter"
)

func TestServerServesEmbeddedIndex(t *testing.T) {
	server, err := NewController(t.TempDir(), adapter.Dependencies{})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	recorder := httptest.NewRecorder()
	server.Index(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if recorder.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("unexpected content type: %s", recorder.Header().Get("Content-Type"))
	}
}

func TestServerServesDailyContent(t *testing.T) {
	contentDir := t.TempDir()
	date := time.Now().Format("2006-01-02")
	content := `{"date":"` + date + `","theme":"운동","level":"N3","words":[],"sentences":[],"grammar":{},"moments_text":"오늘의 일본어"}`
	if err := os.WriteFile(filepath.Join(contentDir, date+".json"), []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	server, err := NewController(contentDir, adapter.Dependencies{})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	recorder := httptest.NewRecorder()
	server.ContentByDate(recorder, httptest.NewRequest(http.MethodGet, "/api/content/"+date, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content type: %s", recorder.Header().Get("Content-Type"))
	}
}

func TestServerRendersDailyContentWithGoTemplate(t *testing.T) {
	contentDir := t.TempDir()
	date := time.Now().Format("2006-01-02")
	content := `{"date":"` + date + `","theme":"운동","level":"N3","words":[{"word":"走る","reading":"はしる","meaning":"달리다","example":"毎朝走る。","example_translation":"매일 아침 달린다."}],"sentences":[{"japanese":"運動します。","reading":"うんどうします。","translation":"운동합니다."}],"grammar":{"pattern":"ます","meaning":"정중체","explanation":"정중한 표현","examples":[{"japanese":"食べます。","reading":"たべます。","translation":"먹습니다."}]},"moments_text":"오늘은 운동했어요."}`
	if err := os.WriteFile(filepath.Join(contentDir, date+".json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	server, err := NewController(contentDir, adapter.Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/today", nil)
	recorder := httptest.NewRecorder()
	server.Index(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "走る") || !strings.Contains(recorder.Body.String(), "운동") {
		t.Fatalf("expected server-rendered content, got %s", recorder.Body.String())
	}
}

func TestServerRejectsPathTraversal(t *testing.T) {
	server, err := NewController(t.TempDir(), adapter.Dependencies{})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}

	recorder := httptest.NewRecorder()
	server.ContentByDate(recorder, httptest.NewRequest(http.MethodGet, "/api/content/../../secret", nil))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}
