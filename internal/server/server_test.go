package server_test

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/oliverlambson/personal-site/internal/server"
	"github.com/oliverlambson/personal-site/web"
)

func TestEmbeddedSite(t *testing.T) {
	originalWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalWorkingDirectory); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	srv, err := server.NewServer("127.0.0.1:0", web.Files)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	testServer := httptest.NewServer(srv.Handler)
	t.Cleanup(testServer.Close)

	pages := map[string]string{
		"/":              "<title>index | oliverlambson</title>",
		"/bio":           "<title>Bio | oliverlambson</title>",
		"/posts":         "<title>posts | oliverlambson</title>",
		"/carbon-guitar": "<title>Why I make carbon fibre guitars | oliverlambson</title>",
		"/pgmq":          "<title>Use what you already have: Building a message queue on Postgres | oliverlambson</title>",
		"/bored-charts":  "<title>bored-charts | oliverlambson</title>",
	}
	for route, want := range pages {
		t.Run(route, func(t *testing.T) {
			body := get(t, testServer.URL+route, http.StatusOK)
			if !strings.Contains(string(body), want) {
				t.Fatalf("GET %s body does not contain %q", route, want)
			}
		})
	}

	if got := string(get(t, testServer.URL+"/healthz", http.StatusOK)); got != "OK" {
		t.Fatalf("GET /healthz body = %q, want %q", got, "OK")
	}
	get(t, testServer.URL+"/not-a-page", http.StatusNotFound)

	err = fs.WalkDir(web.Files, "static", func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		t.Run(file, func(t *testing.T) {
			want, readErr := fs.ReadFile(web.Files, file)
			if readErr != nil {
				t.Fatalf("read embedded asset: %v", readErr)
			}
			route := "/static/" + strings.TrimPrefix(file, "static/")
			got := get(t, testServer.URL+route, http.StatusOK)
			if !bytes.Equal(got, want) {
				t.Fatalf("GET %s differs from embedded asset", route)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded assets: %v", err)
	}
}

func get(t *testing.T, url string, wantStatus int) []byte {
	t.Helper()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read GET %s response: %v", url, err)
	}
	if resp.StatusCode != wantStatus {
		t.Fatalf("GET %s status = %d, want %d", url, resp.StatusCode, wantStatus)
	}
	return body
}
