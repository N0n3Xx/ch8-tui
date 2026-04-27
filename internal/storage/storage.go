package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/N0n3Xx/ch8-tui/internal/telemetry"
)

type Message struct {
	Role      string               `json:"role"`
	Content   string               `json:"content"`
	Timestamp time.Time            `json:"timestamp"`
	Model     string               `json:"model,omitempty"`
	Telemetry *telemetry.Telemetry `json:"telemetry,omitempty"`
}

type Chat struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	SelectedModel string    `json:"selected_model"`
	Messages      []Message `json:"messages"`
}

type Store struct {
	path string
}

var chatIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func New(path string) (*Store, error) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, err
	}
	return &Store{path: path}, nil
}

func NewChat(model string) *Chat {
	now := time.Now()
	return &Chat{
		ID:            fmt.Sprintf("%d", now.UnixNano()),
		Title:         "New chat",
		CreatedAt:     now,
		UpdatedAt:     now,
		SelectedModel: model,
		Messages:      []Message{},
	}
}

func (s *Store) Save(chat *Chat) error {
	if chat == nil {
		return nil
	}
	if chat.ID == "" {
		chat.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if !validChatID(chat.ID) {
		return fmt.Errorf("invalid chat id %q", chat.ID)
	}
	if chat.CreatedAt.IsZero() {
		chat.CreatedAt = time.Now()
	}
	if chat.Title == "" || chat.Title == "New chat" {
		chat.Title = TitleFromMessages(chat.Messages)
	}
	chat.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(chat, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.path, chat.ID+".json"), append(data, '\n'), 0o644)
}

func (s *Store) Load(id string) (*Chat, error) {
	if !validChatID(id) {
		return nil, fmt.Errorf("invalid chat id %q", id)
	}
	data, err := os.ReadFile(filepath.Join(s.path, id+".json"))
	if err != nil {
		return nil, err
	}
	var chat Chat
	if err := json.Unmarshal(data, &chat); err != nil {
		return nil, err
	}
	chat.ID = id
	return &chat, nil
}

func (s *Store) Delete(id string) error {
	if !validChatID(id) {
		return fmt.Errorf("invalid chat id %q", id)
	}
	err := os.Remove(filepath.Join(s.path, id+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Store) List() ([]*Chat, error) {
	files, err := os.ReadDir(s.path)
	if err != nil {
		return nil, err
	}
	chats := make([]*Chat, 0, len(files))
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}
		id := strings.TrimSuffix(file.Name(), ".json")
		if !validChatID(id) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.path, file.Name()))
		if err != nil {
			continue
		}
		var chat Chat
		if json.Unmarshal(data, &chat) == nil {
			chat.ID = id
			chats = append(chats, &chat)
		}
	}
	sort.Slice(chats, func(i, j int) bool {
		return chats[i].UpdatedAt.After(chats[j].UpdatedAt)
	})
	return chats, nil
}

func TitleFromMessages(messages []Message) string {
	for _, msg := range messages {
		if msg.Role == "user" && strings.TrimSpace(msg.Content) != "" {
			title := singleLine(msg.Content)
			if len(title) > 56 {
				title = title[:53] + "..."
			}
			return title
		}
	}
	return "New chat"
}

func Preview(chat *Chat) string {
	for i := len(chat.Messages) - 1; i >= 0; i-- {
		msg := strings.TrimSpace(chat.Messages[i].Content)
		if msg != "" {
			preview := singleLine(msg)
			if len(preview) > 72 {
				preview = preview[:69] + "..."
			}
			return preview
		}
	}
	return ""
}

func singleLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func validChatID(id string) bool {
	return id != "." && id != ".." && chatIDPattern.MatchString(id)
}
