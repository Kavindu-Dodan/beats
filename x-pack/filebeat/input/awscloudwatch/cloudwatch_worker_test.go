// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package awscloudwatch

import (
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/elastic-agent-libs/logp"
)

func TestAckTracker(t *testing.T) {
	t.Run("Simple run - increase and wait", func(t *testing.T) {
		tracker := newACKTracker()

		tracker.increaseAck(10)

		select {
		case <-time.After(1 * time.Second):
			t.Errorf("timed out waiting for acks")
		case <-tracker.waitFor(10):
			// test completed
		}
	})

	t.Run("Wait and acknowledge parallel", func(t *testing.T) {
		tracker := newACKTracker()

		go func() {
			<-time.After(100 * time.Millisecond)
			tracker.increaseAck(10)
		}()

		select {
		case <-time.After(200 * time.Millisecond):
			t.Errorf("timed out waiting for acks")
		case <-tracker.waitFor(10):
			// test completed
		}
	})
}

type filterLogEventsTestCase struct {
	name       string
	logGroupId string
	startTime  time.Time
	endTime    time.Time
	expected   *cloudwatchlogs.FilterLogEventsInput
}

func TestFilterLogEventsInput(t *testing.T) {
	now, _ := time.Parse(time.RFC3339, "2024-07-12T13:00:00+00:00")
	id := "myLogGroup"

	testCases := []filterLogEventsTestCase{
		{
			name:       "StartPosition: beginning, first iteration",
			logGroupId: id,
			// The zero value of type time.Time{} is January 1, year 1, 00:00:00.000000000 UTC
			// Events with a timestamp before the time - January 1, 1970, 00:00:00 UTC are not returned by AWS API
			// make sure zero value of time.Time{} was converted
			startTime: time.Time{},
			endTime:   now,
			expected: &cloudwatchlogs.FilterLogEventsInput{
				LogGroupIdentifier: awssdk.String(id),
				StartTime:          awssdk.Int64(0),
				EndTime:            awssdk.Int64(1720789200000),
			},
		},
	}
	for _, test := range testCases {
		cw := cwWorker{
			log: logp.NewLogger("test"),
		}
		result := cw.constructFilterLogEventsInput(test.startTime, test.endTime, test.logGroupId)
		assert.Equal(t, test.expected, result)
	}

}
