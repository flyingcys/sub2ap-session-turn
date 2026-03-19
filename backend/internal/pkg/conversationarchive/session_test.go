package conversationarchive

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveSessionKeyPriority(t *testing.T) {
	t.Run("session_id 优先于其他信号", func(t *testing.T) {
		resolved := ResolveSession(SessionInput{
			Path: "/responses",
			Headers: http.Header{
				"Session_Id":      []string{"sess-header"},
				"Conversation_Id": []string{"conv-header"},
			},
			Body: []byte(`{"prompt_cache_key":"pcache-1","metadata":{"user_id":"user-1"}}`),
		})

		require.Equal(t, "sess-header", resolved.RawKey)
		require.Equal(t, "header.session_id", resolved.Source)
		require.NotEmpty(t, resolved.DirectoryName)
	})

	t.Run("conversation_id 在 session_id 缺失时生效", func(t *testing.T) {
		resolved := ResolveSession(SessionInput{
			Path: "/responses",
			Headers: http.Header{
				"Conversation_Id": []string{"conv-header"},
			},
			Body: []byte(`{"prompt_cache_key":"pcache-1","metadata":{"user_id":"user-1"}}`),
		})

		require.Equal(t, "conv-header", resolved.RawKey)
		require.Equal(t, "header.conversation_id", resolved.Source)
	})

	t.Run("prompt_cache_key 在两个 header 都缺失时生效", func(t *testing.T) {
		resolved := ResolveSession(SessionInput{
			Path:    "/responses",
			Headers: http.Header{},
			Body:    []byte(`{"prompt_cache_key":"pcache-1","metadata":{"user_id":"user-1"}}`),
		})

		require.Equal(t, "pcache-1", resolved.RawKey)
		require.Equal(t, "body.prompt_cache_key", resolved.Source)
	})

	t.Run("metadata.user_id 作为 messages 的兜底信号", func(t *testing.T) {
		resolved := ResolveSession(SessionInput{
			Path:    "/v1/messages",
			Headers: http.Header{},
			Body:    []byte(`{"metadata":{"user_id":"user-1"}}`),
		})

		require.Equal(t, "user-1", resolved.RawKey)
		require.Equal(t, "body.metadata.user_id", resolved.Source)
	})

	t.Run("messages 里的 metadata.user_id JSON 取 session_id 以对齐现有语义", func(t *testing.T) {
		resolved := ResolveSession(SessionInput{
			Path:    "/v1/messages",
			Headers: http.Header{},
			Body: []byte(`{
				"metadata": {
					"user_id": "{\"device_id\":\"d61f76d0aabbccdd00112233445566778899aabbccddeeff0011223344556677\",\"account_uuid\":\"\",\"session_id\":\"c72554f2-1234-5678-abcd-123456789abc\"}"
				}
			}`),
		})

		require.Equal(t, "c72554f2-1234-5678-abcd-123456789abc", resolved.RawKey)
		require.Equal(t, "body.metadata.user_id", resolved.Source)
	})
}

func TestResolveSessionFallbackUsesPerRequestTemporarySession(t *testing.T) {
	t.Parallel()

	input := SessionInput{
		Path:    "/responses",
		Headers: http.Header{},
		Body:    []byte(`{"input":"hello"}`),
	}

	first := ResolveSession(input)
	second := ResolveSession(input)

	require.Equal(t, "fallback.request_temp", first.Source)
	require.Equal(t, "fallback.request_temp", second.Source)
	require.NotEqual(t, first.RawKey, second.RawKey)
	require.NotEqual(t, first.DirectoryName, second.DirectoryName)
}
