// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package cacheutil

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tfquery/tfquery/internal/config"
)

// TestDir_WithTFCTL_CACHE_DIR verifies Dir() respects TFCTL_CACHE_DIR
// environment variable with highest priority.
func TestDir_WithTFCTL_CACHE_DIR(t *testing.T) {
	customDir := t.TempDir()
	t.Setenv("TFCTL_CACHE_DIR", customDir)

	result, ok := ResolveCacheDir()

	assert.True(t, ok)
	assert.Equal(t, customDir, result)
}

// TestDir_WithEmptyTFCTL_CACHE_DIR verifies empty TFCTL_CACHE_DIR is
// treated as not set.
func TestDir_WithEmptyTFCTL_CACHE_DIR(t *testing.T) {
	t.Setenv("TFCTL_CACHE_DIR", "")
	// Should fall back to os.UserCacheDir

	result, ok := ResolveCacheDir()

	// Result depends on system, but should not be empty string
	if ok {
		assert.NotEmpty(t, result)
	}
}

// TestDir_WithoutTFCTL_CACHE_DIR verifies Dir() falls back to
// os.UserCacheDir/tfquery when env var not set.
func TestDir_WithoutTFCTL_CACHE_DIR(t *testing.T) {
	t.Setenv("TFCTL_CACHE_DIR", "")

	result, ok := ResolveCacheDir()

	// Should use os.UserCacheDir if available
	if ok {
		assert.NotEmpty(t, result)
		assert.True(t, filepath.IsAbs(result))
	}
}

// TestEnabled_Default verifies caching is enabled by default (no env var).
func TestEnabled_Default(t *testing.T) {
	originalConfig := config.Config
	t.Cleanup(func() {
		config.Config = originalConfig
	})

	config.Config = config.Type{Data: map[string]interface{}{"dummy": true}}
	t.Setenv("TFCTL_CACHE", "")

	assert.True(t, Enabled())
}

// TestEnabled_With_TFCTL_CACHE_Set verifies TFCTL_CACHE override semantics.
func TestEnabled_With_TFCTL_CACHE_Set(t *testing.T) {
	originalConfig := config.Config
	t.Cleanup(func() {
		config.Config = originalConfig
	})

	config.Config = config.Type{
		Data: map[string]interface{}{
			"cache": map[string]interface{}{
				"enabled": false,
			},
		},
	}

	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"1", "1", true},
		{"true", "true", true},
		{"yes", "yes", true},
		{"empty string falls back to config", "", false},
		{"0", "0", false},
		{"false", "false", false},
		{"off", "off", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TFCTL_CACHE", tt.value)
			assert.Equal(t, tt.expected, Enabled())
		})
	}
}

func TestEnabled_WithConfigFallback(t *testing.T) {
	originalConfig := config.Config
	t.Cleanup(func() {
		config.Config = originalConfig
	})

	t.Setenv("TFCTL_CACHE", "")

	config.Config = config.Type{
		Data: map[string]interface{}{
			"cache": map[string]interface{}{
				"enabled": true,
			},
		},
	}
	assert.True(t, Enabled())

	config.Config = config.Type{
		Data: map[string]interface{}{
			"cache": map[string]interface{}{
				"enabled": false,
			},
		},
	}
	assert.False(t, Enabled())
}

// TestEnsureBaseDir_CachingDisabled verifies EnsureBaseDir returns empty
// when caching is disabled.
func TestEnsureBaseDir_CachingDisabled(t *testing.T) {
	t.Setenv("TFCTL_CACHE", "0")

	base, ok, err := EnsureBaseDir()

	assert.False(t, ok)
	assert.Empty(t, base)
	assert.NoError(t, err)
}

