// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package asyncstate

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument"
	apiInstrument "go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/sdk/metric/aggregator"
	"go.opentelemetry.io/otel/sdk/metric/internal/viewstate"
	"go.opentelemetry.io/otel/sdk/metric/number"
	"go.opentelemetry.io/otel/sdk/metric/number/traits"
	"go.opentelemetry.io/otel/sdk/metric/reader"
	"go.opentelemetry.io/otel/sdk/metric/sdkinstrument"
)

type (
	readerState struct {
		lock  sync.Mutex
		store map[attribute.Set]viewstate.Accumulator
	}

	Instrument struct {
		apiInstrument.Asynchronous

		descriptor sdkinstrument.Descriptor
		compiled   viewstate.Instrument
		state      map[*reader.Config]*readerState
	}

	Callback struct {
		function    func(context.Context)
		instruments map[*Instrument]struct{}
	}

	readerCallback struct {
		*reader.Config
		*Callback
	}

	Observer[N number.Any, Traits traits.Any[N]] struct {
		instrument.Asynchronous

		inst *Instrument
	}

	memberInstrument interface {
		instrument() *Instrument
	}

	contextKey struct{}
)

var _ memberInstrument = Observer[int64, traits.Int64]{}
var _ memberInstrument = Observer[float64, traits.Float64]{}

func NewInstrument(desc sdkinstrument.Descriptor, compiled viewstate.Instrument, readers []*reader.Config) *Instrument {
	state := map[*reader.Config]*readerState{}
	for _, r := range readers {
		state[r] = &readerState{
			store: map[attribute.Set]viewstate.Accumulator{},
		}
	}
	return &Instrument{
		descriptor: desc,
		compiled:   compiled,
		state:      state,
	}
}

func NewObserver[N number.Any, Traits traits.Any[N]](inst *Instrument) Observer[N, Traits] {
	return Observer[N, Traits]{inst: inst}
}

func NewCallback(instruments []apiInstrument.Asynchronous, function func(context.Context)) (*Callback, error) {
	cb := &Callback{
		function:    function,
		instruments: map[*Instrument]struct{}{},
	}

	for _, inst := range instruments {
		ai, ok := inst.(memberInstrument)
		if !ok {
			return nil, fmt.Errorf("asynchronous instrument does not belong to this provider: %T", inst)
		}
		cb.instruments[ai.instrument()] = struct{}{}
	}

	return cb, nil
}

func (c *Callback) Run(ctx context.Context, r *reader.Config) {
	c.function(context.WithValue(ctx, contextKey{}, readerCallback{
		Config:   r,
		Callback: c,
	}))
}

func (inst *Instrument) AccumulateFor(r *reader.Config) {
	rs := inst.state[r]

	// This limits concurrent asynchronous collection, which is
	// only needed in stateful configurations (i.e.,
	// cumulative-to-delta). TODO: does it matter that this blocks
	// concurrent Prometheus scrapers concurrently? (I think not.)
	rs.lock.Lock()
	defer rs.lock.Unlock()

	for _, capt := range rs.store {
		capt.Accumulate()
	}

	// Reset the instruments used; the view state will remember
	// what it needs.
	rs.store = map[attribute.Set]viewstate.Accumulator{}
}

func (o Observer[N, Traits]) instrument() *Instrument {
	return o.inst
}

func (o Observer[N, Traits]) Observe(ctx context.Context, value N, attrs ...attribute.KeyValue) {
	if o.inst == nil {
		return
	}

	lookup := ctx.Value(contextKey{})
	if lookup == nil {
		otel.Handle(fmt.Errorf("async instrument used outside of callback"))
		return
	}

	rc := lookup.(readerCallback)
	if _, ok := rc.Callback.instruments[o.inst]; !ok {
		otel.Handle(fmt.Errorf("async instrument not declared for use in callback"))
	}

	if err := aggregator.RangeTest[N, Traits](value, &o.inst.descriptor); err != nil {
		otel.Handle(err)
		return
	}
	se := o.inst.get(rc.Config, attrs)
	se.(viewstate.AccumulatorUpdater[N]).Update(value)
}

func (inst *Instrument) get(r *reader.Config, attrs []attribute.KeyValue) viewstate.Accumulator {
	rs := inst.state[r]
	rs.lock.Lock()
	defer rs.lock.Unlock()

	aset := attribute.NewSet(attrs...)
	se, has := rs.store[aset]
	if !has {
		se = inst.compiled.NewAccumulator(aset, r)
		rs.store[aset] = se
	}
	return se
}