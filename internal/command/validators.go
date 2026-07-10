// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

type FlagValidatorType func(any) error

func FlagValidators(value any, validators ...FlagValidatorType) error {
	for _, v := range validators {
		if err := v(value); err != nil {
			return err
		}
	}
	return nil
}

func GlobalFlagsValidator(ctx context.Context, c *cli.Command) error {
	return nil
}

func OutputValidator(value any) error {
	validOutputFlagValues := []string{"text", "csv", "json", "raw", "yaml"}
	valid := false
	for _, v := range validOutputFlagValues {
		if v == value {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("must be one of %v", validOutputFlagValues)
	}
	return nil
}
