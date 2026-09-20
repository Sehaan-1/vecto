// Command vecto-server is a disk-backed stub for vecto's HTTP remote cache.
//
// It speaks exactly the three calls internal/cache makes (ADR-0013):
//
//	HEAD /:hash  -> 200 when the entry exists, 404 otherwise
//	GET  /:hash  -> the stored JSON bundle, 404 when missing
//	PUT  /:hash  -> stores the request body, answers 200
//
// Entries are files under --dir, written atomically (temp file + rename).
// This is a spike, not a service: no auth, no TLS, no S3.
package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses flags, binds, and serves until the process stops.
// It never calls os.Exit itself (ADR-0011).
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("vecto-server", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var addr, dir string
	fs.StringVar(&addr, "addr", "127.0.0.1:8463", "TCP address to listen on (use port 0 for an ephemeral port)")
	fs.StringVar(&dir, "dir", ".vecto-remote", "Directory holding stored entries")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(stderr, "Error creating storage dir: %v\n", err)
		return 1
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(stderr, "Error listening on %s: %v\n", addr, err)
		return 1
	}
	// Tests parse this line to learn an ephemeral port.
	fmt.Fprintf(stdout, "vecto-server listening on %s (dir %s)\n", ln.Addr(), dir)

	srv := &server{dir: dir}
	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.handle)
	if err := http.Serve(ln, mux); err != nil {
		fmt.Fprintf(stderr, "Server stopped: %v\n", err)
		return 1
	}
	return 0
}

type server struct {
	dir string
}

// validHash keeps storage paths inside dir: no separators, no traversal.
func validHash(hash string) bool {
	if hash == "" || hash == "." || hash == ".." {
		return false
	}
	for _, c := range hash {
		isWord := c == '-' || c == '_' || c == '.' ||
			(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if !isWord {
			return false
		}
	}
	return true
}

func (s *server) path(hash string) string {
	return filepath.Join(s.dir, hash+".json")
}

func (s *server) handle(w http.ResponseWriter, r *http.Request) {
	hash := strings.TrimPrefix(r.URL.Path, "/")
	if !validHash(hash) {
		http.Error(w, "invalid hash", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodHead:
		if _, err := os.Stat(s.path(hash)); err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		http.ServeFile(w, r, s.path(hash))

	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "reading body", http.StatusBadRequest)
			return
		}
		tmp, err := os.CreateTemp(s.dir, "put-*")
		if err != nil {
			http.Error(w, "staging entry", http.StatusInternalServerError)
			return
		}
		tmpName := tmp.Name()
		if _, err := tmp.Write(body); err != nil {
			tmp.Close()
			os.Remove(tmpName)
			http.Error(w, "writing entry", http.StatusInternalServerError)
			return
		}
		if err := tmp.Close(); err != nil {
			os.Remove(tmpName)
			http.Error(w, "writing entry", http.StatusInternalServerError)
			return
		}
		if err := os.Rename(tmpName, s.path(hash)); err != nil {
			os.Remove(tmpName)
			http.Error(w, "committing entry", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
