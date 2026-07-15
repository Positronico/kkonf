package tui

import (
	"bytes"
	"fmt"

	"github.com/positronico/kkonf/v2/internal/config"
	"github.com/positronico/kkonf/v2/internal/models"
)

// Session holds the loaded kubeconfig and its persistence machinery. All
// screens share one Session and mutate Config through the models ops layer.
type Session struct {
	Path      string
	Config    *models.Config
	loader    *config.Loader
	writer    *config.Writer
	validator *config.Validator
	pristine  []byte
}

func NewSession(path string) (*Session, error) {
	s := &Session{
		Path:      path,
		loader:    config.NewLoader(path),
		writer:    config.NewWriter(path),
		validator: config.NewValidator(),
	}
	if err := s.Reload(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Session) Reload() error {
	cfg, err := s.loader.Load()
	if err != nil {
		return err
	}
	s.Config = cfg
	s.snapshot()
	return nil
}

func (s *Session) snapshot() {
	// MarshalConfig converts yaml.v3 panics into errors — Dirty runs on
	// every render, so it must never be able to crash the TUI.
	s.pristine, _ = config.MarshalConfig(s.Config)
}

// Dirty reports whether the in-memory config differs from the last
// loaded/saved state.
func (s *Session) Dirty() bool {
	current, err := config.MarshalConfig(s.Config)
	if err != nil {
		return true
	}
	return !bytes.Equal(current, s.pristine)
}

func (s *Session) Validate() *config.ValidationResult {
	return s.validator.Validate(s.Config)
}

// ExternalChange reports whether the file on disk changed since load/save.
func (s *Session) ExternalChange() bool {
	return config.ChangedSince(s.Path, s.loader.Fingerprint())
}

// ReloadError means the save itself succeeded (data is on disk) but the
// post-save re-read failed — callers must not present it as a failed save.
type ReloadError struct{ Err error }

func (e *ReloadError) Error() string { return fmt.Sprintf("saved, but reloading failed: %v", e.Err) }
func (e *ReloadError) Unwrap() error { return e.Err }

// Save writes the config, failing with config.ErrExternalChange if the file
// on disk was modified since load (checked under the write lock).
func (s *Session) Save() error {
	if err := s.writer.SaveGuarded(s.Config, s.loader.Fingerprint()); err != nil {
		return err
	}
	// Re-load to refresh the loader fingerprint and pristine snapshot.
	if err := s.Reload(); err != nil {
		return &ReloadError{Err: err}
	}
	return nil
}

// ForceSave overwrites the file even if it changed externally — only after
// the user has explicitly confirmed the overwrite.
func (s *Session) ForceSave() error {
	if err := s.writer.Save(s.Config); err != nil {
		return err
	}
	if err := s.Reload(); err != nil {
		return &ReloadError{Err: err}
	}
	return nil
}
