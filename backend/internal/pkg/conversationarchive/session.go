package conversationarchive

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

type SessionInput struct {
	Path    string
	Headers http.Header
	Body    []byte
}

type ResolvedSession struct {
	RawKey        string
	Source        string
	DirectoryName string
}

var legacyMetadataUserIDPattern = regexp.MustCompile(`_session_([a-fA-F0-9-]{36})$`)

func ResolveSession(input SessionInput) ResolvedSession {
	candidates := []struct {
		value  string
		source string
	}{
		{
			value:  strings.TrimSpace(getHeader(input.Headers, "session_id")),
			source: "header.session_id",
		},
		{
			value:  strings.TrimSpace(getHeader(input.Headers, "conversation_id")),
			source: "header.conversation_id",
		},
		{
			value:  strings.TrimSpace(gjson.GetBytes(input.Body, "prompt_cache_key").String()),
			source: "body.prompt_cache_key",
		},
	}

	if strings.TrimSpace(input.Path) == "/v1/messages" {
		candidates = append(candidates, struct {
			value  string
			source string
		}{
			value:  resolveMetadataUserIDSession(input.Body),
			source: "body.metadata.user_id",
		})
	}

	for _, candidate := range candidates {
		if candidate.value == "" {
			continue
		}
		return ResolvedSession{
			RawKey:        candidate.value,
			Source:        candidate.source,
			DirectoryName: sessionDirectoryName(candidate.value),
		}
	}

	fallback := temporarySessionKey()
	return ResolvedSession{
		RawKey:        fallback,
		Source:        "fallback.request_temp",
		DirectoryName: sessionDirectoryName(fallback),
	}
}

func sessionDirectoryName(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return "sess_" + hex.EncodeToString(sum[:8])
}

func getHeader(headers http.Header, key string) string {
	if headers == nil {
		return ""
	}
	for headerKey, values := range headers {
		if !strings.EqualFold(strings.TrimSpace(headerKey), key) {
			continue
		}
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				return value
			}
		}
	}
	return ""
}

func resolveMetadataUserIDSession(body []byte) string {
	raw := strings.TrimSpace(gjson.GetBytes(body, "metadata.user_id").String())
	if raw == "" {
		return ""
	}

	if strings.HasPrefix(raw, "{") {
		var payload struct {
			SessionID string `json:"session_id"`
		}
		if err := json.Unmarshal([]byte(raw), &payload); err == nil && strings.TrimSpace(payload.SessionID) != "" {
			return strings.TrimSpace(payload.SessionID)
		}
	}

	if matches := legacyMetadataUserIDPattern.FindStringSubmatch(raw); len(matches) == 2 && strings.TrimSpace(matches[1]) != "" {
		return strings.TrimSpace(matches[1])
	}

	return raw
}

func temporarySessionKey() string {
	var random [16]byte
	if _, err := rand.Read(random[:]); err == nil {
		return "temp-request-" + hex.EncodeToString(random[:])
	}

	return fmt.Sprintf("temp-request-%d", time.Now().UnixNano())
}