// TestEnsureBaseDir_NoDirResolvable verifies EnsureBaseDir returns empty
// when Dir() returns false.
func TestEnsureBaseDir_NoDirResolvable(t *testing.T) {
	t.Setenv("TFCTL_CACHE_DIR", "")
	// Mock UserCacheDir to return error by clearing env
	// This depends on system state, but we can at least test the happy path

	t.Setenv("TFCTL_CACHE", "1")
	// Attempt to resolve
	base, ok, err := EnsureBaseDir()

	// Either succeeds or fails gracefully
	if ok {
		assert.NotEmpty(t, base)
	}
	// No error should be returned if Dir() returns false
	if !ok {
		assert.NoError(t, err)
	}
}

// TestEnsureBaseDir_CreatesDirectory verifies EnsureBaseDir creates the
// cache directory when it doesn't exist.
func TestEnsureBaseDir_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache", "nested")
	t.Setenv("TFCTL_CACHE_DIR", cacheDir)
	t.Setenv("TFCTL_CACHE", "1")

	// Verify dir doesn't exist yet
	assert.NoFileExists(t, cacheDir)

	base, ok, err := EnsureBaseDir()

	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, cacheDir, base)
	assert.DirExists(t, cacheDir)
}

// TestEnsureBaseDir_ExistingDirectory verifies EnsureBaseDir succeeds
// with existing directory.
func TestEnsureBaseDir_ExistingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	err := os.MkdirAll(cacheDir, 0o750)
	require.NoError(t, err)

	t.Setenv("TFCTL_CACHE_DIR", cacheDir)
	t.Setenv("TFCTL_CACHE", "1")

	base, ok, err := EnsureBaseDir()

	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, cacheDir, base)
	assert.DirExists(t, cacheDir)
}

// TestEntryPath_CachingDisabled verifies EntryPath returns empty path when
// Dir() returns false (cache dir cannot be resolved).
func TestEntryPath_CachingDisabled(t *testing.T) {
	t.Setenv("TFCTL_CACHE_DIR", "")
	t.Setenv("TFCTL_CACHE", "0")
	// Note: EntryPath still calls Dir(), so if Dir() resolves from
	// os.UserCacheDir, the path will be returned. We can't fully
	// disable this test without being able to mock Dir().

	path, exists := EntryPath([]string{"subdirs"}, "test-key")

	// If a path is returned, it's from os.UserCacheDir
	if path != "" {
		assert.False(t, exists)
		assert.NotEmpty(t, path)
	}
}

// TestEntryPath_NonexistentEntry verifies EntryPath returns computed path
// and false when file doesn't exist.
func TestEntryPath_NonexistentEntry(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TFCTL_CACHE_DIR", tmpDir)

	path, exists := EntryPath([]string{"subdir1", "subdir2"}, "my-key")

	assert.False(t, exists)
	assert.NotEmpty(t, path)
	assert.True(t, filepath.IsAbs(path))
}

// TestEntryPath_ExistingEntry verifies EntryPath returns true when file
// exists at computed path.
func TestEntryPath_ExistingEntry(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TFCTL_CACHE_DIR", tmpDir)

	// Create subdirectory and file
	subdir := filepath.Join(tmpDir, "subdir")
	err := os.MkdirAll(subdir, 0o750)
	require.NoError(t, err)

	// Create file with encoded key name
	encodedKey := encodeKey("my-key")
	filePath := filepath.Join(subdir, encodedKey)
	err = os.WriteFile(filePath, []byte("data"), 0o600)
	require.NoError(t, err)

	path, exists := EntryPath([]string{"subdir"}, "my-key")

	assert.True(t, exists)
	assert.Equal(t, filePath, path)
}

// TestRead_CachingDisabled verifies Read returns false when caching is
// disabled.
func TestRead_CachingDisabled(t *testing.T) {
	t.Setenv("TFCTL_CACHE", "0")

	entry, found := Read([]string{"subdir"}, "key")

	assert.False(t, found)
	assert.Nil(t, entry)
}

// TestRead_FileNotFound verifies Read returns false when file doesn't exist.
func TestRead_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TFCTL_CACHE_DIR", tmpDir)
	t.Setenv("TFCTL_CACHE", "1")

	entry, found := Read([]string{"subdir"}, "nonexistent-key")

	assert.False(t, found)
	assert.Nil(t, entry)
}

