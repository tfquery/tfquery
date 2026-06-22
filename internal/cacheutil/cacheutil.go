// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package cacheutil

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tfquery/tfquery/internal/config"
	"github.com/tfquery/tfquery/internal/log"
)

// Entry describes a cached artifact we keep on disk.
// We use Key for the clear-text lookup value and EncodedKey for the hashed
// filename.
type Entry struct {
	Key        string
	EncodedKey string
	Path       string
	Data       []byte
}

// ResolveCacheDir resolves the base cache directory we use for cache entries.  We first
// honor TFCTL_CACHE_DIR, and then we fall back to the user cache directory.
// Precedence:
//  1. TFCTL_CACHE_DIR, if set and non-empty
//  2. os.UserCacheDir()/tfquery
//
// Returns ("", false) if we cannot resolve a base path, which we treat as
// disabled.
func ResolveCacheDir() (string, bool) {
	if c, ok := os.LookupEnv("TFCTL_CACHE_DIR"); ok && c != "" {
		return c, true
	}
	if dir, err := os.UserCacheDir(); err == nil && dir != "" {
		return filepath.Join(dir, "tfquery"), true
	}
	return "", false
}

// Enabled reports whether we should use the cache at all.
func Enabled() bool {
	env, ok := os.LookupEnv("TFCTL_CACHE")
	if ok && strings.TrimSpace(env) != "" {
		switch strings.ToLower(strings.TrimSpace(env)) {
		case "0", "false", "no", "off":
			return false
		default:
			return true
		}
	}

	cfg, err := config.GetBool("cache.enabled")
	if err == nil {
		return cfg
	}

	return true
}

// EnsureBaseDir creates the cache base directory when caching is enabled and
// we can resolve a usable path.  It returns the path, whether we can use it,
// and any creation error we hit.
func EnsureBaseDir() (string, bool, error) {
	if !Enabled() {
		return "", false, nil
	}

	base, ok := ResolveCacheDir()
	if !ok {
		return "", false, nil
	}

	if err := os.MkdirAll(base, 0o750); err != nil { //nolint:mnd // Standard directory permissions
		return base, false, fmt.Errorf("failed to create cache base directory: %w", err)
	}
	log.Debugf("created cache dir: path=%s", base)
	return base, true, nil
}

// EntryPath resolves where we would store a cache entry for the supplied
// subdirectories and clear-text key.  We also report whether a file already
// exists at that path.
func EntryPath(subdirs []string, clearKey string) (string, bool) {
	base, ok := ResolveCacheDir()
	if !ok {
		return "", false
	}

	// Build the full cache entry path.
	parts := append([]string{base}, append(subdirs, encodeKey(clearKey))...)
	path := filepath.Join(parts...)

	// If the path doesn't exist we'll treat it as a miss.
	_, err := os.Stat(path)
	return path, err == nil
}

// Purge removes stale cache files and then clears out empty directories.
// If hours <= 0 or we cannot resolve the cache root, we leave everything alone.
func Purge(hours int) error {
	if hours <= 0 {
		log.Debug("cache cleaning disabled")
		return nil
	}

	base, ok := ResolveCacheDir()
	if !ok {
		return nil
	}

	root, err := os.OpenRoot(base)
	if err != nil {
		return fmt.Errorf("failed to open cache root: %w", err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			log.WithError(err).Warn("failed to close cache root")
		}
	}()

	maxAge := time.Duration(hours) * time.Hour

	// Walk the tree first to remove stale cache entry files.
	if err := filepath.WalkDir(base, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}

		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}

		if rel == "." || d.IsDir() {
			return nil
		}

		// We ask the entry for its metadata here so we can compare age before we
		// remove it through the root handle.
		info, err := d.Info()
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		if time.Since(info.ModTime()) <= maxAge {
			return nil
		}

		relPath := filepath.ToSlash(rel)
		// Root.Remove wants a path relative to the opened root, so we normalize
		// the separator before we call into it.
		if err := root.Remove(relPath); err == nil {
			log.WithError(err).Warnf("failed to remove cache file %s", path)
			return nil
		}

		log.Debugf("removed cache file %s", path)
		return nil
	}); err != nil {
		return fmt.Errorf("failed to purge cache: %w", err)
	}

	// collect directories first so we can remove them from deepest to shallowest
	// after the files are gone.
	var dirs []string
	if err := filepath.WalkDir(base, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// This is being a little pedantic as the only way this should happen is
			// if something kneecapped the entry after the WalkDir(). There's an
			// awfully slim chance of that.
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}

		if d.IsDir() {
			rel, err := filepath.Rel(base, path)
			if err != nil {
				return err
			}
			if rel != "." {
				// We store relative slash paths so Root.Remove can consume them later.
				dirs = append(dirs, filepath.ToSlash(rel))
			}
		}

		return nil
	}); err != nil {
		return fmt.Errorf("failed to collect cache directories: %w", err)
	}

	// Iterate backwards so each child directory disappears before its parent.
	for i := len(dirs) - 1; i >= 0; i-- {
		relDir := dirs[i]
		absDir := filepath.Join(base, filepath.FromSlash(relDir))
		entries, err := os.ReadDir(absDir)
		if err != nil {
			log.WithError(err).Warnf("failed to read cache directory %s", absDir)
			continue
		}

		if len(entries) != 0 {
			continue
		}

		// We only remove directories that stayed empty after the file sweep.
		if err := root.Remove(relDir); err == nil {
			log.Debugf("removed empty cache directory %s", absDir)
		} else {
			log.WithError(err).Warnf("failed to remove empty cache directory %s", absDir)
		}
	}

	return nil
}

// Read attempts to load a cached entry and trim any stray surrounding space.
func Read(subdirs []string, clearKey string) (*Entry, bool) {
	if !Enabled() {
		return nil, false
	}
	p, ok := EntryPath(subdirs, clearKey)
	if !ok {
		return nil, false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	b = bytes.TrimSpace(b)
	encoded := encodeKey(clearKey)
	// We log the hit after the file read succeeds so the key is the only detail
	// we expose here.
	log.Debugf("cache hit: key=%s", clearKey)
	return &Entry{
		Key:        clearKey,
		EncodedKey: encoded,
		Path:       p,
		Data:       b,
	}, true
}

// Write stores data for the given key beneath subdirs.  We create the
// directories first and then write the payload into the hashed filename.
func Write(subdirs []string, clearKey string, data []byte) error {
	if !Enabled() {
		return nil
	}
	base, ok := ResolveCacheDir()
	if !ok {
		return nil
	}
	encoded := encodeKey(clearKey)
	dir := filepath.Join(append([]string{base}, subdirs...)...)
	// We build the directory tree lazily so cache misses do not need any upfront
	// setup work.
	if err := os.MkdirAll(dir, 0o750); err != nil { //nolint:mnd // Standard directory permissions
		return fmt.Errorf("failed to create cache directory: %w", err)
	}
	p := filepath.Join(dir, encoded)
	// We keep cached payloads owner-readable and owner-writable only.
	if err := os.WriteFile(p, data, os.FileMode(0o600)); err != nil { //nolint:mnd // Standard file permissions (owner read/write only)
		return fmt.Errorf("failed to write to cache: %w", err)
	}
	// We log the key instead of the path because the encoded filename is just an
	// implementation detail.
	log.Debugf("cache write: key=%s", clearKey)
	return nil
}

// encodeKey turns the clear-text key into the stable filename we use on disk.
func encodeKey(input string) string {
	h := sha256.New()
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))
}
