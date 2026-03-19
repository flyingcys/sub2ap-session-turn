package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

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
