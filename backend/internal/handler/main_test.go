package handler

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	archiveRoot, err := os.MkdirTemp("", "sub2api-handler-archive-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(archiveRoot)

	originalRoot, hadOriginal := os.LookupEnv("SUB2API_CONVERSATION_ARCHIVE_ROOT")
	if err := os.Setenv("SUB2API_CONVERSATION_ARCHIVE_ROOT", archiveRoot); err != nil {
		panic(err)
	}
	defer func() {
		if hadOriginal {
			_ = os.Setenv("SUB2API_CONVERSATION_ARCHIVE_ROOT", originalRoot)
			return
		}
		_ = os.Unsetenv("SUB2API_CONVERSATION_ARCHIVE_ROOT")
	}()

	return m.Run()
}
