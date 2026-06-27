package version

import (
	"fmt"
	"runtime/debug"
	"strings"
)

var Version = "dev"

func init() {
	if Version != "dev" {
		return
	}

	var (
		revision string
		tag      string
	)

	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				revision = s.Value
			}
		}

		// This is Go's best guess at the nearest semantic version tag.
		tag = strings.TrimPrefix(info.Main.Version, "v")
	}

	if tag == "" || tag == "(devel)" {
		tag = "dev"
	}

	if len(revision) > 7 {
		revision = revision[:7]
	}

	if revision != "" {
		Version = fmt.Sprintf("%s+%s", tag, revision)
	} else {
		Version = tag
	}
}
