package conversationarchive

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type Store struct {
	root string
	mu   sync.Mutex
	lock map[string]*sync.Mutex
}

type HTTPMessage struct {
	StartLine string
	Headers   [][2]string
	Body      []byte
}

type TurnRecord struct {
	SessionDir string
	Turn       int
	Request    HTTPMessage
	Response   HTTPMessage
}

func NewStore(root string) *Store {
	return &Store{
		root: root,
		lock: make(map[string]*sync.Mutex),
	}
}

func (s *Store) AllocateTurn(_ context.Context, sessionDir string) (int, error) {
	sessionLock := s.sessionLock(sessionDir)
	sessionLock.Lock()
	defer sessionLock.Unlock()

	dir := filepath.Join(s.root, sessionDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*.txt"))
	if err != nil {
		return 0, err
	}
	if len(matches) == 0 {
		return 1, nil
	}

	sort.Strings(matches)
	last := filepath.Base(matches[len(matches)-1])
	turnText := strings.TrimSuffix(last, filepath.Ext(last))
	turn, err := strconv.Atoi(turnText)
	if err != nil {
		return 0, fmt.Errorf("parse turn from %q: %w", last, err)
	}
	return turn + 1, nil
}

func (s *Store) WriteTurn(_ context.Context, record TurnRecord) (string, error) {
	dir := filepath.Join(s.root, record.SessionDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	filename := fmt.Sprintf("%04d.txt", record.Turn)
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, renderTurnRecord(record), 0o644); err != nil {
		return "", err
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

func (s *Store) sessionLock(sessionDir string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()

	if lock, ok := s.lock[sessionDir]; ok {
		return lock
	}

	lock := &sync.Mutex{}
	s.lock[sessionDir] = lock
	return lock
}
