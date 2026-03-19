package conversationarchive

import (
	"context"
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
			Headers: [][2]string{
				{"content-type", "application/json"},
			},
			Body: []byte(`{"input":"hello"}`),
		},
		Response: HTTPMessage{
			StartLine: "HTTP/1.1 200 OK",
			Headers: [][2]string{
				{"content-type", "text/event-stream"},
			},
			Body: []byte("event: done\ndata: {}\n\n"),
		},
	})
	require.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(content)
	require.Contains(t, text, "===== REQUEST =====")
	require.Contains(t, text, "POST /responses HTTP/1.1")
	require.Contains(t, text, `{"input":"hello"}`)
	require.Contains(t, text, "===== RESPONSE =====")
	require.Contains(t, text, "HTTP/1.1 200 OK")
	require.Contains(t, text, "event: done")
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
				err = os.WriteFile(filepath.Join(root, "sess_concurrent", fmt.Sprintf("%04d.txt", turn)), []byte("reserved"), 0o644)
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
