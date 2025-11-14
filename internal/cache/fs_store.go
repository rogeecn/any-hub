package cache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// NewStore 以 basePath 为根目录构建磁盘缓存，整站复用一份实例。
func NewStore(basePath string) (Store, error) {
	if basePath == "" {
		return nil, errors.New("storage path required")
	}

	abs, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("resolve storage path: %w", err)
	}

	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("create storage path: %w", err)
	}

	return &fileStore{
		basePath: abs,
		locks:    make(map[string]*entryLock),
	}, nil
}

// fileStore 通过 entryLock 避免同一 Locator 并发写入，同时复用 basePath。
type fileStore struct {
	basePath string

	mu    sync.Mutex
	locks map[string]*entryLock
}

type entryLock struct {
	mu   sync.Mutex
	refs int
}

func (s *fileStore) Get(ctx context.Context, locator Locator) (*ReadResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	filePath, err := s.path(locator)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if info.IsDir() {
		return nil, ErrNotFound
	}

	f, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	entry := Entry{
		Locator:   locator,
		FilePath:  filePath,
		SizeBytes: info.Size(),
		ModTime:   info.ModTime(),
	}

	return &ReadResult{
		Entry:  entry,
		Reader: f,
	}, nil
}

func (s *fileStore) Put(ctx context.Context, locator Locator, body io.Reader, opts PutOptions) (*Entry, error) {
	unlock, err := s.lockEntry(locator)
	if err != nil {
		return nil, err
	}
	defer unlock()

	filePath, err := s.path(locator)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return nil, err
	}

	tempFile, err := os.CreateTemp(filepath.Dir(filePath), ".cache-*")
	if err != nil {
		return nil, err
	}
	tempName := tempFile.Name()

	written, err := copyWithContext(ctx, tempFile, body)
	closeErr := tempFile.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(tempName)
		return nil, err
	}

	if err := os.Rename(tempName, filePath); err != nil {
		os.Remove(tempName)
		return nil, err
	}

	modTime := opts.ModTime
	if modTime.IsZero() {
		modTime = time.Now().UTC()
	}
	if err := os.Chtimes(filePath, modTime, modTime); err != nil {
		return nil, err
	}

	entry := Entry{
		Locator:   locator,
		FilePath:  filePath,
		SizeBytes: written,
		ModTime:   modTime,
	}
	return &entry, nil
}

func (s *fileStore) Remove(ctx context.Context, locator Locator) error {
	unlock, err := s.lockEntry(locator)
	if err != nil {
		return err
	}
	defer unlock()

	filePath, err := s.path(locator)
	if err != nil {
		return err
	}
	if err := os.Remove(filePath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (s *fileStore) lockEntry(locator Locator) (func(), error) {
	key := locatorKey(locator)
	s.mu.Lock()
	lock := s.locks[key]
	if lock == nil {
		lock = &entryLock{}
		s.locks[key] = lock
	}
	lock.refs++
	s.mu.Unlock()

	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()
		s.mu.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(s.locks, key)
		}
		s.mu.Unlock()
	}, nil
}

func (s *fileStore) path(locator Locator) (string, error) {
	if locator.HubName == "" {
		return "", errors.New("hub name required")
	}

	rel := locator.Path
	if rel == "" || rel == "/" {
		rel = "root"
	}
	rel = path.Clean("/" + rel)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		rel = "root"
	}

	filePath := filepath.Join(s.basePath, locator.HubName, filepath.FromSlash(rel))
	if !strings.HasPrefix(filePath, filepath.Join(s.basePath, locator.HubName)) {
		return "", errors.New("invalid cache path")
	}
	return filePath, nil
}

func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	var copied int64
	buf := make([]byte, 32*1024)
	for {
		if err := ctx.Err(); err != nil {
			return copied, err
		}
		n, err := src.Read(buf)
		if n > 0 {
			w, wErr := dst.Write(buf[:n])
			copied += int64(w)
			if wErr != nil {
				return copied, wErr
			}
			if w < n {
				return copied, io.ErrShortWrite
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return copied, nil
			}
			return copied, err
		}
	}
}

func locatorKey(locator Locator) string {
	return locator.HubName + "::" + locator.Path
}
