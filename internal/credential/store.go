// Package credential stores the local username/password pair and reads
// passwords from explicit sources.
package credential

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Credential is one portal username and plaintext password.
type Credential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Store persists one Credential.
type Store interface {
	// Load returns nil, nil when no credential has been saved.
	Load() (*Credential, error)
	Save(Credential) error
}

var _ Store = FileStore{}

// FileStore keeps the credential in a 0600 JSON file inside a 0700 directory.
// Writes are atomic and symlinks are rejected.
type FileStore struct {
	Path string
}

// DefaultPath is config.json under configDir when it is set, otherwise under
// the user configuration directory.
func DefaultPath(configDir string) (string, error) {
	if dir := strings.TrimSpace(configDir); dir != "" {
		return filepath.Join(dir, "config.json"), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "njulogin", "config.json"), nil
}

// Load reads and validates the saved credential.
func (s FileStore) Load() (*Credential, error) {
	path := s.Path
	if err := validatePermissions(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var saved Credential
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&saved); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse %s: trailing JSON data", path)
	}
	saved.Username = strings.TrimSpace(saved.Username)
	if saved.Username == "" {
		return nil, fmt.Errorf("invalid config %s: missing username", path)
	}
	if strings.TrimSpace(saved.Password) == "" {
		return nil, fmt.Errorf("invalid config %s: missing password", path)
	}
	return &saved, nil
}

// Save atomically replaces the credential file and repairs directory and file
// modes.
func (s FileStore) Save(value Credential) error {
	path := s.Path
	value.Username = strings.TrimSpace(value.Username)
	if value.Username == "" {
		return errors.New("cannot save config with empty username")
	}
	if strings.TrimSpace(value.Password) == "" {
		return errors.New("cannot save config with empty password")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := requireDirectory(dir); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("config %s must be a regular file, not a symlink or special file", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(dir, ".config.json.tmp-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := writeAndClose(temporary, data); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func writeAndClose(file *os.File, data []byte) error {
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func validatePermissions(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("config %s must be a regular file, not a symlink or special file", path)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		return fmt.Errorf("config %s has mode %04o; expected 0600", path, mode)
	}
	dir := filepath.Dir(path)
	if err := requireDirectory(dir); err != nil {
		return err
	}
	dirInfo, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if mode := dirInfo.Mode().Perm(); mode != 0o700 {
		return fmt.Errorf("config directory %s has mode %04o; expected 0700", dir, mode)
	}
	return nil
}

func requireDirectory(dir string) error {
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("config parent %s must be a directory, not a symlink or special file", dir)
	}
	return nil
}
