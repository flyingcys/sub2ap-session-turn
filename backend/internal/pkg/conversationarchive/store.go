package conversationarchive

import (
	"bytes"
	"context"
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

type HTTPMessage struct {
	StartLine string
	Headers   [][2]string
	Body      []byte
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
	if err := os.WriteFile(tempPath, renderTurnRecord(record), 0o644); err != nil {
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

func renderTurnRecord(record TurnRecord) []byte {
	var buffer bytes.Buffer
	writeSection := func(title string, message HTTPMessage) {
		buffer.WriteString(title)
		buffer.WriteByte('\n')
		if message.StartLine != "" {
			buffer.WriteString(message.StartLine)
			buffer.WriteByte('\n')
		}
		for _, header := range message.Headers {
			buffer.WriteString(header[0])
			buffer.WriteString(": ")
			buffer.WriteString(header[1])
			buffer.WriteByte('\n')
		}
		buffer.WriteByte('\n')
		buffer.Write(message.Body)
		buffer.WriteString("\n\n")
	}

	writeSection("===== REQUEST =====", record.Request)
	writeSection("===== RESPONSE =====", record.Response)
	return buffer.Bytes()
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
		baseName, ok := strings.CutSuffix(name, ".txt")
		if !ok {
			baseName, ok = strings.CutSuffix(name, ".pending")
			if !ok {
				continue
			}
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
		return fmt.Sprintf("%04d.txt", turn)
	}
	return fallbackTurnFilename()
}

func fallbackTurnFilename() string {
	return "fallback-" + uuid.NewString() + ".txt"
}
