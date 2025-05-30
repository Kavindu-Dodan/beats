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

/*
Package testing provides utility functions for testing Module and MetricSet
implementations.

# MetricSet Example

This is an example showing how to use this package to test a MetricSet. By
using these methods you ensure the MetricSet is instantiated in the same way
that Metricbeat does it and with the same validations.

	package mymetricset_test

	import (
		"github.com/stretchr/testify/assert"

		mbtest "github.com/elastic/beats/v7/metricbeat/mb/testing"
	)

	func TestFetch(t *testing.T) {
		f := mbtest.NewFetcher(t, getConfig())
		events, errs := f.FetchEvents()
		assert.Empty(t, errs)
		assert.NotEmpty(t, events)

		event := events[0]
		t.Logf("%s/%s event: %+v", f.Module().Name(), f.Name(), event)

		// Test event attributes...
	}

	func getConfig() map[string]interface{} {
		return map[string]interface{}{
			"module":     "mymodule",
			"metricsets": []string{"status"},
			"hosts":      []string{mymodule.GetHostFromEnv()},
		}
	}
*/

package testing

import (
	"context"
	"testing"
	"time"

	"github.com/elastic/beats/v7/libbeat/management/status"
	"github.com/elastic/go-concert/timed"

	"github.com/elastic/beats/v7/metricbeat/mb"
	conf "github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp/logptest"
)

type TestModule struct {
	ModName   string
	ModConfig mb.ModuleConfig
	RawConfig *conf.C
}

func (m *TestModule) Name() string                              { return m.ModName }
func (m *TestModule) Config() mb.ModuleConfig                   { return m.ModConfig }
func (m *TestModule) UnpackConfig(to interface{}) error         { return m.RawConfig.Unpack(to) }
func (m *TestModule) UpdateStatus(_ status.Status, _ string)    {}
func (m *TestModule) SetStatusReporter(_ status.StatusReporter) {}

func NewTestModule(t testing.TB, config interface{}) *TestModule {
	c, err := conf.NewConfigFrom(config)
	if err != nil {
		t.Fatal(err)
	}

	return &TestModule{RawConfig: c}
}

// NewMetricSet instantiates a new MetricSet using the given configuration.
// The ModuleFactory and MetricSetFactory are obtained from the global
// Registry.
func NewMetricSet(t testing.TB, config interface{}) mb.MetricSet {
	return NewMetricSetWithRegistry(t, config, mb.Registry)
}

// NewMetricSetWithRegistry instantiates a new MetricSet using the given configuration.
// The ModuleFactory and MetricSetFactory are obtained from the passed in registry.
func NewMetricSetWithRegistry(t testing.TB, config interface{}, registry *mb.Register) mb.MetricSet {
	metricsets := NewMetricSetsWithRegistry(t, config, registry)

	if len(metricsets) != 1 {
		t.Fatal("invalid number of metricsets instantiated")
	}

	metricset := metricsets[0]
	if metricset == nil {
		t.Fatal("metricset is nil")
	}
	return metricset
}

// NewMetricSets instantiates a list of new MetricSets using the given
// module configuration.
func NewMetricSets(t testing.TB, config interface{}) []mb.MetricSet {
	return NewMetricSetsWithRegistry(t, config, mb.Registry)
}

// NewMetricSetsWithRegistry instantiates a list of new MetricSets using the given
// module configuration and provided registry.
func NewMetricSetsWithRegistry(t testing.TB, config interface{}, registry *mb.Register) []mb.MetricSet {
	c, err := conf.NewConfigFrom(config)
	if err != nil {
		t.Fatal(err)
	}
	m, metricsets, err := mb.NewModule(c, registry, logptest.NewTestingLogger(t, ""))
	if err != nil {
		t.Fatal("failed to create new MetricSet", err)
	}
	if m == nil {
		t.Fatal("no module instantiated")
	}

	return metricsets
}

// NewReportingMetricSetV2 returns a new ReportingMetricSetV2 instance. Then
// you can use ReportingFetchV2 to perform a Fetch operation with the MetricSet.
func NewReportingMetricSetV2(t testing.TB, config interface{}) mb.ReportingMetricSetV2 {
	return NewReportingMetricSetV2WithRegistry(t, config, mb.Registry)
}

// NewReportingMetricSetV2WithRegistry returns a new ReportingMetricSetV2 instance. Then
// you can use ReportingFetchV2 to perform a Fetch operation with the MetricSet.
func NewReportingMetricSetV2WithRegistry(t testing.TB, config interface{}, registry *mb.Register) mb.ReportingMetricSetV2 {
	metricSet := NewMetricSetWithRegistry(t, config, registry)

	reportingMetricSetV2, ok := metricSet.(mb.ReportingMetricSetV2)
	if !ok {
		t.Fatal("MetricSet does not implement ReportingMetricSetV2")
	}

	return reportingMetricSetV2
}

