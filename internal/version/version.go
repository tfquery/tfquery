// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

// Do not import any other tfquery packages to avoid import cycles.
package version

import "runtime/debug"

var Version = "dev"

func init() {
	if Version != "dev" {
		return
	}

	if info, ok := debug.ReadBuildInfo(); ok &&
		info.Main.Version != "" &&
		info.Main.Version != "(devel)" {
		Version = info.Main.Version
	}
}
