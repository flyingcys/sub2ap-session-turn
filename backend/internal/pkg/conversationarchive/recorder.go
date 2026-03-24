package conversationarchive

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
)

type RequestInput struct {
	Method  string
	Path    string
	Proto   string
	Headers http.Header
	Body    []byte
}

type Recorder struct {
	store      *Store
	sessionDir string
	turn       int
	filename   string
	request    HTTPMessage
}

func NewRecorder(ctx context.Context, store *Store, input RequestInput) (*Recorder, error) {
	if store == nil {
		return nil, fmt.Errorf("store is required")
	}

	resolved := ResolveSession(SessionInput{
		Path:    input.Path,
		Headers: input.Headers,
		Body:    input.Body,
	})
	turn, err := store.AllocateTurn(ctx, resolved.DirectoryName)
	filename := ""
	if err != nil {
		filename = fallbackTurnFilename()
		slog.Warn("conversation archive turn allocation failed, falling back to random filename",
			"session_dir", resolved.DirectoryName,
			"fallback_filename", filename,
			"error", err,
		)
	}

	return &Recorder{
		store:      store,
		sessionDir: resolved.DirectoryName,
		turn:       turn,
		filename:   filename,
		request: HTTPMessage{
			StartLine: fmt.Sprintf("%s %s %s", input.Method, input.Path, input.Proto),
			Headers:   flattenHeaders(input.Headers),
			Body:      string(input.Body),
		},
	}, nil
}

func (r *Recorder) Finish(ctx context.Context, writer *CaptureWriter) error {
	if r == nil || writer == nil {
		return nil
	}

	statusCode := writer.Status()
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	response := HTTPMessage{
		StartLine: fmt.Sprintf("HTTP/1.1 %d %s", statusCode, http.StatusText(statusCode)),
		Headers:   flattenHeaders(writer.Header()),
		Body:      string(writer.BodyBytes()),
	}

	_, err := r.store.WriteTurn(ctx, TurnRecord{
		SessionDir: r.sessionDir,
		Turn:       r.turn,
		Filename:   r.filename,
		Request:    r.request,
		Response:   response,
	})
	return err
}

func flattenHeaders(headers http.Header) []Header {
	if headers == nil {
		return nil
	}

	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]Header, 0, len(keys))
	for _, key := range keys {
		for _, value := range headers.Values(key) {
			result = append(result, Header{Name: key, Value: value})
		}
	}
	return result
}