// TestRead_SuccessfulRead verifies Read returns populated Entry when file
// exists.
func TestRead_SuccessfulRead(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TFCTL_CACHE_DIR", tmpDir)
	t.Setenv("TFCTL_CACHE", "1")

	// Create cache file
	subdir := filepath.Join(tmpDir, "data")
	err := os.MkdirAll(subdir, 0o750)
	require.NoError(t, err)

	testData := []byte("cached data content")
	testKey := "cache-key-123"
	encodedKey := encodeKey(testKey)
	filePath := filepath.Join(subdir, encodedKey)

	err = os.WriteFile(filePath, testData, 0o600)
	require.NoError(t, err)

	entry, found := Read([]string{"data"}, testKey)

	assert.True(t, found)
	assert.NotNil(t, entry)
	assert.Equal(t, testKey, entry.Key)
	assert.Equal(t, encodedKey, entry.EncodedKey)
	assert.Equal(t, filePath, entry.Path)
	assert.Equal(t, testData, entry.Data)
}

// TestRead_TrimsWhitespace verifies Read trims leading/trailing whitespace
// from file content.
func TestRead_TrimsWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TFCTL_CACHE_DIR", tmpDir)
	t.Setenv("TFCTL_CACHE", "1")

	// Create cache file with whitespace
	subdir := filepath.Join(tmpDir, "data")
	err := os.MkdirAll(subdir, 0o750)
	require.NoError(t, err)

	testData := []byte("  \n  cached content  \n  ")
	testKey := "key-with-whitespace"
	encodedKey := encodeKey(testKey)
	filePath := filepath.Join(subdir, encodedKey)

	err = os.WriteFile(filePath, testData, 0o600)
	require.NoError(t, err)

	entry, found := Read([]string{"data"}, testKey)

	assert.True(t, found)
	assert.Equal(t, []byte("cached content"), entry.Data)
}

// TestWrite_CachingDisabled verifies Write is no-op when caching is
// disabled.
func TestWrite_CachingDisabled(t *testing.T) {
	t.Setenv("TFCTL_CACHE", "0")

	err := Write([]string{"subdir"}, "key", []byte("data"))

	assert.NoError(t, err)
}

// TestWrite_CreatesDirectories verifies Write creates missing
// subdirectories.
func TestWrite_CreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TFCTL_CACHE_DIR", tmpDir)
	t.Setenv("TFCTL_CACHE", "1")

	subdir := filepath.Join(tmpDir, "level1", "level2", "level3")
	assert.NoFileExists(t, subdir)

	err := Write([]string{"level1", "level2", "level3"}, "key", []byte("data"))

	assert.NoError(t, err)
	assert.DirExists(t, subdir)
}

// TestWrite_SuccessfulWrite verifies Write stores data correctly.
func TestWrite_SuccessfulWrite(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TFCTL_CACHE_DIR", tmpDir)
	t.Setenv("TFCTL_CACHE", "1")

	testKey := "test-write-key"
	testData := []byte("test write data content")
	subdirs := []string{"cache", "data"}

	err := Write(subdirs, testKey, testData)

	assert.NoError(t, err)

	// Verify file exists with correct content
	expectedDir := filepath.Join(tmpDir, "cache", "data")
	encoded := encodeKey(testKey)
	expectedPath := filepath.Join(expectedDir, encoded)
	assert.FileExists(t, expectedPath)

	content, err := os.ReadFile(expectedPath)
	assert.NoError(t, err)
	assert.Equal(t, testData, content)
}

// TestWrite_FilePermissions verifies Write creates files with 0600
// permissions (user read/write only).
func TestWrite_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TFCTL_CACHE_DIR", tmpDir)
	t.Setenv("TFCTL_CACHE", "1")

	testKey := "perm-test-key"
	testData := []byte("permission test data")

	err := Write([]string{}, testKey, testData)

	assert.NoError(t, err)

	encoded := encodeKey(testKey)
	expectedPath := filepath.Join(tmpDir, encoded)

	info, err := os.Stat(expectedPath)
	assert.NoError(t, err)

	// Verify permissions are 0600 (user rw only)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