// NewReportingMetricSetV2Error returns a new ReportingMetricSetV2 instance. Then
// you can use ReportingFetchV2 to perform a Fetch operation with the MetricSet.
func NewReportingMetricSetV2Error(t testing.TB, config interface{}) mb.ReportingMetricSetV2Error {
	metricSet := NewMetricSet(t, config)

	reportingMetricSetV2Error, ok := metricSet.(mb.ReportingMetricSetV2Error)
	if !ok {
		t.Fatal("MetricSet does not implement ReportingMetricSetV2Error")
	}

	return reportingMetricSetV2Error
}

// NewReportingMetricSetV2Errors returns an array of new ReportingMetricSetV2 instances.
func NewReportingMetricSetV2Errors(t testing.TB, config interface{}) []mb.ReportingMetricSetV2Error {
	metricSets := NewMetricSets(t, config)
	reportingMetricSets := make([]mb.ReportingMetricSetV2Error, 0, len(metricSets))
	for _, metricSet := range metricSets {
		rMS, ok := metricSet.(mb.ReportingMetricSetV2Error)
		if !ok {
			t.Fatalf("MetricSet %v does not implement ReportingMetricSetV2Error", metricSet.Name())
		}

		reportingMetricSets = append(reportingMetricSets, rMS)
	}

	return reportingMetricSets
}

// NewReportingMetricSetV2WithContext returns a new ReportingMetricSetV2WithContext instance. Then
// you can use ReportingFetchV2 to perform a Fetch operation with the MetricSet.
func NewReportingMetricSetV2WithContext(t testing.TB, config interface{}) mb.ReportingMetricSetV2WithContext {
	metricSet := NewMetricSet(t, config)

	reportingMetricSet, ok := metricSet.(mb.ReportingMetricSetV2WithContext)
	if !ok {
		t.Fatal("MetricSet does not implement ReportingMetricSetV2WithContext")
	}

	return reportingMetricSet
}

// CapturingReporterV2 is a reporter used for testing which stores all events and errors
type CapturingReporterV2 struct {
	events []mb.Event
	errs   []error
}

// Event is used to report an event
func (r *CapturingReporterV2) Event(event mb.Event) bool {
	r.events = append(r.events, event)
	return true
}

// Error is used to report an error
func (r *CapturingReporterV2) Error(err error) bool {
	r.errs = append(r.errs, err)
	return true
}

// GetEvents returns all reported events
func (r *CapturingReporterV2) GetEvents() []mb.Event {
	return r.events
}

// GetErrors returns all reported errors
func (r *CapturingReporterV2) GetErrors() []error {
	return r.errs
}

// ReportingFetchV2 runs the given reporting metricset and returns all of the
// events and errors that occur during that period.
func ReportingFetchV2(metricSet mb.ReportingMetricSetV2) ([]mb.Event, []error) {
	r := &CapturingReporterV2{}
	metricSet.Fetch(r)
	return r.events, r.errs
}

// ReportingFetchV2Error runs the given reporting metricset and returns all of the
// events and errors that occur during that period.
func ReportingFetchV2Error(metricSet mb.ReportingMetricSetV2Error) ([]mb.Event, []error) {
	r := &CapturingReporterV2{}
	err := metricSet.Fetch(r)
	if err != nil {
		r.errs = append(r.errs, err)
	}
	return r.events, r.errs
}

// PeriodicReportingFetchV2Error runs the given metricset and returns
// the first batch of events or errors that occur during that period.
//
// `period` is the time between each fetch.
// `timeout` is the maximum time to wait for the first event.
//
// The function tries to fetch the metrics every `period` until it gets
// the first batch of metrics or the `timeout` is reached.
func PeriodicReportingFetchV2Error(metricSet mb.ReportingMetricSetV2Error, period time.Duration, timeout time.Duration) ([]mb.Event, []error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	r := &CapturingReporterV2{}
	_ = timed.Periodic(ctx, period, func() error {
		// Fetch the metrics and store them in the
		// reporter.
		if err := metricSet.Fetch(r); err != nil {
			r.errs = append(r.errs, err)
			return err
		}

		if len(r.events) > 0 {
			// We have metrics, stop the periodic
			// and return the metrics.
			cancel()
		}

		// No metrics yet, retry again
		// in the next period.
		return nil
	})

	return r.events, r.errs
}

