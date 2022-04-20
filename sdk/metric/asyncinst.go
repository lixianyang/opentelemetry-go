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

package metric

import (
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/asyncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/asyncint64"
	"go.opentelemetry.io/otel/sdk/metric/internal/asyncstate"
	"go.opentelemetry.io/otel/sdk/metric/number"
	"go.opentelemetry.io/otel/sdk/metric/number/traits"
	"go.opentelemetry.io/otel/sdk/metric/sdkinstrument"
)

type (
	asyncint64Instruments   struct{ *meter }
	asyncfloat64Instruments struct{ *meter }
)

func (m *meter) AsyncInt64() asyncint64.InstrumentProvider {
	return asyncint64Instruments{m}
}

func (m *meter) AsyncFloat64() asyncfloat64.InstrumentProvider {
	return asyncfloat64Instruments{m}
}

func (m *meter) newAsyncInst(name string, opts []instrument.Option, nk number.Kind, ik sdkinstrument.Kind) (*asyncstate.Instrument, error) {
	return configureInstrument(
		m, name, opts, nk, ik,
		func(desc sdkinstrument.Descriptor) (*asyncstate.Instrument, error) {
			compiled, err := m.views.Compile(desc)
			inst := asyncstate.NewInstrument(desc, compiled, m.provider.cfg.readers)
			return inst, err
		})
}

func (i asyncint64Instruments) Counter(name string, opts ...instrument.Option) (asyncint64.Counter, error) {
	inst, err := i.newAsyncInst(name, opts, number.Int64Kind, sdkinstrument.CounterObserverKind)
	return asyncstate.NewObserver[int64, traits.Int64](inst), err
}

func (i asyncint64Instruments) UpDownCounter(name string, opts ...instrument.Option) (asyncint64.UpDownCounter, error) {
	inst, err := i.newAsyncInst(name, opts, number.Int64Kind, sdkinstrument.UpDownCounterObserverKind)
	return asyncstate.NewObserver[int64, traits.Int64](inst), err
}

func (i asyncint64Instruments) Gauge(name string, opts ...instrument.Option) (asyncint64.Gauge, error) {
	inst, err := i.newAsyncInst(name, opts, number.Int64Kind, sdkinstrument.GaugeObserverKind)
	return asyncstate.NewObserver[int64, traits.Int64](inst), err
}

func (f asyncfloat64Instruments) Counter(name string, opts ...instrument.Option) (asyncfloat64.Counter, error) {
	inst, err := f.newAsyncInst(name, opts, number.Float64Kind, sdkinstrument.CounterObserverKind)
	return asyncstate.NewObserver[float64, traits.Float64](inst), err
}

func (f asyncfloat64Instruments) UpDownCounter(name string, opts ...instrument.Option) (asyncfloat64.UpDownCounter, error) {
	inst, err := f.newAsyncInst(name, opts, number.Float64Kind, sdkinstrument.UpDownCounterObserverKind)
	return asyncstate.NewObserver[float64, traits.Float64](inst), err
}

func (f asyncfloat64Instruments) Gauge(name string, opts ...instrument.Option) (asyncfloat64.Gauge, error) {
	inst, err := f.newAsyncInst(name, opts, number.Float64Kind, sdkinstrument.GaugeObserverKind)
	return asyncstate.NewObserver[float64, traits.Float64](inst), err
}