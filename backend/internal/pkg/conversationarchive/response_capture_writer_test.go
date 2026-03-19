package conversationarchive

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCaptureWriterMirrorsBodyAndStatus(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	writer := WrapWriter(ctx.Writer)
	writer.Header().Set("Content-Type", "text/plain")
	writer.WriteHeader(202)
	_, err := writer.Write([]byte("hello"))
	require.NoError(t, err)

	require.Equal(t, 202, rec.Code)
	require.Equal(t, "hello", rec.Body.String())
	require.Equal(t, 202, writer.Status())
	require.Equal(t, "hello", string(writer.BodyBytes()))
}

func TestCaptureWriterSupportsWriteString(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	writer := WrapWriter(ctx.Writer)
	_, err := writer.WriteString("stream-data")
	require.NoError(t, err)

	require.Equal(t, "stream-data", rec.Body.String())
	require.Equal(t, "stream-data", string(writer.BodyBytes()))
}
