package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ollama-tui/internal/storage"
	"ollama-tui/internal/telemetry"
)

type Client struct {
	baseURL string
	http    *http.Client
}

type Model struct {
	Name       string    `json:"name"`
	ModifiedAt time.Time `json:"modified_at"`
	Size       int64     `json:"size"`
}

type modelListResponse struct {
	Models []Model `json:"models"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type streamResponse struct {
	Model              string      `json:"model"`
	CreatedAt          time.Time   `json:"created_at"`
	Message            chatMessage `json:"message"`
	Done               bool        `json:"done"`
	TotalDuration      int64       `json:"total_duration"`
	LoadDuration       int64       `json:"load_duration"`
	PromptEvalCount    int         `json:"prompt_eval_count"`
	PromptEvalDuration int64       `json:"prompt_eval_duration"`
	EvalCount          int         `json:"eval_count"`
	EvalDuration       int64       `json:"eval_duration"`
	Error              string      `json:"error"`
}

type StreamChunk struct {
	Content   string
	Done      bool
	Telemetry *telemetry.Telemetry
	Err       error
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not connect to Ollama at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var out modelListResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Models, nil
}

func (c *Client) StreamChat(ctx context.Context, model, systemPrompt string, messages []storage.Message) <-chan StreamChunk {
	out := make(chan StreamChunk)
	go func() {
		defer close(out)
		started := time.Now()
		body, err := json.Marshal(chatRequest{
			Model:    model,
			Stream:   true,
			Messages: buildMessages(systemPrompt, messages),
		})
		if err != nil {
			out <- StreamChunk{Err: err}
			return
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(body))
		if err != nil {
			out <- StreamChunk{Err: err}
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := (&http.Client{}).Do(req)
		if err != nil {
			if ctx.Err() != nil {
				out <- StreamChunk{Err: ctx.Err()}
				return
			}
			out <- StreamChunk{Err: fmt.Errorf("could not connect to Ollama at %s: %w", c.baseURL, err)}
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			data, _ := io.ReadAll(resp.Body)
			out <- StreamChunk{Err: fmt.Errorf("Ollama returned %s: %s", resp.Status, strings.TrimSpace(string(data)))}
			return
		}

		var firstToken time.Time
		var lastToken time.Time
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			var item streamResponse
			if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
				out <- StreamChunk{Err: err}
				return
			}
			if item.Error != "" {
				out <- StreamChunk{Err: errors.New(item.Error)}
				return
			}
			if item.Message.Content != "" {
				now := time.Now()
				if firstToken.IsZero() {
					firstToken = now
				}
				lastToken = now
				out <- StreamChunk{Content: item.Message.Content}
			}
			if item.Done {
				ended := time.Now()
				t := telemetry.FromOllama(model, started, firstToken, lastToken, ended, "done", item.PromptEvalCount, item.EvalCount, item.TotalDuration, item.LoadDuration, item.PromptEvalDuration, item.EvalDuration)
				out <- StreamChunk{Done: true, Telemetry: &t}
				return
			}
		}
		if err := scanner.Err(); err != nil {
			if ctx.Err() != nil {
				out <- StreamChunk{Err: ctx.Err()}
				return
			}
			out <- StreamChunk{Err: err}
		}
	}()
	return out
}

func buildMessages(systemPrompt string, messages []storage.Message) []chatMessage {
	out := make([]chatMessage, 0, len(messages)+1)
	if strings.TrimSpace(systemPrompt) != "" {
		out = append(out, chatMessage{Role: "system", Content: systemPrompt})
	}
	for _, msg := range messages {
		if msg.Role == "system" {
			continue
		}
		out = append(out, chatMessage{Role: msg.Role, Content: msg.Content})
	}
	return out
}
