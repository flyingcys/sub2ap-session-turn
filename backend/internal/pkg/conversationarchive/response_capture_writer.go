package conversationarchive

import (
	"bytes"

	"github.com/gin-gonic/gin"
)

type CaptureWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func WrapWriter(writer gin.ResponseWriter) *CaptureWriter {
	return &CaptureWriter{ResponseWriter: writer}
}

func (w *CaptureWriter) Write(data []byte) (int, error) {
	if len(data) > 0 {
		_, _ = w.body.Write(data)
	}
	return w.ResponseWriter.Write(data)
}

func (w *CaptureWriter) WriteString(value string) (int, error) {
	if value != "" {
		_, _ = w.body.WriteString(value)
	}
	return w.ResponseWriter.WriteString(value)
}

func (w *CaptureWriter) BodyBytes() []byte {
	return w.body.Bytes()
}
