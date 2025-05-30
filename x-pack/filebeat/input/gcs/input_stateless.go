// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package gcs

import (
	"context"

	"cloud.google.com/go/storage"
	gax "github.com/googleapis/gax-go/v2"
	"golang.org/x/sync/errgroup"

	v2 "github.com/elastic/beats/v7/filebeat/input/v2"
	cursor "github.com/elastic/beats/v7/filebeat/input/v2/input-cursor"
	stateless "github.com/elastic/beats/v7/filebeat/input/v2/input-stateless"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/management/status"
)

type statelessInput struct {
	config config
}

func (statelessInput) Name() string {
	return "gcs-stateless"
}

func newStatelessInput(config config) *statelessInput {
	return &statelessInput{config: config}
}

func (in *statelessInput) Test(v2.TestContext) error {
	return nil
}

type statelessPublisher struct {
	wrapped stateless.Publisher
}

func (pub statelessPublisher) Publish(event beat.Event, _ interface{}) error {
	pub.wrapped.Publish(event)
	return nil
}

// Run starts the input and blocks until it ends the execution.
// It will return on context cancellation, any other error will be retried.
func (in *statelessInput) Run(inputCtx v2.Context, publisher stateless.Publisher, client *storage.Client) error {
	pub := statelessPublisher{wrapped: publisher}
	var source cursor.Source
	var g errgroup.Group

	stat := inputCtx.StatusReporter
	if stat == nil {
		stat = noopReporter{}
	}
	stat.UpdateStatus(status.Starting, "")
	stat.UpdateStatus(status.Configuring, "")

	for _, b := range in.config.Buckets {
		bucket := tryOverrideOrDefault(in.config, b)
		source = &Source{
			ProjectId:                in.config.ProjectId,
			BucketName:               bucket.Name,
			BatchSize:                *bucket.BatchSize,
			MaxWorkers:               *bucket.MaxWorkers,
			Poll:                     *bucket.Poll,
			PollInterval:             *bucket.PollInterval,
			ParseJSON:                *bucket.ParseJSON,
			TimeStampEpoch:           bucket.TimeStampEpoch,
			ExpandEventListFromField: bucket.ExpandEventListFromField,
			FileSelectors:            bucket.FileSelectors,
			ReaderConfig:             bucket.ReaderConfig,
			Retry:                    in.config.Retry,
		}

		st := newState()
		currentSource := source.(*Source)
		log := inputCtx.Logger.With("project_id", currentSource.ProjectId).With("bucket", currentSource.BucketName)
		metrics := newInputMetrics(inputCtx.ID+":"+currentSource.BucketName, nil)
		defer metrics.Close()
		metrics.url.Set("gs://" + currentSource.BucketName)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			<-inputCtx.Cancelation.Done()
			stat.UpdateStatus(status.Stopping, "")
			cancel()
		}()

		bkt := client.Bucket(currentSource.BucketName).Retryer(
			// Use WithMaxAttempts to change the maximum number of attempts.
			storage.WithMaxAttempts(currentSource.Retry.MaxAttempts),
			// Use WithBackoff to change the timing of the exponential backoff.
			storage.WithBackoff(gax.Backoff{
				Initial:    currentSource.Retry.InitialBackOffDuration,
				Max:        currentSource.Retry.MaxBackOffDuration,
				Multiplier: currentSource.Retry.BackOffMultiplier,
			}),
			// RetryAlways will retry the operation even if it is non-idempotent.
			// Since we are only reading, the operation is always idempotent
			storage.WithPolicy(storage.RetryAlways),
		)
		scheduler := newScheduler(pub, bkt, currentSource, &in.config, st, stat, metrics, log)
		// allows multiple containers to be scheduled concurrently while testing
		// the stateless input is triggered only while testing and till now it did not mimic
		// the real world concurrent execution of multiple containers. This fix allows it to do so.
		g.Go(func() error {
			return scheduler.schedule(ctx)
		})
	}
	return g.Wait()
}
