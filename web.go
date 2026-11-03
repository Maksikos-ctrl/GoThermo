package main

import (
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// mountWeb serves the browser build of the interface from dir: the chat lives
// in dir/app and the bare address of the server opens it. Nothing is mounted
// when dir is empty or has no app/index.html, so the API keeps working alone.
func mountWeb(mux *http.ServeMux, dir string) bool {
	if dir == "" {
		return false
	}
	index := filepath.Join(dir, "app", "index.html")
	if _, err := os.Stat(index); err != nil {
		log.Printf("⚠️  GOTHERMO_WEB_DIR=%s has no app/index.html, the browser version is off", dir)
		return false
	}

	files := http.StripPrefix("/app", webFiles(http.Dir(filepath.Join(dir, "app"))))
	mux.Handle("/app/", files)
	mux.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app/", http.StatusFound)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/app/", http.StatusFound)
	})
	log.Printf("🌐 Browser version is served from %s", dir)
	return true
}

// webFiles serves static files only: no directory listings, no writes.
func webFiles(root http.FileSystem) http.Handler {
	files := http.FileServer(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p := path.Clean("/" + r.URL.Path)
		f, err := root.Open(p)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		st, err := f.Stat()
		_ = f.Close()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if st.IsDir() {
			idx, err := root.Open(path.Join(p, "index.html"))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			_ = idx.Close()
		}

		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		if strings.HasPrefix(p, "/assets/") {
			h.Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			h.Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}
