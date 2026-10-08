package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Todo struct {
	ID      int     `json:"id"`
	Title   string  `json:"title"`
	DueDate *string `json:"dueDate"`
}

type todoStore struct {
	mu    sync.Mutex
	items []Todo
}

func (s *todoStore) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "index.html")
	})
	mux.HandleFunc("/api/todos", s.todos)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (s *todoStore) todos(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.mu.Lock()
		items := append([]Todo{}, s.items...)
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"todos": items})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "지원하지 않는 요청 방식입니다."})
		return
	}
	var input map[string]json.RawMessage
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "요청 형식이 올바르지 않습니다."})
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "요청 형식이 올바르지 않습니다."})
		return
	}
	raw, exists := input["title"]
	if !exists {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "제목을 입력해 주세요."})
		return
	}
	var decodedTitle any
	if err := json.Unmarshal(raw, &decodedTitle); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "요청 형식이 올바르지 않습니다."})
		return
	}
	title, ok := decodedTitle.(string)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "요청 형식이 올바르지 않습니다."})
		return
	}
	title = strings.TrimSpace(title)
	if title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "제목을 입력해 주세요."})
		return
	}
	if len([]rune(title)) > 200 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "제목은 200자까지 입력할 수 있습니다."})
		return
	}
	var dueDate *string
	if rawDueDate, exists := input["dueDate"]; exists && string(rawDueDate) != "null" {
		var value string
		if err := json.Unmarshal(rawDueDate, &value); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "요청 형식이 올바르지 않습니다."})
			return
		}
		if value != "" {
			if !validDateFormat(value) {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "마감일은 YYYY-MM-DD 형식으로 입력해 주세요."})
				return
			}
			parsed, err := time.Parse("2006-01-02", value)
			if err != nil || parsed.Format("2006-01-02") != value {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "달력에 없는 날짜입니다. 다시 확인해 주세요."})
				return
			}
			dueDate = &value
		}
	}
	s.mu.Lock()
	item := Todo{ID: len(s.items) + 1, Title: title, DueDate: dueDate}
	s.items = append(s.items, item)
	s.mu.Unlock()
	writeJSON(w, http.StatusCreated, item)
}

func validDateFormat(value string) bool {
	if len(value) != 10 || value[4] != '-' || value[7] != '-' {
		return false
	}
	for i, r := range value {
		if i == 4 || i == 7 {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func main() {
	listen := flag.String("listen", "127.0.0.1:8091", "HTTP listen address")
	flag.Parse()
	log.Fatal(http.ListenAndServe(*listen, (&todoStore{}).handler()))
}
