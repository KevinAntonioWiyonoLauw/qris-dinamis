package main

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
)

func randomHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// spaFileServer serves static files and falls back to index.html for
// non-API, non-file routes so client-side routing works.
func spaFileServer(fs http.FileSystem) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			f, err := fs.Open(path)
			if err == nil {
				info, statErr := f.Stat()
				if statErr == nil && !info.IsDir() {
					defer f.Close()
					if ct := guessContentType(path); ct != "" {
						w.Header().Set("Content-Type", ct)
					}
					io.Copy(w, f)
					return
				}
				f.Close()
			}
		}
		index, err := fs.Open("index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer index.Close()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.Copy(w, index)
	})
}

func guessContentType(path string) string {
	switch {
	case strings.HasSuffix(path, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		return "application/javascript"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".woff2"):
		return "font/woff2"
	case strings.HasSuffix(path, ".woff"):
		return "font/woff"
	case strings.HasSuffix(path, ".webmanifest"):
		return "application/manifest+json"
	case strings.HasSuffix(path, ".json"):
		return "application/json"
	case strings.HasSuffix(path, ".ico"):
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}
