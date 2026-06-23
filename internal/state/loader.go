// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/term"

	"github.com/apex/log"
	"github.com/urfave/cli/v3"

	"github.com/tfquery/tfquery/internal/backend"
)

// DecryptOpenTofuState decrypts an encrypted OpenTofu state file using the
// provided passphrase.
func DecryptOpenTofuState(stateData []byte, passphrase string) ([]byte, error) {
	var state struct {
		Meta struct {
			Key string `json:"key_provider.pbkdf2.mykey"`
		} `json:"meta"`
		EncryptedData string `json:"encrypted_data"`
	}

	if err := json.Unmarshal(stateData, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state: %w", err)
	}

	// Decode key provider config
	keyProviderConfig, err := base64.StdEncoding.DecodeString(state.Meta.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to decode key provider config: %w", err)
	}

	var kpConfig struct {
		Salt       string `json:"salt"`
		Iterations int    `json:"iterations"`
		HashFunc   string `json:"hash_function"`
		KeyLength  int    `json:"key_length"`
	}

	if err = json.Unmarshal(keyProviderConfig, &kpConfig); err != nil {
		return nil, fmt.Errorf("failed to parse key provider config: %w", err)
	}

	// Decode salt
	salt, err := base64.StdEncoding.DecodeString(kpConfig.Salt)
	if err != nil {
		return nil, fmt.Errorf("failed to decode salt: %w", err)
	}

	// Generate key using configured PBKDF2 parameters
	key := pbkdf2.Key(
		[]byte(passphrase),
		salt,
		kpConfig.Iterations,
		kpConfig.KeyLength,
		sha512.New,
	)

	// Decrypt the state data using the derived key
	return decryptState(state.EncryptedData, key)
}

// GetPassphrase prompts interactively for a passphrase without echoing input.
func GetPassphrase() (string, error) {
	var password []byte
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt)

	oldState, err := term.MakeRaw(int(syscall.Stdin))
	if err != nil {
		return "", err
	}
	defer term.Restore(int(syscall.Stdin), oldState) //nolint:errcheck

	fmt.Print("Enter passphrase: ")
	defer fmt.Print("\r")

loop:
	for {
		select {
		case <-signalChannel:
			fmt.Println("\nInterrupt received, exiting...")
			return "", fmt.Errorf("interrupted")
		default:
			var buf [1]byte
			n, readErr := syscall.Read(syscall.Stdin, buf[:])
			if readErr != nil || n == 0 {
				break loop
			}
			if buf[0] == '\n' || buf[0] == '\r' {
				break loop
			}
			if buf[0] == 127 || buf[0] == 8 { // Handle backspace
				if len(password) > 0 {
					password = password[:len(password)-1]
					fmt.Print("\b \b")
				}
			} else {
				password = append(password, buf[0])
				fmt.Print("*")
			}
		}
	}
	fmt.Println()
	return string(password), nil
}

// LoadStateData loads and optionally decrypts a state document from the
// detected backend at the provided rootDir.
func LoadStateData(ctx context.Context, cmd *cli.Command, rootDir string) (map[string]interface{}, error) {
	// Check to make sure the target directory looks like it might be a legit TF
	// workspace.
	tfConfigFile := fmt.Sprintf("%s/.terraform/terraform.tfstate", rootDir)
	if _, err := os.Stat(tfConfigFile); err != nil {
		return nil, fmt.Errorf("terraform config file not found: %s", tfConfigFile)
	}

	// Figure out what type of Backend we're in.
	be, err := backend.NewBackend(ctx, *cmd)
	if err != nil {
		log.Errorf("err: %v", err)
		return nil, err
	}

	// Get the state data.
	doc, err := be.State()
	if err != nil {
		log.Errorf("err: %v", err)
		return nil, err
	}

	// If the state is encrypted, there's a little more work to do.
	var jsonData map[string]interface{}
	if err := json.Unmarshal(doc, &jsonData); err == nil {
		if _, exists := jsonData["encrypted_data"]; exists {
			// First, look to the flag for passphrase value.
			passphrase := cmd.String("passphrase")

			// Issue 14 - Next look in env TF_VAR_passphrase and use it if found.
			if passphrase == "" {
				passphrase = os.Getenv("TF_VAR_passphrase")
			}

			// Finally, prompt for passphrase
			if passphrase == "" {
				passphrase, _ = GetPassphrase()
			}

			doc, err = DecryptOpenTofuState(doc, passphrase)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt: %w", err)
			}
		}
	}

	// Parse the state data as JSON
	var stateData map[string]interface{}
	if err := json.Unmarshal(doc, &stateData); err != nil {
		return nil, fmt.Errorf("failed to parse state JSON: %w", err)
	}

	return stateData, nil
}

func decryptState(encryptedData string, derivedKey []byte) ([]byte, error) {
	// Decode base64 data
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	// Create cipher directly with derived key
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce and ciphertext - no salt needed since key is already derived
	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf(
			"ciphertext too short: expected at least %d bytes, got %d",
			nonceSize,
			len(ciphertext),
		)
	}

	nonce := ciphertext[:nonceSize]
	encrypted := ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := aesGCM.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}
