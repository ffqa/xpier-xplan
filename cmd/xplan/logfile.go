package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// maxLogBytes is the size at which serve.log is moved to serve.log.1.
const maxLogBytes = 5 << 20

// rotatingFile is an append-only log file that keeps one previous generation,
// so a server running for months does not fill the disk.
type rotatingFile struct {
	mu   sync.Mutex
	path string
	file *os.File
	size int64
}

func openRotatingFile(path string) (*rotatingFile, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	logFile := &rotatingFile{path: path}
	if err := logFile.open(); err != nil {
		return nil, err
	}
	if logFile.size >= maxLogBytes {
		if err := logFile.rotate(); err != nil {
			return nil, err
		}
	}
	return logFile, nil
}

func (r *rotatingFile) Write(data []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.size+int64(len(data)) > maxLogBytes {
		if err := r.rotate(); err != nil {
			fmt.Fprintln(os.Stderr, "xplan: rotate log:", err)
		}
	}
	written, err := r.file.Write(data)
	r.size += int64(written)
	return written, err
}

func (r *rotatingFile) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.file.Close()
}

func (r *rotatingFile) open() error {
	file, err := os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open log %s: %w", r.path, err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return fmt.Errorf("stat log %s: %w", r.path, err)
	}
	r.file, r.size = file, info.Size()
	return nil
}

func (r *rotatingFile) rotate() error {
	if r.file != nil {
		_ = r.file.Close()
	}
	if err := os.Rename(r.path, r.path+".1"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("rotate log: %w", err)
	}
	return r.open()
}
