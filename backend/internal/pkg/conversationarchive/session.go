package conversationarchive

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

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
			value:  strings.TrimSpace(gjson.GetBytes(input.Body, "metadata.user_id").String()),
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

	fallback := strings.TrimSpace(input.Path) + "\n" + string(input.Body)
	if fallback == "\n" {
		fallback = "empty-request"
	}
	return ResolvedSession{
		RawKey:        fallback,
		Source:        "fallback.request_hash",
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
