package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckedInSchemaIsCurrent(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("..", "..", "config", "config.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := schemaBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("config/config.schema.json is stale; run go generate ./backend/config")
	}
}
