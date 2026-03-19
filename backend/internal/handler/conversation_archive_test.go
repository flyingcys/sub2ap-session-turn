package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConversationArchiveHelpers_PersistTurnFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SUB2API_CONVERSATION_ARCHIVE_ROOT", root)

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/responses", nil)
	ctx.Request.Header.Set("session_id", "sess-archive-helper")

	body := []byte(`{"input":"hello"}`)
	archiveRecorder, captureWriter := beginConversationArchive(ctx, body)
	require.NotNil(t, captureWriter)
	require.NotNil(t, archiveRecorder)

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.WriteHeader(http.StatusOK)
	_, err := ctx.Writer.Write([]byte("event: done\ndata: {}\n\n"))
	require.NoError(t, err)

	finishConversationArchive(ctx, archiveRecorder, captureWriter)

	files, err := filepath.Glob(filepath.Join(root, "sess_*", "*.txt"))
	require.NoError(t, err)
	require.Len(t, files, 1)

	content, err := os.ReadFile(files[0])
	require.NoError(t, err)
	require.Contains(t, string(content), "POST /responses HTTP/1.1")
	require.Contains(t, string(content), "event: done")
}

func TestConversationArchiveHelpers_UseDataArchiveRootByDefault(t *testing.T) {
	workingDir := t.TempDir()
	originalWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(workingDir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(originalWD))
	})
	t.Setenv("SUB2API_CONVERSATION_ARCHIVE_ROOT", "")

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/responses", nil)
	ctx.Request.Header.Set("session_id", "sess-default-root")

	body := []byte(`{"input":"hello"}`)
	archiveRecorder, captureWriter := beginConversationArchive(ctx, body)
	require.NotNil(t, captureWriter)
	require.NotNil(t, archiveRecorder)

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.WriteHeader(http.StatusOK)
	_, err = ctx.Writer.Write([]byte("event: done\ndata: {}\n\n"))
	require.NoError(t, err)

	finishConversationArchive(ctx, archiveRecorder, captureWriter)

	files, err := filepath.Glob(filepath.Join(workingDir, "data", "archive", "conversations", "sess_*", "*.txt"))
	require.NoError(t, err)
	require.Len(t, files, 1)
}

func TestConversationArchiveHelpers_PanicRecoveryArchivesRecoveredOpenAIResponse(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SUB2API_CONVERSATION_ARCHIVE_ROOT", root)

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5"}`))
	ctx.Request.Header.Set("session_id", "sess-openai-panic")

	body := []byte(`{"model":"gpt-5"}`)
	h := &OpenAIGatewayHandler{}
	streamStarted := false

	require.NotPanics(t, func() {
		func() {
			archiveRecorder, captureWriter := beginConversationArchive(ctx, body)
			defer func() {
				if recovered := recover(); recovered != nil {
					h.handleRecoveredResponsesPanic(ctx, &streamStarted, recovered)
				}
				finishConversationArchive(ctx, archiveRecorder, captureWriter)
			}()
			panic("test panic")
		}()
	})

	require.Equal(t, http.StatusBadGateway, rec.Code)

	files, err := filepath.Glob(filepath.Join(root, "sess_*", "*.txt"))
	require.NoError(t, err)
	require.Len(t, files, 1)

	content, err := os.ReadFile(files[0])
	require.NoError(t, err)
	text := string(content)
	require.Contains(t, text, "HTTP/1.1 502 Bad Gateway")
	require.Contains(t, text, `"type":"upstream_error"`)
	require.Contains(t, text, `"message":"Upstream request failed"`)
}

func TestConversationArchiveHelpers_PanicRecoveryArchivesRecoveredAnthropicResponse(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SUB2API_CONVERSATION_ARCHIVE_ROOT", root)

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-sonnet-4-5"}`))
	ctx.Request.Header.Set("session_id", "sess-anthropic-panic")

	body := []byte(`{"model":"claude-sonnet-4-5"}`)
	h := &OpenAIGatewayHandler{}
	streamStarted := false

	require.NotPanics(t, func() {
		func() {
			archiveRecorder, captureWriter := beginConversationArchive(ctx, body)
			defer func() {
				if recovered := recover(); recovered != nil {
					h.handleRecoveredAnthropicMessagesPanic(ctx, &streamStarted, recovered)
				}
				finishConversationArchive(ctx, archiveRecorder, captureWriter)
			}()
			panic("test panic")
		}()
	})

	require.Equal(t, http.StatusInternalServerError, rec.Code)

	var parsed map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &parsed)
	require.NoError(t, err)

	files, err := filepath.Glob(filepath.Join(root, "sess_*", "*.txt"))
	require.NoError(t, err)
	require.Len(t, files, 1)

	content, err := os.ReadFile(files[0])
	require.NoError(t, err)
	text := string(content)
	require.Contains(t, text, "HTTP/1.1 500 Internal Server Error")
	require.Contains(t, text, `"type":"error"`)
	require.Contains(t, text, `"message":"Internal server error"`)
}

func TestConversationArchiveHelpers_GatewayMessagesPanicArchivesRecoveryResponse(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SUB2API_CONVERSATION_ARCHIVE_ROOT", root)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.Recovery())

	h := &GatewayHandler{}
	groupID := int64(99)
	user := &service.User{
		ID:          7,
		Role:        service.RoleUser,
		Concurrency: 1,
	}
	apiKey := &service.APIKey{
		ID:      11,
		UserID:  user.ID,
		GroupID: &groupID,
		User:    user,
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformAnthropic,
			Status:   service.StatusActive,
			Hydrated: true,
		},
	}

	router.POST("/v1/messages", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), apiKey)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{
			UserID:      user.ID,
			Concurrency: user.Concurrency,
		})
		c.Set(string(middleware.ContextKeyUserRole), user.Role)
		h.Messages(c)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-sonnet-4-5"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("session_id", "sess-gateway-panic")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), `"message":"internal error"`)

	files, err := filepath.Glob(filepath.Join(root, "sess_*", "*.txt"))
	require.NoError(t, err)
	require.Len(t, files, 1)

	content, err := os.ReadFile(files[0])
	require.NoError(t, err)
	text := string(content)
	require.Contains(t, text, "HTTP/1.1 500 Internal Server Error")
	require.Contains(t, text, `"message":"internal error"`)
}
