package conversationarchive

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllocateNextTurnSequential(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)
	ctx := context.Background()

	first, err := store.AllocateTurn(ctx, "sess_abc")
	require.NoError(t, err)
	require.Equal(t, 1, first)

	err = os.WriteFile(filepath.Join(root, "sess_abc", "0001.txt"), []byte("existing"), 0o644)
	require.NoError(t, err)

	second, err := store.AllocateTurn(ctx, "sess_abc")
	require.NoError(t, err)
	require.Equal(t, 2, second)
}

func TestWriteTurnFileCreatesExpectedFormat(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)

	path, err := store.WriteTurn(context.Background(), TurnRecord{
		SessionDir: "sess_abc",
		Turn:       1,
		Request: HTTPMessage{
			StartLine: "POST /responses HTTP/1.1",
			Headers: []Header{
				{Name: "content-type", Value: "application/json"},
			},
			Body: `{"input":"hello"}`,
		},
		Response: HTTPMessage{
			StartLine: "HTTP/1.1 200 OK",
			Headers: []Header{
				{Name: "content-type", Value: "text/event-stream"},
			},
			Body: "event: done\ndata: {}\n\n",
		},
	})
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "sess_abc", "0001.json"), path)

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	var archived ArchivedTurn
	require.NoError(t, json.Unmarshal(content, &archived))
	require.Equal(t, map[string]any{"input": "hello"}, archived.RequestBody)
	require.Len(t, archived.ResponseBody.Events, 1)
	require.Equal(t, ArchivedSSEEvent{
		Seq:   1,
		Event: "done",
		Data:  map[string]any{},
	}, archived.ResponseBody.Events[0])
	require.Nil(t, archived.ResponseBody.Completed)
	require.Empty(t, archived.ResponseText)
	require.Nil(t, archived.Usage)
	require.Nil(t, archived.OutputItems)
}

func TestWriteTurnFileExtractsCompletedResponseMetadata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)

	path, err := store.WriteTurn(context.Background(), TurnRecord{
		SessionDir: "sess_meta",
		Turn:       1,
		Request: HTTPMessage{
			Headers: []Header{
				{Name: "content-type", Value: "application/json"},
			},
			Body: `{"input":"hi"}`,
		},
		Response: HTTPMessage{
			Headers: []Header{
				{Name: "content-type", Value: "text/event-stream"},
			},
			Body: "event: response.output_text.done\ndata: {\"type\":\"response.output_text.done\",\"text\":\"Hi.\"}\n\n" +
				"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"output\":[{\"type\":\"reasoning\",\"encrypted_content\":\"abc\"},{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"Hi.\"}]}],\"usage\":{\"input_tokens\":1,\"output_tokens\":2}}}\n\n",
		},
	})
	require.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	var archived ArchivedTurn
	require.NoError(t, json.Unmarshal(content, &archived))
	require.Equal(t, "Hi.", archived.ResponseText)
	require.Equal(t, map[string]any{"input_tokens": float64(1), "output_tokens": float64(2)}, archived.Usage)

	items, ok := archived.OutputItems.([]any)
	require.True(t, ok)
	require.Len(t, items, 2)

	require.NotNil(t, archived.ResponseBody.Completed)
	require.Len(t, archived.ResponseBody.Events, 2)
}

func TestAllocateNextTurnConcurrent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := NewStore(root)
	ctx := context.Background()

	const workers = 4
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		numbers []int
		errs    []error
	)

	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			turn, err := store.AllocateTurn(ctx, "sess_concurrent")
			if err == nil {
				err = os.WriteFile(filepath.Join(root, "sess_concurrent", fmt.Sprintf("%04d.json", turn)), []byte("reserved"), 0o644)
			}

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			numbers = append(numbers, turn)
		}()
	}
	wg.Wait()

	require.Empty(t, errs)
	sort.Ints(numbers)
	require.Equal(t, []int{1, 2, 3, 4}, numbers)
}

func TestAllocateNextTurnReservesAcrossStoreInstances(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	firstStore := NewStore(root)
	secondStore := NewStore(root)

	firstTurn, err := firstStore.AllocateTurn(context.Background(), "sess_shared")
	require.NoError(t, err)
	require.Equal(t, 1, firstTurn)

	secondTurn, err := secondStore.AllocateTurn(context.Background(), "sess_shared")
	require.NoError(t, err)
	require.Equal(t, 2, secondTurn)
}
