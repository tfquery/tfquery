// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

// Do not import any other tfquery packages to avoid import cycles.

package version

import "runtime/debug"

var Version = func() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}()
