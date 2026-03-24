package conversationarchive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

type Store struct {
	root string
}

type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ArchivedTurn struct {
	RequestBody  any                  `json:"request_body,omitempty"`
	ResponseBody ArchivedResponseBody `json:"response_body,omitempty"`
	ResponseText string               `json:"response_text,omitempty"`
	Usage        any                  `json:"usage,omitempty"`
	OutputItems  any                  `json:"output_items,omitempty"`
}

type ArchivedResponseBody struct {
	JSON      any                `json:"json,omitempty"`
	Completed map[string]any     `json:"completed,omitempty"`
}

type HTTPMessage struct {
	StartLine string   `json:"start_line,omitempty"`
	Headers   []Header `json:"headers,omitempty"`
	Body      string   `json:"body,omitempty"`
}

type TurnRecord struct {
	SessionDir string
	Turn       int
	Filename   string
	Request    HTTPMessage
	Response   HTTPMessage
}

func NewStore(root string) *Store {
	return &Store{root: root}
}

func (s *Store) AllocateTurn(ctx context.Context, sessionDir string) (int, error) {
	dir := filepath.Join(s.root, sessionDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}

	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}

		nextTurn, err := nextTurnNumber(dir)
		if err != nil {
			return 0, err
		}

		reservationPath := filepath.Join(dir, reservationFilename(nextTurn))
		file, err := os.OpenFile(reservationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			if closeErr := file.Close(); closeErr != nil {
				_ = os.Remove(reservationPath)
				return 0, closeErr
			}
			return nextTurn, nil
		}
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return 0, err
	}
}

func (s *Store) WriteTurn(_ context.Context, record TurnRecord) (string, error) {
	dir := filepath.Join(s.root, record.SessionDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	filename := record.Filename
	if filename == "" {
		filename = turnFilename(record.Turn)
	}
	path := filepath.Join(dir, filename)
	tempPath := filepath.Join(dir, "."+filename+"."+uuid.NewString()+".tmp")
	content, err := renderTurnRecord(record)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(tempPath, content, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return "", err
	}
	if record.Turn > 0 {
		_ = os.Remove(filepath.Join(dir, reservationFilename(record.Turn)))
	}
	return path, nil
}

func renderTurnRecord(record TurnRecord) ([]byte, error) {
	payload := buildArchivedTurn(record)
	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

func buildArchivedTurn(record TurnRecord) ArchivedTurn {
	responseBody, responseText, usage, outputItems := buildArchivedResponse(record.Response)

	return ArchivedTurn{
		RequestBody:  parseBodyValue(record.Request.Body),
		ResponseBody: responseBody,
		ResponseText: responseText,
		Usage:        usage,
		OutputItems:  outputItems,
	}
}

func buildArchivedResponse(message HTTPMessage) (ArchivedResponseBody, string, any, any) {
	contentType := strings.ToLower(headerValue(message.Headers, "content-type"))
	if strings.Contains(contentType, "text/event-stream") {
		return parseSSEBody(message.Body)
	}

	return ArchivedResponseBody{
		JSON: parseBodyValue(message.Body),
	}, "", nil, nil
}

func parseSSEBody(body string) (ArchivedResponseBody, string, any, any) {
	blocks := strings.Split(body, "\n\n")

	var (
		completed    map[string]any
		responseText string
		usage        any
		outputItems  any
	)

	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		_, data := parseSSEBlock(block)
		parsedData := parseBodyValue(data)

		payload, ok := parsedData.(map[string]any)
		if !ok {
			continue
		}

		if responseText == "" && payload["type"] == "response.output_text.done" {
			if text, ok := payload["text"].(string); ok {
				responseText = text
			}
		}

		if payload["type"] != "response.completed" {
			continue
		}

		response, ok := payload["response"].(map[string]any)
		if !ok {
			continue
		}

		completed = response
		usage = response["usage"]
		outputItems = response["output"]
		if text := extractResponseText(outputItems); text != "" {
			responseText = text
		}
	}

	return ArchivedResponseBody{
		Completed: completed,
	}, responseText, usage, outputItems
}

func parseSSEBlock(block string) (string, string) {
	var (
		event     string
		dataLines []string
	)

	for _, line := range strings.Split(block, "\n") {
		switch {
		case strings.HasPrefix(line, "event: "):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event: "))
		case strings.HasPrefix(line, "data: "):
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
		}
	}

	return event, strings.Join(dataLines, "\n")
}

func parseBodyValue(body string) any {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return nil
	}

	var value any
	if err := json.Unmarshal([]byte(trimmed), &value); err == nil {
		return value
	}
	return body
}

func extractResponseText(outputItems any) string {
	items, ok := outputItems.([]any)
	if !ok {
		return ""
	}

	texts := make([]string, 0, len(items))
	for _, item := range items {
		payload, ok := item.(map[string]any)
		if !ok || payload["type"] != "message" {
			continue
		}

		contentItems, ok := payload["content"].([]any)
		if !ok {
			continue
		}

		var parts []string
		for _, content := range contentItems {
			part, ok := content.(map[string]any)
			if !ok || part["type"] != "output_text" {
				continue
			}
			text, _ := part["text"].(string)
			if text != "" {
				parts = append(parts, text)
			}
		}
		if len(parts) > 0 {
			texts = append(texts, strings.Join(parts, ""))
		}
	}

	return strings.Join(texts, "\n\n")
}

func headerValue(headers []Header, name string) string {
	for _, header := range headers {
		if strings.EqualFold(header.Name, name) {
			return header.Value
		}
	}
	return ""
}

func nextTurnNumber(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}

	maxTurn := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		baseName, ok := cutTurnFileSuffix(name)
		if !ok {
			continue
		}

		turn, err := strconv.Atoi(baseName)
		if err != nil {
			continue
		}
		if turn > maxTurn {
			maxTurn = turn
		}
	}

	return maxTurn + 1, nil
}

func reservationFilename(turn int) string {
	return fmt.Sprintf("%04d.pending", turn)
}

func turnFilename(turn int) string {
	if turn > 0 {
		return fmt.Sprintf("%04d.json", turn)
	}
	return fallbackTurnFilename()
}

func fallbackTurnFilename() string {
	return "fallback-" + uuid.NewString() + ".json"
}

func cutTurnFileSuffix(name string) (string, bool) {
	for _, suffix := range []string{".json", ".txt", ".pending"} {
		if baseName, ok := strings.CutSuffix(name, suffix); ok {
			return baseName, true
		}
	}
	return "", false
}
