// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	s3v2 "github.com/aws/aws-sdk-go-v2/service/s3"

	tfcfg "github.com/tfquery/tfquery/internal/config"
	"github.com/tfquery/tfquery/internal/log"
)

// options holds optional overrides for AWS config loading.
type options struct {
	profile string
	region  string
	retryer func() awsv2.Retryer
}

// Option customizes how AWS config is loaded.
// Default behavior (no options) inherits the shell environment and shared
// config chain (AWS_PROFILE, ~/.aws/config, ~/.aws/credentials, IMDS, etc.).
type Option func(*options)

// LoadAWSConfig loads AWS SDK v2 config. By default it inherits the shell's
// AWS setup (AWS_PROFILE, shared config, env, IMDS). Options can override
// profile, region, and retryer without changing callers.
func LoadAWSConfig(ctx context.Context, opts ...Option) (awsv2.Config, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	log.Debugf("opts applied: profile=%s, region=%s", o.profile, o.region)

	var loadOpts []func(*config.LoadOptions) error
	if o.profile != "" {
		loadOpts = append(loadOpts, config.WithSharedConfigProfile(o.profile))
	}
	if o.region != "" {
		loadOpts = append(loadOpts, config.WithRegion(o.region))
	}
	if o.retryer != nil {
		loadOpts = append(loadOpts, config.WithRetryer(o.retryer))
	}
	log.Debugf("loadOpts built: len=%d", len(loadOpts))

	cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		log.Debugf("config load err: err=%v", err)
		return awsv2.Config{}, err
	}
	log.Debugf("config loaded")
	return cfg, nil
}

// NewS3 constructs a v2 S3 client from the provided config. Additional service
// options can be supplied via optFns.
func NewS3(cfg awsv2.Config, optFns ...func(*s3v2.Options)) *s3v2.Client {
	moreOptFns := append(append([]func(*s3v2.Options){}, optFns...), func(o *s3v2.Options) {
		// Side-effect of s3 implementations (localstack, ministack, minio, etc)
		// that may not have full S3 support.
		// https://github.com/aws/aws-sdk-go-v2/discussions/2960#discussioncomment-12077210
		o.UsePathStyle, _ = tfcfg.GetBool("s3_backend.path_style", true)

		v, _ := tfcfg.GetBool("s3_backend.checksum_calculation", true)
		if v {
			o.RequestChecksumCalculation = awsv2.RequestChecksumCalculationWhenRequired
			o.ResponseChecksumValidation = awsv2.ResponseChecksumValidationWhenRequired
		} else {
			o.RequestChecksumCalculation = awsv2.RequestChecksumCalculationWhenSupported
			o.ResponseChecksumValidation = awsv2.ResponseChecksumValidationWhenSupported
		}
	})

	client := s3v2.NewFromConfig(cfg, moreOptFns...)
	log.Debugf("s3 client created")
	return client
}

// WithProfile sets the shared config profile. Defaults to AWS_PROFILE/env chain.
func WithProfile(profile string) Option {
	return func(o *options) { o.profile = profile }
}

// WithRegion sets the region override. Defaults to env/profile/metadata chain.
func WithRegion(region string) Option {
	return func(o *options) { o.region = region }
}

// WithRetryer injects a custom retryer; if not set, SDK defaults are used.
func WithRetryer(newRetryer func() awsv2.Retryer) Option {
	return func(o *options) { o.retryer = newRetryer }
}

// Endpoint resolution is service-specific in AWS SDK v2.
// For S3, pass an option to NewS3 that sets Options.EndpointResolverV2.

// WithS3EndpointResolver allows callers to set the S3 EndpointResolverV2
// in a type-safe way when constructing the client.
func WithS3EndpointResolver(r s3v2.EndpointResolverV2) func(*s3v2.Options) {
	return func(o *s3v2.Options) {
		o.EndpointResolverV2 = r
	}
}