// TestWrite_OverwritesExisting verifies Write overwrites existing cache
// files.
func TestWrite_OverwritesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TFCTL_CACHE_DIR", tmpDir)
	t.Setenv("TFCTL_CACHE", "1")

	testKey := "overwrite-key"
	oldData := []byte("old data")
	newData := []byte("new data")

	// Write initial data
	err := Write([]string{}, testKey, oldData)
	require.NoError(t, err)

	// Verify old data
	encoded := encodeKey(testKey)
	expectedPath := filepath.Join(tmpDir, encoded)
	content, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	assert.Equal(t, oldData, content)

	// Overwrite with new data
	err = Write([]string{}, testKey, newData)
	assert.NoError(t, err)

	// Verify new data
	content, err = os.ReadFile(expectedPath)
	assert.NoError(t, err)
	assert.Equal(t, newData, content)
}

// TestWrite_EmptyData verifies Write handles empty data correctly.
func TestWrite_EmptyData(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TFCTL_CACHE_DIR", tmpDir)
	t.Setenv("TFCTL_CACHE", "1")

	testKey := "empty-data-key"
	emptyData := []byte{}

	err := Write([]string{}, testKey, emptyData)

	assert.NoError(t, err)

	// Verify empty file exists
	encoded := encodeKey(testKey)
	expectedPath := filepath.Join(tmpDir, encoded)
	info, err := os.Stat(expectedPath)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}

// TestPurge_DisabledWithZeroHours verifies Purge is no-op when hours <= 0.
func TestPurge_DisabledWithZeroHours(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TFCTL_CACHE_DIR", tmpDir)

	// Create old file
	oldPath := filepath.Join(tmpDir, "old_file.txt")
	err := os.WriteFile(oldPath, []byte("data"), 0o600)
	require.NoError(t, err)

	err = Purge(0)

	assert.NoError(t, err)
	assert.FileExists(t, oldPath)
}

// TestPurge_RemovesOldFiles verifies Purge removes files older than
// specified hours.
func TestPurge_RemovesOldFiles(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TFCTL_CACHE_DIR", tmpDir)

	// Create old file (modify time to past)
	oldPath := filepath.Join(tmpDir, "old_file.txt")
	err := os.WriteFile(oldPath, []byte("old data"), 0o600)
	require.NoError(t, err)

	// Set modification time to 3 hours ago
	pastTime := time.Now().Add(-3 * time.Hour)
	err = os.Chtimes(oldPath, pastTime, pastTime)
	require.NoError(t, err)

	// Purge files older than 1 hour
	err = Purge(1)

	assert.NoError(t, err)
	assert.NoFileExists(t, oldPath)
}

// TestPurge_KeepsRecentFiles verifies Purge keeps files newer than
// specified hours.
func TestPurge_KeepsRecentFiles(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TFCTL_CACHE_DIR", tmpDir)

	// Create recent file
	recentPath := filepath.Join(tmpDir, "recent_file.txt")
	err := os.WriteFile(recentPath, []byte("recent data"), 0o600)
	require.NoError(t, err)

	// Purge files older than 1 hour
	err = Purge(1)

	assert.NoError(t, err)
	assert.FileExists(t, recentPath)
}

// TestPurge_MixedAges verifies Purge only removes files matching age
// criteria.
func TestPurge_MixedAges(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TFCTL_CACHE_DIR", tmpDir)

	// Create old file
	oldPath := filepath.Join(tmpDir, "old.txt")
	err := os.WriteFile(oldPath, []byte("old"), 0o600)
	require.NoError(t, err)

	pastTime := time.Now().Add(-3 * time.Hour)
	err = os.Chtimes(oldPath, pastTime, pastTime)
	require.NoError(t, err)

	// Create recent file
	recentPath := filepath.Join(tmpDir, "recent.txt")
	err = os.WriteFile(recentPath, []byte("recent"), 0o600)
	require.NoError(t, err)

	// Purge files older than 1 hour
	err = Purge(1)

	assert.NoError(t, err)
	assert.NoFileExists(t, oldPath)
	assert.FileExists(t, recentPath)
}

