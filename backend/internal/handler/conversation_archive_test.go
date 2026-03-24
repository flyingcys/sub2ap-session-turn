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

	files, err := filepath.Glob(filepath.Join(root, "sess_*", "*.json"))
	require.NoError(t, err)
	require.Len(t, files, 1)

	content, err := os.ReadFile(files[0])
	require.NoError(t, err)
	require.NotContains(t, string(content), `"seq"`)
	require.NotContains(t, string(content), `"events"`)

	var archived struct {
		RequestBody map[string]any `json:"request_body"`
	}
	require.NoError(t, json.Unmarshal(content, &archived))
	require.Equal(t, map[string]any{"input": "hello"}, archived.RequestBody)
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

	files, err := filepath.Glob(filepath.Join(workingDir, "data", "archive", "conversations", "sess_*", "*.json"))
	require.NoError(t, err)
	require.Len(t, files, 1)
}

func TestConversationArchiveHelpers_SkipUnsupportedAntigravityMessagesPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SUB2API_CONVERSATION_ARCHIVE_ROOT", root)

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/antigravity/v1/messages", nil)
	ctx.Request.Header.Set("session_id", "sess-antigravity-should-skip")

	body := []byte(`{"model":"claude-sonnet-4-5"}`)
	archiveRecorder, captureWriter := beginConversationArchive(ctx, body)
	require.Nil(t, archiveRecorder)
	require.Nil(t, captureWriter)

	ctx.Writer.Header().Set("Content-Type", "application/json")
	ctx.Writer.WriteHeader(http.StatusOK)
	_, err := ctx.Writer.Write([]byte(`{"ok":true}`))
	require.NoError(t, err)

	finishConversationArchive(ctx, archiveRecorder, captureWriter)

	files, err := filepath.Glob(filepath.Join(root, "sess_*", "*.json"))
	require.NoError(t, err)
	require.Empty(t, files)
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

	files, err := filepath.Glob(filepath.Join(root, "sess_*", "*.json"))
	require.NoError(t, err)
	require.Len(t, files, 1)

	content, err := os.ReadFile(files[0])
	require.NoError(t, err)
	var archived struct {
		ResponseBody struct {
			JSON map[string]any `json:"json"`
		} `json:"response_body"`
	}
	require.NoError(t, json.Unmarshal(content, &archived))
	errorPayload, ok := archived.ResponseBody.JSON["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "upstream_error", errorPayload["type"])
	require.Equal(t, "Upstream request failed", errorPayload["message"])
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

	files, err := filepath.Glob(filepath.Join(root, "sess_*", "*.json"))
	require.NoError(t, err)
	require.Len(t, files, 1)

	content, err := os.ReadFile(files[0])
	require.NoError(t, err)
	var archived struct {
		ResponseBody struct {
			JSON map[string]any `json:"json"`
		} `json:"response_body"`
	}
	require.NoError(t, json.Unmarshal(content, &archived))
	errorPayload, ok := archived.ResponseBody.JSON["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "error", errorPayload["type"])
	require.Equal(t, "Internal server error", errorPayload["message"])
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

	files, err := filepath.Glob(filepath.Join(root, "sess_*", "*.json"))
	require.NoError(t, err)
	require.Len(t, files, 1)

	content, err := os.ReadFile(files[0])
	require.NoError(t, err)
	var archived struct {
		ResponseBody struct {
			JSON map[string]any `json:"json"`
		} `json:"response_body"`
	}
	require.NoError(t, json.Unmarshal(content, &archived))
	errorPayload, ok := archived.ResponseBody.JSON["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "internal error", errorPayload["message"])
}
