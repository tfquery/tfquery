// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0
// no-cloc

// Package config provides loading and typed accessors for tfquery's user
// configuration. The configuration is expected to be a YAML document located
// in the user's configuration directory, typically:
//   - Linux/macOS: $XDG_CONFIG_HOME/tfquery.yaml or $HOME/.config/tfquery.yaml
//   - Windows: %APPDATA%/tfquery/tfquery.yaml
//
// Actual resolution relies on os.UserConfigDir which follows platform
// conventions.
package config
