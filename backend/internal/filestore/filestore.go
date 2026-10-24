// Package filestore хранит загруженные файлы в каталоге на диске.
package filestore

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

var (
	ErrTooLarge   = errors.New("file too large")
	ErrInvalidKey = errors.New("invalid storage key")
)

// Ключ — UUID, поэтому выйти за пределы каталога по нему нельзя.
var keyPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type Store struct {
	dir string
}

// New создаёт каталог хранилища и проверяет, что в него можно писать.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("filestore: create %s: %w", dir, err)
	}
	probe, err := os.CreateTemp(dir, ".probe-*")
	if err != nil {
		return nil, fmt.Errorf("filestore: %s is not writable: %w", dir, err)
	}
	probe.Close()
	os.Remove(probe.Name())
	return &Store{dir: dir}, nil
}

// Dir возвращает корневой каталог хранилища.
func (s *Store) Dir() string { return s.dir }

func (s *Store) path(key string) (string, error) {
	if !keyPattern.MatchString(key) {
		return "", ErrInvalidKey
	}
	return filepath.Join(s.dir, key[:2], key), nil
}

// Put пишет поток в файл не больше max байт: сначала во временный, затем переименование.
func (s *Store) Put(key string, r io.Reader, max int64) (int64, error) {
	p, err := s.path(key)
	if err != nil {
		return 0, err
	}
	sub := filepath.Dir(p)
	if err := os.MkdirAll(sub, 0o750); err != nil {
		return 0, fmt.Errorf("filestore: mkdir: %w", err)
	}
	tmp, err := os.CreateTemp(sub, ".tmp-*")
	if err != nil {
		return 0, fmt.Errorf("filestore: temp file: %w", err)
	}
	tmpName := tmp.Name()
	n, copyErr := io.Copy(tmp, io.LimitReader(r, max+1))
	closeErr := tmp.Close()
	switch {
	case copyErr != nil:
		os.Remove(tmpName)
		return 0, copyErr
	case closeErr != nil:
		os.Remove(tmpName)
		return 0, closeErr
	case n > max:
		os.Remove(tmpName)
		return 0, ErrTooLarge
	}
	if err := os.Rename(tmpName, p); err != nil {
		os.Remove(tmpName)
		return 0, fmt.Errorf("filestore: rename: %w", err)
	}
	return n, nil
}

func (s *Store) Open(key string) (*os.File, error) {
	p, err := s.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(p)
}

// Remove удаляет файл; отсутствие файла ошибкой не считается.
func (s *Store) Remove(key string) error {
	p, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
