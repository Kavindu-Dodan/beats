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

package add_cloud_metadata

import (
	"context"
	"errors"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	conf "github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
)

func init() {
	os.Unsetenv("BEATS_ADD_CLOUD_METADATA_PROVIDERS")
}

func TestProvidersFilter(t *testing.T) {
	var allLocal []string
	for name, ff := range cloudMetaProviders {
		if ff.DefaultEnabled {
			allLocal = append(allLocal, name)
		}
	}

	cases := map[string]struct {
		config   map[string]interface{}
		env      string
		fail     bool
		expected []string
	}{
		"all with local access only if not configured": {
			config:   map[string]interface{}{},
			expected: allLocal,
		},
		"BEATS_ADD_CLOUD_METADATA_PROVIDERS overrides default": {
			config:   map[string]interface{}{},
			env:      "alibaba, digitalocean",
			expected: []string{"alibaba", "digitalocean"},
		},
		"none if BEATS_ADD_CLOUD_METADATA_PROVIDERS is explicitly set to an empty list": {
			config:   map[string]interface{}{},
			env:      " ",
			expected: nil,
		},
		"fail to load if unknown name is used": {
			config: map[string]interface{}{
				"providers": []string{"unknown"},
			},
			fail: true,
		},
		"only selected": {
			config: map[string]interface{}{
				"providers": []string{"aws", "gcp", "digitalocean"},
			},
		},
		"BEATS_ADD_CLOUD_METADATA_PROVIDERS overrides selected": {
			config: map[string]interface{}{
				"providers": []string{"aws", "gcp", "digitalocean"},
			},
			env:      "alibaba, digitalocean",
			expected: []string{"alibaba", "digitalocean"},
		},
	}

	copyStrings := func(in []string) (out []string) {
		return append(out, in...)
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			rawConfig := conf.MustNewConfigFrom(test.config)
			if test.env != "" {
				t.Setenv("BEATS_ADD_CLOUD_METADATA_PROVIDERS", test.env)
			}

			config := defaultConfig()
			err := rawConfig.Unpack(&config)
			if err == nil && test.fail {
				t.Fatal("Did expect to fail on unpack")
			} else if err != nil && !test.fail {
				t.Fatal("Unpack failed", err)
			} else if test.fail && err != nil {
				return
			}

			// compute list of providers that should have matched
			var expected []string
			if len(test.expected) == 0 && len(config.Providers) > 0 {
				expected = copyStrings(config.Providers)
			} else {
				expected = copyStrings(test.expected)
			}
			sort.Strings(expected)

			var actual []string
			for name := range selectProviders(config.Providers, cloudMetaProviders) {
				actual = append(actual, name)
			}

			sort.Strings(actual)
			assert.Equal(t, expected, actual)
		})
	}
}

func Test_priorityResult(t *testing.T) {
	tLogger := logptest.NewTestingLogger(t, "add_cloud_metadata testing")
	awsRsp := result{
		provider: "aws",
		metadata: map[string]interface{}{
			"id": "a-1",
		},
	}

	openStackRsp := result{
		provider: "openstack",
		metadata: map[string]interface{}{
			"id": "o-1",
		},
	}

	digitaloceanRsp := result{
		provider: "digitalocean",
		metadata: map[string]interface{}{
			"id": "d-1",
		},
	}

	tests := []struct {
		name      string
		collected []result
		want      *result
	}{
		{
			name:      "Empty results returns nil",
			collected: []result{},
			want:      nil,
		},
		{
			name: "Error result returns nil",
			collected: []result{
				{
					provider: "aws",
					err:      errors.New("some error"),
				},
			},
			want: nil,
		},
		{
			name:      "Single result returns the same",
			collected: []result{awsRsp},
			want:      &awsRsp,
		},
		{
			name:      "Priority result wins",
			collected: []result{openStackRsp, awsRsp},
			want:      &awsRsp,
		},
		{
			name:      "For non-priority result, response order wins",
			collected: []result{openStackRsp, digitaloceanRsp},
			want:      &openStackRsp,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffered result channel with the test results preloaded
			resultChan := make(chan result)
			ctx, cancel := context.WithCancel(context.Background())

			responseChan := make(chan *result)
			go func() {
				response := acceptFirstPriorityResult(ctx, tLogger, time.Now(), resultChan)
				cancel()
				responseChan <- response
			}()

			for _, result := range tt.collected {
				select {
				case resultChan <- result:
				case <-ctx.Done():
				}
			}
			// Cancel the context for cases that haven't returned yet and
			// fetch the final response.
			cancel()
			response := <-responseChan

			assert.Equal(t, tt.want, response)
		})
	}
}
