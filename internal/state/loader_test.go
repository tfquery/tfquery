// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/pbkdf2"
)

// TestDecryptOpenTofuState_ValidEncryption verifies successful decryption of
// a properly encrypted OpenTofu state file with valid passphrase.
func TestDecryptOpenTofuState_ValidEncryption(t *testing.T) {
	t.Parallel()
	passphrase := "test-passphrase"
	plaintext := []byte(`{"version":4,"terraform_version":"1.5.0"}`)

	// Create properly encrypted state file
	stateData := createEncryptedStateFile(t, plaintext, passphrase)

	// Decrypt
	result, err := DecryptOpenTofuState(stateData, passphrase)

	assert.NoError(t, err)
	assert.Equal(t, plaintext, result)
}

// TestDecryptOpenTofuState_WrongPassphrase verifies that decryption fails
// with wrong passphrase.
func TestDecryptOpenTofuState_WrongPassphrase(t *testing.T) {
	t.Parallel()
	passphrase := "correct-passphrase"
	plaintext := []byte(`{"version":4}`)

	stateData := createEncryptedStateFile(t, plaintext, passphrase)

	// Try to decrypt with wrong passphrase
	_, err := DecryptOpenTofuState(stateData, "wrong-passphrase")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decrypt")
}

// TestDecryptOpenTofuState_InvalidJSON verifies that invalid JSON state
// returns error.
func TestDecryptOpenTofuState_InvalidJSON(t *testing.T) {
	t.Parallel()
	invalidJSON := []byte("not valid json")

	result, err := DecryptOpenTofuState(invalidJSON, "passphrase")

	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestDecryptOpenTofuState_MissingEncryptedData verifies error when
// encrypted_data field is missing.
func TestDecryptOpenTofuState_MissingEncryptedData(t *testing.T) {
	t.Parallel()
	stateJSON := map[string]any{
		"meta": map[string]any{
			"key_provider.pbkdf2.mykey": "dGVzdA==",
		},
	}

	stateData, err := json.Marshal(stateJSON)
	require.NoError(t, err)

	result, err := DecryptOpenTofuState(stateData, "passphrase")

	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestDecryptOpenTofuState_InvalidBase64Key verifies error when key provider
// config is not valid base64.
func TestDecryptOpenTofuState_InvalidBase64Key(t *testing.T) {
	t.Parallel()
	stateJSON := map[string]any{
		"meta": map[string]any{
			"key_provider.pbkdf2.mykey": "not-valid-base64!@#$",
		},
		"encrypted_data": "dGVzdA==",
	}

	stateData, err := json.Marshal(stateJSON)
	require.NoError(t, err)

	result, err := DecryptOpenTofuState(stateData, "passphrase")

	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestDecryptOpenTofuState_InvalidKeyProviderConfig verifies error when key
// provider config JSON is invalid.
func TestDecryptOpenTofuState_InvalidKeyProviderConfig(t *testing.T) {
	t.Parallel()
	stateJSON := map[string]any{
		"meta": map[string]any{
			"key_provider.pbkdf2.mykey": base64.StdEncoding.EncodeToString(
				[]byte("invalid json"),
			),
		},
		"encrypted_data": "dGVzdA==",
	}

	stateData, err := json.Marshal(stateJSON)
	require.NoError(t, err)

	result, err := DecryptOpenTofuState(stateData, "passphrase")

	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestDecryptOpenTofuState_InvalidSaltBase64 verifies error when salt is
// not valid base64.
func TestDecryptOpenTofuState_InvalidSaltBase64(t *testing.T) {
	t.Parallel()
	kpConfig := map[string]any{
		"salt":          "not-valid-base64!@#$",
		"iterations":    200000,
		"hash_function": "sha512",
		"key_length":    32,
	}

	kpConfigJSON, err := json.Marshal(kpConfig)
	require.NoError(t, err)

	stateJSON := map[string]any{
		"meta": map[string]any{
			"key_provider.pbkdf2.mykey": base64.StdEncoding.EncodeToString(
				kpConfigJSON,
			),
		},
		"encrypted_data": "dGVzdA==",
	}

	stateData, err := json.Marshal(stateJSON)
	require.NoError(t, err)

	result, err := DecryptOpenTofuState(stateData, "passphrase")

	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestDecryptOpenTofuState_InvalidEncryptedDataBase64 verifies error when
// encrypted data is not valid base64.
func TestDecryptOpenTofuState_InvalidEncryptedDataBase64(t *testing.T) {
	t.Parallel()
	kpConfig := map[string]any{
		"salt":          base64.StdEncoding.EncodeToString([]byte("salt")),
		"iterations":    200000,
		"hash_function": "sha512",
		"key_length":    32,
	}

	kpConfigJSON, err := json.Marshal(kpConfig)
	require.NoError(t, err)

	stateJSON := map[string]any{
		"meta": map[string]any{
			"key_provider.pbkdf2.mykey": base64.StdEncoding.EncodeToString(
				kpConfigJSON,
			),
		},
		"encrypted_data": "not-valid-base64!@#$",
	}

	stateData, err := json.Marshal(stateJSON)
	require.NoError(t, err)

	result, err := DecryptOpenTofuState(stateData, "passphrase")

	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestDecryptOpenTofuState_CorruptedCiphertext verifies error when
// ciphertext is corrupted (wrong nonce/data).
func TestDecryptOpenTofuState_CorruptedCiphertext(t *testing.T) {
	t.Parallel()
	passphrase := "test-passphrase"
	plaintext := []byte(`{"version":4}`)

	stateData := createEncryptedStateFile(t, plaintext, passphrase)

	// Parse and corrupt the encrypted_data
	var state struct {
		Meta struct {
			Key string `json:"key_provider.pbkdf2.mykey"`
		} `json:"meta"`
		EncryptedData string `json:"encrypted_data"`
	}

	err := json.Unmarshal(stateData, &state)
	require.NoError(t, err)

	// Corrupt by truncating
	state.EncryptedData = state.EncryptedData[:len(state.EncryptedData)-10]

	corruptedData, err := json.Marshal(state)
	require.NoError(t, err)

	result, err := DecryptOpenTofuState(corruptedData, passphrase)

	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestDecryptOpenTofuState_EmptyPlaintext verifies decryption of empty
// plaintext returns nil (GCM.Open returns nil for empty plaintext).
func TestDecryptOpenTofuState_EmptyPlaintext(t *testing.T) {
	t.Parallel()
	passphrase := "test-passphrase"
	plaintext := []byte("")

	stateData := createEncryptedStateFile(t, plaintext, passphrase)

	result, err := DecryptOpenTofuState(stateData, passphrase)

	assert.NoError(t, err)
	// GCM.Open returns nil for empty plaintext, not []byte{}
	assert.Nil(t, result)
}

// TestDecryptOpenTofuState_LargePlaintext verifies decryption of large
// plaintext.
func TestDecryptOpenTofuState_LargePlaintext(t *testing.T) {
	t.Parallel()
	passphrase := "test-passphrase"
	plaintext := bytes.Repeat([]byte("x"), 10000)

	stateData := createEncryptedStateFile(t, plaintext, passphrase)

	result, err := DecryptOpenTofuState(stateData, passphrase)

	assert.NoError(t, err)
	assert.Equal(t, plaintext, result)
}

// TestDecryptOpenTofuState_SpecialCharactersPassphrase verifies decryption
// with special characters in passphrase.
func TestDecryptOpenTofuState_SpecialCharactersPassphrase(t *testing.T) {
	t.Parallel()
	passphrase := `test!@#$%^&*()_+-=[]{}|;:'",.<>?/\~` + "`"
	plaintext := []byte(`{"version":4}`)

	stateData := createEncryptedStateFile(t, plaintext, passphrase)

	result, err := DecryptOpenTofuState(stateData, passphrase)

	assert.NoError(t, err)
	assert.Equal(t, plaintext, result)
}

// TestDecryptOpenTofuState_UnicodePassphrase verifies decryption with
// unicode characters in passphrase.
func TestDecryptOpenTofuState_UnicodePassphrase(t *testing.T) {
	t.Parallel()
	passphrase := "测试密码🔐🔑" //nolint:gosec
	plaintext := []byte(`{"version":4}`)

	stateData := createEncryptedStateFile(t, plaintext, passphrase)

	result, err := DecryptOpenTofuState(stateData, passphrase)

	assert.NoError(t, err)
	assert.Equal(t, plaintext, result)
}

// TestDecryptOpenTofuState_LongPassphrase verifies decryption with very long
// passphrase.
func TestDecryptOpenTofuState_LongPassphrase(t *testing.T) {
	t.Parallel()
	passphrase := string(bytes.Repeat([]byte("a"), 1000))
	plaintext := []byte(`{"version":4}`)

	stateData := createEncryptedStateFile(t, plaintext, passphrase)

	result, err := DecryptOpenTofuState(stateData, passphrase)

	assert.NoError(t, err)
	assert.Equal(t, plaintext, result)
}

// createEncryptedStateFile is a helper that creates a properly encrypted
// OpenTofu state file for testing.
func createEncryptedStateFile(
	t *testing.T,
	plaintext []byte,
	passphrase string,
) []byte {
	// Create key provider config
	salt := []byte("test-salt-12345")
	iterations := 200000
	hashFunc := sha512.New

	key := pbkdf2.Key(
		[]byte(passphrase),
		salt,
		iterations,
		32, // key length for AES-256
		hashFunc,
	)

	// Encrypt the plaintext
	block, err := aes.NewCipher(key)
	require.NoError(t, err)

	aesGCM, err := cipher.NewGCM(block)
	require.NoError(t, err)

	nonce := make([]byte, aesGCM.NonceSize())
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)

	// Create key provider config JSON
	kpConfig := map[string]any{
		"salt":          base64.StdEncoding.EncodeToString(salt),
		"iterations":    iterations,
		"hash_function": "sha512",
		"key_length":    32,
	}

	kpConfigJSON, err := json.Marshal(kpConfig)
	require.NoError(t, err)

	// Create state JSON
	state := map[string]any{
		"meta": map[string]any{
			"key_provider.pbkdf2.mykey": base64.StdEncoding.EncodeToString(
				kpConfigJSON,
			),
		},
		"encrypted_data": base64.StdEncoding.EncodeToString(ciphertext),
	}

	stateJSON, err := json.Marshal(state)
	require.NoError(t, err)

	return stateJSON
}

// TestDecryptState_ValidDecryption verifies the private decryptState
// function works correctly.
func TestDecryptState_ValidDecryption(t *testing.T) {
	t.Parallel()
	passphrase := "test-passphrase"
	plaintext := []byte(`{"resources":[]}`)
	salt := []byte("test-salt-12345")

	key := pbkdf2.Key(
		[]byte(passphrase),
		salt,
		200000,
		32,
		sha512.New,
	)

	// Encrypt
	block, err := aes.NewCipher(key)
	require.NoError(t, err)

	aesGCM, err := cipher.NewGCM(block)
	require.NoError(t, err)

	nonce := make([]byte, aesGCM.NonceSize())
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)

	encryptedData := base64.StdEncoding.EncodeToString(ciphertext)

	// Decrypt using the private function
	result, err := decryptState(encryptedData, key)

	assert.NoError(t, err)
	assert.Equal(t, plaintext, result)
}

// TestDecryptState_InvalidBase64 verifies decryptState rejects invalid
// base64.
func TestDecryptState_InvalidBase64(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)

	result, err := decryptState("not-valid-base64!@#$", key)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "decode")
}

// TestDecryptState_InvalidCipherKey verifies decryptState errors with
// invalid key size.
func TestDecryptState_InvalidCipherKey(t *testing.T) {
	t.Parallel()
	key := make([]byte, 15) // Invalid: must be 16, 24, or 32

	result, err := decryptState(
		base64.StdEncoding.EncodeToString([]byte("test")),
		key,
	)

	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestDecryptState_ShortCiphertext verifies decryptState errors gracefully
// when ciphertext is shorter than nonce size (bounds-check protection).
func TestDecryptState_ShortCiphertext(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)

	// Create ciphertext shorter than GCM nonce size (12 bytes)
	shortData := []byte("x")
	encryptedData := base64.StdEncoding.EncodeToString(shortData)

	result, err := decryptState(encryptedData, key)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "ciphertext too short")
}
