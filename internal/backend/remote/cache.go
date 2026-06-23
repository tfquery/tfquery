// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package remote

import (
	"os"

	"github.com/tfquery/tfquery/internal/cacheutil"
	"github.com/tfquery/tfquery/internal/config"
)

// CacheEntry is provided by cacheutil.Entry; local alias removed to avoid
// duplication.

// CacheEntryPath returns the path to the cache entry for the given key, if it
// exists. The cache is organized first by the backend hostname
// (app.terraform.io) and then by the organization name. The key is hashed and
// used as the filename.
func CacheEntryPath(be *BackendRemote, key string) (string, bool) {
	hostname, organization := getOverrides(be)
	p, exists := cacheutil.EntryPath([]string{hostname, organization}, key)
	if !exists {
		return "", false
	}
	return p, true
}

// CacheReader reads the cache entry for the given key, if it exists. If the
// cache is disabled, or the entry does not exist, the second return value will
// be false.
func CacheReader(be *BackendRemote, key string) (*cacheutil.Entry, bool) {
	hostname, organization := getOverrides(be)
	return cacheutil.Read([]string{hostname, organization}, key)
}

func CacheWriter(be *BackendRemote, key string, data []byte) error {
	hostname, organization := getOverrides(be)
	return cacheutil.Write([]string{hostname, organization}, key, data)
}

func PurgeCache() error {
	cleanHours, _ := config.GetInt("cache.clean")
	return cacheutil.Purge(cleanHours)
}

func getOverrides(be *BackendRemote) (hostname, organization string) {
	hostname = be.Backend.Config.Hostname
	if h, ok := os.LookupEnv("TFE_HOSTNAME"); ok {
		hostname = h
	}

	organization = be.Backend.Config.Organization
	if org, ok := os.LookupEnv("TFE_ORGANIZATION"); ok {
		organization = org
	}

	return
}