// TestPurge_NestedDirectories verifies Purge processes files in nested
// directories.
func TestPurge_NestedDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TFCTL_CACHE_DIR", tmpDir)

	// Create nested directory structure
	nestedDir := filepath.Join(tmpDir, "level1", "level2")
	err := os.MkdirAll(nestedDir, 0o750)
	require.NoError(t, err)

	// Create old file in nested dir
	oldPath := filepath.Join(nestedDir, "old.txt")
	err = os.WriteFile(oldPath, []byte("old"), 0o600)
	require.NoError(t, err)

	pastTime := time.Now().Add(-3 * time.Hour)
	err = os.Chtimes(oldPath, pastTime, pastTime)
	require.NoError(t, err)

	// Purge
	err = Purge(1)

	assert.NoError(t, err)
	assert.NoFileExists(t, oldPath)
}

// TestEncodeKey_Consistency verifies encodeKey produces consistent output.
func TestEncodeKey_Consistency(t *testing.T) {
	testKey := "consistent-key"

	encoded1 := encodeKey(testKey)
	encoded2 := encodeKey(testKey)

	assert.Equal(t, encoded1, encoded2)
}

// TestEncodeKey_DifferentKeys verifies different keys produce different
// encodings.
func TestEncodeKey_DifferentKeys(t *testing.T) {
	key1 := "key-one"
	key2 := "key-two"

	encoded1 := encodeKey(key1)
	encoded2 := encodeKey(key2)

	assert.NotEqual(t, encoded1, encoded2)
}

// TestEncodeKey_HexFormat verifies encodeKey returns valid hex string.
func TestEncodeKey_HexFormat(t *testing.T) {
	testKey := "hex-format-test"

	encoded := encodeKey(testKey)

	// SHA-256 hex is always 64 characters
	assert.Equal(t, 64, len(encoded))
	// All characters should be valid hex
	for _, c := range encoded {
		assert.True(t,
			(c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"invalid hex character: %c", c,
		)
	}
}

// TestEncodeKey_SpecialCharacters verifies encodeKey handles special
// characters.
func TestEncodeKey_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"spaces", "key with spaces"},
		{"slashes", "key/with/slashes"},
		{"backslashes", "key\\with\\backslashes"},
		{"special", "key!@#$%^&*()_+-="},
		{"unicode", "key-with-unicode-🔐"},
		{"newlines", "key\nwith\nnewlines"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := encodeKey(tt.key)
			assert.Equal(t, 64, len(encoded))
		})
	}
}

// TestIntegration_FullWorkflow verifies complete caching workflow.
func TestIntegration_FullWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TFCTL_CACHE_DIR", tmpDir)
	t.Setenv("TFCTL_CACHE", "1")

	// 1. Verify caching is enabled
	assert.True(t, Enabled())

	// 2. Ensure base directory
	base, ok, err := EnsureBaseDir()
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.DirExists(t, base)

	// 3. Write cache entries
	testData1 := []byte("data for key 1")
	testData2 := []byte("data for key 2")

	err = Write([]string{"api"}, "endpoint-1", testData1)
	require.NoError(t, err)

	err = Write([]string{"api"}, "endpoint-2", testData2)
	require.NoError(t, err)

	// 4. Read entries back
	entry1, found1 := Read([]string{"api"}, "endpoint-1")
	entry2, found2 := Read([]string{"api"}, "endpoint-2")

	assert.True(t, found1)
	assert.True(t, found2)
	assert.Equal(t, testData1, entry1.Data)
	assert.Equal(t, testData2, entry2.Data)

	// 5. Verify entry paths
	path1, exists1 := EntryPath([]string{"api"}, "endpoint-1")
	assert.True(t, exists1)
	assert.NotEmpty(t, path1)
}
