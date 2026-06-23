// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/tfquery/tfquery/internal/cacheutil"
	"github.com/tfquery/tfquery/internal/command"
	"github.com/tfquery/tfquery/internal/log"
)

var ctx = context.Background()

func main() {
	os.Exit(realMain())
}

func realMain() int {
	log.InitLogger()

	args := os.Args
	log.Debugf("args captured: args=%v", args)

	if handleVersion(args) {
		return 0
	}

	args = handleNakedCommand(args)

	// If --help appears anywhere, skip command processing and let the CLI
	// handle it.
	helpFound := false
	for _, a := range args {
		if a == "--help" || a == "-h" {
			helpFound = true
			break
		}
	}

	if !helpFound {
		args = processCommandArgs(args)
	}

	return initAndRunApp(args)
}

func initAndRunApp(args []string) int {
	// Pre-create cache directory when caching is enabled.
	if _, ok, err := cacheutil.EnsureBaseDir(); err != nil && ok {
		fmt.Fprintln(os.Stderr, err)
		log.Debugf("cache ensure err: err=%v", err)
	}

	app, err := command.InitApp(ctx, args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		log.Debugf("app init err: err=%v", err)
		return 1
	}

	if err := app.Run(ctx, args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		log.Debugf("app run err: err=%v", err)
		return 2
	}

	return 0
}
