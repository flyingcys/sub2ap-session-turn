package handler

import (
	"os"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/conversationarchive"
	"github.com/gin-gonic/gin"
)

const defaultConversationArchiveRoot = "archive/conversations"

func beginConversationArchive(c *gin.Context, body []byte) (*conversationarchive.Recorder, *conversationarchive.CaptureWriter) {
	if c == nil || c.Request == nil || c.Writer == nil {
		return nil, nil
	}

	captureWriter := conversationarchive.WrapWriter(c.Writer)
	c.Writer = captureWriter

	root := strings.TrimSpace(os.Getenv("SUB2API_CONVERSATION_ARCHIVE_ROOT"))
	if root == "" {
		root = defaultConversationArchiveRoot
	}

	recorder, err := conversationarchive.NewRecorder(c.Request.Context(), conversationarchive.NewStore(root), conversationarchive.RequestInput{
		Method:  c.Request.Method,
		Path:    c.Request.URL.Path,
		Proto:   c.Request.Proto,
		Headers: c.Request.Header.Clone(),
		Body:    body,
	})
	if err != nil {
		return nil, captureWriter
	}
	return recorder, captureWriter
}

func finishConversationArchive(c *gin.Context, recorder *conversationarchive.Recorder, writer *conversationarchive.CaptureWriter) {
	if c == nil || recorder == nil || writer == nil {
		return
	}
	_ = recorder.Finish(c.Request.Context(), writer)
}
