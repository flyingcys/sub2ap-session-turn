package conversationarchive

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRecorderFinishWritesTurnFileFromCapturedResponse(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	recorder, err := NewRecorder(context.Background(), NewStore(root), RequestInput{
		Method:  http.MethodPost,
		Path:    "/responses",
		Proto:   "HTTP/1.1",
		Headers: http.Header{"Session_Id": []string{"sess-1"}},
		Body:    []byte(`{"input":"hello"}`),
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	httpRecorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(httpRecorder)
	writer := WrapWriter(ctx.Writer)
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.WriteHeader(200)
	_, err = writer.Write([]byte("event: done\ndata: {}\n\n"))
	require.NoError(t, err)

	err = recorder.Finish(context.Background(), writer)
	require.NoError(t, err)

	files, err := filepath.Glob(filepath.Join(root, "sess_*", "*.txt"))
	require.NoError(t, err)
	require.Len(t, files, 1)

	content, err := os.ReadFile(files[0])
	require.NoError(t, err)
	text := string(content)
	require.Contains(t, text, "POST /responses HTTP/1.1")
	require.Contains(t, text, `{"input":"hello"}`)
	require.Contains(t, text, "HTTP/1.1 200 OK")
	require.Contains(t, text, "event: done")
}
