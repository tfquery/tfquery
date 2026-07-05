// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/apex/log"
	"gopkg.in/yaml.v3"
)

// Type is the in-memory representation of the loaded configuration.
//
// Fields:
//   - Source: absolute path of the YAML file loaded.
//   - Namespace: optional dot-prefixed keyspace used to prefer namespaced
//     lookups (e.g. "backend.s3.region").
//   - Data: raw key/value tree unmarshaled from YAML.
//
// Note: Data is intentionally kept as map[string]any to allow flexible shapes.
// Callers should use typed getters (GetString, GetInt) for convenience.
type Type struct {
	// Configuration data
	Data map[string]any

	// Metadata
	Namespace string
	Source    string
}

// Config holds the global, lazily-initialized configuration instance.
var Config Type

// init attempts to load configuration at process start. Errors are ignored so
// the application can still run without a config file; callers of getters will
// trigger a lazy reload when needed.
func init() {
	_, _ = Load()
}

// GetInt returns the integer value for the given dotted key path. A single
// defaultValue may be provided and is returned when the key is missing.
// YAML numbers may decode as int, int64, or float64; common cases are handled.
func GetInt(key string, defaultValue ...int) (int, error) {
	if len(Config.Data) == 0 {
		_, _ = Load()
	}

	val, err := Config.get(key)
	if err != nil && Config.Namespace != "" {
		val, err = Config.get(Config.Namespace + "." + key)
	}

	if err != nil {
		if len(defaultValue) == 1 {
			return defaultValue[0], nil
		}
		return 0, err
	}

	// YAML numbers may be unmarshaled as int/float64 depending on content.
	switch v := val.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	default:
		return 0, errors.New("value is not an int")
	}
}

// GetBool returns the boolean value for the given dotted key path. A single
// defaultValue may be provided and is returned when the key is missing.
// YAML booleans may decode as bool or string; common cases are handled.
func GetBool(key string, defaultValue ...bool) (bool, error) {
	if len(Config.Data) == 0 {
		_, _ = Load()
	}

	val, err := Config.get(key)
	if err != nil && Config.Namespace != "" {
		val, err = Config.get(Config.Namespace + "." + key)
	}

	if err != nil {
		if len(defaultValue) == 1 {
			return defaultValue[0], nil
		}
		return false, err
	}

	switch v := val.(type) {
	case bool:
		return v, nil
	case string:
		switch strings.ToLower(v) {
		case "true", "yes", "on", "1":
			return true, nil
		case "false", "no", "off", "0":
			return false, nil
		default:
			return false, fmt.Errorf("cannot parse %q as bool", v)
		}
	default:
		return false, errors.New("value is not a bool")
	}
}

// GetString returns the string value for the given dotted key path. If the key
// is not found and a single defaultValue is provided, the default is returned.
// Returns an error if the value exists but is not a string.
func GetString(key string, defaultValue ...string) (string, error) {
	if len(Config.Data) == 0 {
		_, _ = Load()
	}

	val, err := Config.get(key)
	if err != nil {
		if len(defaultValue) == 1 {
			return defaultValue[0], nil
		}
		return "", err
	}

	s, ok := val.(string)
	if !ok {
		return "", errors.New("value is not a string")
	}

	return s, nil
}

// GetStringSlice returns the string slice value for the given dotted key path.
// If the key is not found and a single default slice is provided, that default
// is returned. Returns an error if the value exists but is not a string slice.
func GetStringSlice(key string, defaultValue ...[]string) ([]string, error) {
	if len(Config.Data) == 0 {
		_, _ = Load()
	}

	val, err := Config.get(key)
	if err != nil && Config.Namespace != "" {
		val, err = Config.get(Config.Namespace + "." + key)
	}
	if err != nil {
		if len(defaultValue) == 1 {
			return defaultValue[0], nil
		}
		return nil, err
	}

	switch v := val.(type) {
	case []string:
		return v, nil
	case string:
		// Single string treated as single-element slice
		return []string{v}, nil
	case []any:
		result := make([]string, len(v))
		for i, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, errors.New("slice element is not a string")
			}
			result[i] = s
		}
		return result, nil
	default:
		return nil, errors.New("value is not a slice")
	}
}

// Load reads the YAML configuration file from the standard user config
// directory and populates the global Config. If cfgFilePath is provided in the
// future, it can be used to override the path selection (currently ignored).
// THINK Is there a use case for passing cfg file path?  --cfg= or some such?
//
// Returns the loaded Type or an error if the file could not be located or
// parsed.
func Load(cfgFilePath ...string) (Type, error) {
	path, err := getConfigFile()
	if err != nil {
		return Type{}, err
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return Type{}, err
	}

	var data map[string]any
	if err := yaml.Unmarshal(bytes, &data); err != nil {
		return Type{}, err
	}

	Config = Type{
		Source: path,
		Data:   data,
	}

	return Config, nil
}

// get traverses the configuration tree using a dotted key path (e.g.
// "backend.s3.bucket"). If Namespace is set, a namespaced candidate key is
// attempted first (Namespace + "." + kspec), then the unnamespaced key.
// Returns the raw value (any) if found.
func (cfg *Type) get(kspec string) (any, error) {
	if len(cfg.Data) == 0 {
		_, _ = Load(cfg.Source)
	}

	candidateKeys := []string{"", kspec}
	if cfg.Namespace != "" {
		candidateKeys[0] = cfg.Namespace + "." + kspec
	}

	for _, key := range candidateKeys {
		keys := strings.Split(key, ".")
		var current any = cfg.Data

		success := true
		for _, key := range keys {
			m, ok := current.(map[string]any)
			if !ok {
				success = false
				break
			}
			current, ok = m[key]
			if !ok {
				success = false
				break
			}
		}

		if success {
			return current, nil
		}
	}

	return nil, fmt.Errorf("no valid path found among: %v", candidateKeys)
}

// getConfigFile returns the absolute path to the YAML config file. If the
// TFQUERY_CFG_FILE environment variable is set, it is treated as the full path
// to the config file. Otherwise, the OS-specific user configuration directory
// returned by os.UserConfigDir is used with the filename "tfquery.yaml". The
// file must exist and not be a directory.
func getConfigFile() (string, error) {
	// Check for TFQUERY_CFG_FILE environment variable first
	if cfgPath := os.Getenv("TFQUERY_CFG_FILE"); cfgPath != "" {
		if fileInfo, err := os.Stat(filepath.Clean(cfgPath)); err == nil {
			if !fileInfo.IsDir() {
				log.Debugf("using config file from TFQUERY_CFG_FILE: %s", cfgPath)
				return cfgPath, nil
			}
			return "", fmt.Errorf("TFQUERY_CFG_FILE points to a directory: %s", cfgPath)
		}
		return "", fmt.Errorf("config file not found at TFQUERY_CFG_FILE path: %s", cfgPath)
	}

	// Fall back to user config directory
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	file := filepath.Join(dir, "tfquery.yaml")
	if fileInfo, err := os.Stat(file); err == nil {
		if !fileInfo.IsDir() {
			log.Debugf("using config file: %s", file)
			return file, nil
		}
	}

	return "", fmt.Errorf("no config file found in standard locations")
}
