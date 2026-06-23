// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package s3

import (
	"github.com/tfquery/tfquery/internal/cacheutil"
	"github.com/tfquery/tfquery/internal/config"
)

// CacheEntry is provided by cacheutil.Entry; local alias removed to avoid duplication.

// CacheEntryPath returns the path to the cache entry for the given key, if it
// exists. The cache is organized first by the backend hostname
// (app.terraform.io) and then by the organization name. The key is hashed and
// used as the filename.
func CacheEntryPath(be *BackendS3, key string) (string, bool) {
	sub := []string{be.Backend.Config.Bucket, be.Backend.Config.Prefix, be.Backend.Config.Key}
	p, exists := cacheutil.EntryPath(sub, key)
	if !exists {
		return "", false
	}
	return p, true
}

// CacheReader reads the cache entry for the given key, if it exists. If the
// cache is disabled, or the entry does not exist, the second return value will
// be false.
func CacheReader(be *BackendS3, key string) (*cacheutil.Entry, bool) {
	sub := []string{be.Backend.Config.Bucket, be.Backend.Config.Prefix, be.Backend.Config.Key}
	return cacheutil.Read(sub, key)
}

func CacheWriter(be *BackendS3, key string, data []byte) error {
	sub := []string{be.Backend.Config.Bucket, be.Backend.Config.Prefix, be.Backend.Config.Key}
	return cacheutil.Write(sub, key, data)
}

func PurgeCache() error {
	cleanHours, _ := config.GetInt("cache.clean")
	return cacheutil.Purge(cleanHours)
}