// ReportingFetchV2WithContext runs the given reporting metricset and returns all of the
// events and errors that occur during that period.
func ReportingFetchV2WithContext(metricSet mb.ReportingMetricSetV2WithContext) ([]mb.Event, []error) {
	r := &CapturingReporterV2{}
	err := metricSet.Fetch(context.Background(), r)
	if err != nil {
		r.errs = append(r.errs, err)
	}
	return r.events, r.errs
}

// NewPushMetricSetV2 instantiates a new PushMetricSetV2 using the given
// configuration. The ModuleFactory and MetricSetFactory are obtained from the
// global Registry.
func NewPushMetricSetV2(t testing.TB, config interface{}) mb.PushMetricSetV2 {
	metricSet := NewMetricSet(t, config)

	pushMetricSet, ok := metricSet.(mb.PushMetricSetV2)
	if !ok {
		t.Fatal("MetricSet does not implement PushMetricSetV2")
	}

	return pushMetricSet
}

// NewPushMetricSetV2WithRegistry instantiates a new PushMetricSetV2 using the given
// configuration. The ModuleFactory and MetricSetFactory are obtained from the
// passed in the registry.
func NewPushMetricSetV2WithRegistry(t testing.TB, config interface{}, registry *mb.Register) mb.PushMetricSetV2 {
	metricSet := NewMetricSetWithRegistry(t, config, registry)

	pushMetricSet, ok := metricSet.(mb.PushMetricSetV2)
	if !ok {
		t.Fatal("MetricSet does not implement PushMetricSetV2")
	}

	return pushMetricSet
}

// NewPushMetricSetV2WithContext instantiates a new PushMetricSetV2WithContext
// using the given configuration. The ModuleFactory and MetricSetFactory are
// obtained from the global Registry.
func NewPushMetricSetV2WithContext(t testing.TB, config interface{}) mb.PushMetricSetV2WithContext {
	metricSet := NewMetricSet(t, config)

	pushMetricSet, ok := metricSet.(mb.PushMetricSetV2WithContext)
	if !ok {
		t.Fatal("MetricSet does not implement PushMetricSetV2WithContext")
	}

	return pushMetricSet
}

// CapturingPushReporterV2 stores all the events and errors from a metricset's
// Run method.
type CapturingPushReporterV2 struct {
	context.Context
	eventsC chan mb.Event
}

func newCapturingPushReporterV2(ctx context.Context) *CapturingPushReporterV2 {
	return &CapturingPushReporterV2{Context: ctx, eventsC: make(chan mb.Event)}
}

// report writes an event to the output channel and returns true. If the output
// is closed it returns false.
func (r *CapturingPushReporterV2) report(event mb.Event) bool {
	select {
	case <-r.Done():
		// Publisher is stopped.
		return false
	case r.eventsC <- event:
		return true
	}
}

// Event stores the passed-in event into the events array
func (r *CapturingPushReporterV2) Event(event mb.Event) bool {
	return r.report(event)
}

// Error stores the given error into the errors array.
func (r *CapturingPushReporterV2) Error(err error) bool {
	return r.report(mb.Event{Error: err})
}

func (r *CapturingPushReporterV2) capture(waitEvents int) []mb.Event {
	var events []mb.Event
	for {
		select {
		case <-r.Done():
			// Timeout
			return events
		case e := <-r.eventsC:
			events = append(events, e)
			if waitEvents > 0 && len(events) >= waitEvents {
				return events
			}
		}
	}
}

// BlockingCapture blocks until waitEvents n of events are captured
func (r *CapturingPushReporterV2) BlockingCapture(waitEvents int) []mb.Event {
	events := make([]mb.Event, 0, waitEvents)

	for e := range r.eventsC {
		events = append(events, e)
		if waitEvents > 0 && len(events) >= waitEvents {
			return events
		}
	}

	return events
}

// RunPushMetricSetV2 run the given push metricset for the specific amount of
// time and returns all of the events and errors that occur during that period.
func RunPushMetricSetV2(timeout time.Duration, waitEvents int, metricSet mb.PushMetricSetV2) []mb.Event {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	r := newCapturingPushReporterV2(ctx)

	go metricSet.Run(r)
	return r.capture(waitEvents)
}

// GetCapturingPushReporterV2 is a factory for a capturing push metricset
func GetCapturingPushReporterV2() mb.PushReporterV2 {
	return newCapturingPushReporterV2(context.Background())
}

// RunPushMetricSetV2WithContext run the given push metricset for the specific amount of
// time and returns all of the events that occur during that period.
func RunPushMetricSetV2WithContext(timeout time.Duration, waitEvents int, metricSet mb.PushMetricSetV2WithContext) []mb.Event {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	r := newCapturingPushReporterV2(ctx)

	go metricSet.Run(ctx, r)
	return r.capture(waitEvents)
}
