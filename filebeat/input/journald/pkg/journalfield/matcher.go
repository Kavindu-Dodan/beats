// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

//go:build linux

package journalfield

import (
	"errors"
	"fmt"
	"strings"
)

// Matcher is a single field condition for filtering journal entries.
//
// The Matcher type can be used as is with Beats configuration unpacking. The
// internal default conversion table will be used, similar to BuildMatcher.
type Matcher struct {
	str string
}

// MatcherBuilder can be used to create a custom builder for creating matchers
// based on a conversion table.
type MatcherBuilder struct {
	Conversions map[string]Conversion
}

// IncludeMatches stores the advanced matching configuratio
// provided by the user.
type IncludeMatches struct {
	Matches []Matcher `config:"match"`
}

var (
	defaultBuilder = MatcherBuilder{Conversions: journaldEventFields}
)

func (i IncludeMatches) Validate() error {
	var errs []error
	for _, m := range i.Matches {
		if err := m.validate(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

var errInvalidMatcher = errors.New("expression must be '+' or in the format 'field=value'")

func (m Matcher) validate() error {
	if len(m.str) == 1 {
		if m.str != "+" {
			return fmt.Errorf("'%s' is invalid, %w", m.str, errInvalidMatcher)
		}

		return nil
	}

	elems := strings.Split(m.str, "=")
	if len(elems) != 2 {
		return fmt.Errorf("'%s' is invalid, %w", m.str, errInvalidMatcher)
	}

	return nil
}

// Build creates a new Matcher using the configured conversion table.
// If no table has been configured the internal default table will be used.
func (b MatcherBuilder) Build(in string) (Matcher, error) {
	if in == "+" {
		return Matcher{in}, nil
	}

	elems := strings.Split(in, "=")
	if len(elems) != 2 {
		return Matcher{}, fmt.Errorf("invalid match format: %s", in)
	}

	conversions := b.Conversions
	if conversions == nil {
		conversions = journaldEventFields
	}

	for journalKey, eventField := range conversions {
		for _, name := range eventField.Names {
			if elems[0] == name {
				return Matcher{journalKey + "=" + elems[1]}, nil
			}
		}
	}

	// pass custom fields as is
	return Matcher{in}, nil
}

// BuildMatcher creates a Matcher from a field filter string.
func BuildMatcher(in string) (Matcher, error) {
	return defaultBuilder.Build(in)
}

// String returns the string representation of the field match.
func (m Matcher) String() string { return m.str }

// Unpack initializes the Matcher from a given string representation. Unpack
// fails if the input string is invalid.
// Unpack can be used with Beats configuration loading.
func (m *Matcher) Unpack(value string) error {
	tmp, err := BuildMatcher(value)
	if err != nil {
		return err
	}
	*m = tmp
	return nil
}
