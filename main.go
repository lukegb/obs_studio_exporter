// Copyright 2017 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package main is a loadable OBS Studio module which exports Prometheus-compatible metrics over HTTP.
package main

/*
#cgo CFLAGS: -Ithird_party/obs-studio/libobs
#cgo LDFLAGS: -L. -lobs
#include <obs-module.h>
#include <obs.h>

typedef bool (*mc_enum_outputs_proc)(void*, obs_output_t*);
typedef bool (*mc_enum_encoders_proc)(void*, obs_encoder_t*);

bool mc_enum_outputs_cb(void*, obs_output_t*);
bool mc_enum_encoders_cb(void*, obs_encoder_t*);
*/
import "C"

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"unsafe"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	obsLock sync.Mutex

	activeMetricCollector *MetricCollector
)

type MetricCollector struct {
	ActiveFPS          *prometheus.Desc
	AverageFrameTimeNS *prometheus.Desc
	TotalFrames        *prometheus.Desc
	LaggedFrames       *prometheus.Desc

	InfoPerOutput          *prometheus.Desc
	OutputActivePerOutput  *prometheus.Desc
	TotalBytesPerOutput    *prometheus.Desc
	DroppedFramesPerOutput *prometheus.Desc
	TotalFramesPerOutput   *prometheus.Desc
	WidthPerOutput         *prometheus.Desc
	HeightPerOutput        *prometheus.Desc
	CongestionPerOutput    *prometheus.Desc
	ConnectTimePerOutput   *prometheus.Desc
	ReconnectingPerOutput  *prometheus.Desc

	InfoPerEncoder       *prometheus.Desc
	CodecPerEncoder      *prometheus.Desc
	WidthPerEncoder      *prometheus.Desc
	HeightPerEncoder     *prometheus.Desc
	SampleRatePerEncoder *prometheus.Desc
	ActivePerEncoder     *prometheus.Desc

	enumOutputsCB  func(unsafe.Pointer, *C.obs_output_t) C.bool
	enumEncodersCB func(unsafe.Pointer, *C.obs_encoder_t) C.bool
}

func NewMetricCollector() *MetricCollector {
	return &MetricCollector{
		ActiveFPS:          prometheus.NewDesc("obs_global_active_fps", "Active frames per second.", nil, prometheus.Labels{}),
		AverageFrameTimeNS: prometheus.NewDesc("obs_global_average_frame_time_ns", "Average time to render a frame in nanoseconds.", nil, prometheus.Labels{}),
		TotalFrames:        prometheus.NewDesc("obs_global_total_frames", "Total frames generated.", nil, prometheus.Labels{}),
		LaggedFrames:       prometheus.NewDesc("obs_global_lagged_frames", "Lagged frames.", nil, prometheus.Labels{}),

		InfoPerOutput:          prometheus.NewDesc("obs_output_info", "Information about this output.", []string{"output_id", "output_name", "output_display_name"}, prometheus.Labels{}),
		OutputActivePerOutput:  prometheus.NewDesc("obs_output_active", "Whether the output is active.", []string{"output_id", "output_name"}, prometheus.Labels{}),
		TotalBytesPerOutput:    prometheus.NewDesc("obs_output_total_bytes", "Total bytes sent to this output.", []string{"output_id", "output_name"}, prometheus.Labels{}),
		DroppedFramesPerOutput: prometheus.NewDesc("obs_output_dropped_frames", "Frames dropped by this output.", []string{"output_id", "output_name"}, prometheus.Labels{}),
		TotalFramesPerOutput:   prometheus.NewDesc("obs_output_total_frames", "Total frames sent from this output.", []string{"output_id", "output_name"}, prometheus.Labels{}),
		WidthPerOutput:         prometheus.NewDesc("obs_output_video_width", "Video width of this output.", []string{"output_id", "output_name"}, prometheus.Labels{}),
		HeightPerOutput:        prometheus.NewDesc("obs_output_video_height", "Video height of this output.", []string{"output_id", "output_name"}, prometheus.Labels{}),
		CongestionPerOutput:    prometheus.NewDesc("obs_output_congestion", "'Congestion' of this output.", []string{"output_id", "output_name"}, prometheus.Labels{}),
		ConnectTimePerOutput:   prometheus.NewDesc("obs_output_connect_time_ms", "Time taken to connect in milliseconds for this output.", []string{"output_id", "output_name"}, prometheus.Labels{}),
		ReconnectingPerOutput:  prometheus.NewDesc("obs_output_reconnecting", "Whether the output is reconnecting.", []string{"output_id", "output_name"}, prometheus.Labels{}),

		InfoPerEncoder:       prometheus.NewDesc("obs_encoder_info", "Information about this encoder.", []string{"encoder_id", "encoder_name", "encoder_display_name", "encoder_codec"}, prometheus.Labels{}),
		WidthPerEncoder:      prometheus.NewDesc("obs_encoder_width", "Video width of this encoder.", []string{"encoder_id", "encoder_name"}, prometheus.Labels{}),
		HeightPerEncoder:     prometheus.NewDesc("obs_encoder_height", "Video height of this encoder.", []string{"encoder_id", "encoder_name"}, prometheus.Labels{}),
		SampleRatePerEncoder: prometheus.NewDesc("obs_encoder_sample_rate", "Audio sample rate of this encoder.", []string{"encoder_id", "encoder_name"}, prometheus.Labels{}),
		ActivePerEncoder:     prometheus.NewDesc("obs_encoder_active", "Whether the encoder is active.", []string{"encoder_id", "encoder_name"}, prometheus.Labels{}),
	}
}

func (c *MetricCollector) Describe(ch chan<- *prometheus.Desc) {
	obsLock.Lock()
	defer obsLock.Unlock()

	ch <- c.ActiveFPS
	ch <- c.AverageFrameTimeNS
	ch <- c.TotalFrames
	ch <- c.LaggedFrames

	ch <- c.OutputActivePerOutput
	ch <- c.TotalBytesPerOutput
	ch <- c.DroppedFramesPerOutput
	ch <- c.TotalFramesPerOutput
	ch <- c.WidthPerOutput
	ch <- c.HeightPerOutput
	ch <- c.CongestionPerOutput
	ch <- c.ConnectTimePerOutput
	ch <- c.ReconnectingPerOutput

	ch <- c.InfoPerEncoder
	ch <- c.WidthPerEncoder
	ch <- c.HeightPerEncoder
	ch <- c.SampleRatePerEncoder
	ch <- c.ActivePerEncoder
}

func obsBoolMetric(b C.bool) float64 {
	if bool(b) {
		return 1
	}
	return 0
}

func (c *MetricCollector) Collect(ch chan<- prometheus.Metric) {
	obsLock.Lock()
	defer obsLock.Unlock()

	ch <- prometheus.MustNewConstMetric(c.ActiveFPS, prometheus.GaugeValue, float64(C.obs_get_active_fps()))
	ch <- prometheus.MustNewConstMetric(c.AverageFrameTimeNS, prometheus.GaugeValue, float64(C.obs_get_average_frame_time_ns()))
	ch <- prometheus.MustNewConstMetric(c.TotalFrames, prometheus.CounterValue, float64(C.obs_get_total_frames()))
	ch <- prometheus.MustNewConstMetric(c.LaggedFrames, prometheus.CounterValue, float64(C.obs_get_lagged_frames()))

	c.enumOutputsCB = func(v unsafe.Pointer, o *C.obs_output_t) C.bool {
		idC := C.obs_output_get_id(o)
		id := C.GoString(idC)
		name := C.GoString(C.obs_output_get_name(o))
		displayName := C.GoString(C.obs_output_get_display_name(idC))

		ch <- prometheus.MustNewConstMetric(c.InfoPerOutput, prometheus.GaugeValue, 1, id, name, displayName)
		ch <- prometheus.MustNewConstMetric(c.OutputActivePerOutput, prometheus.GaugeValue, obsBoolMetric(C.obs_output_active(o)), id, name)
		ch <- prometheus.MustNewConstMetric(c.TotalBytesPerOutput, prometheus.CounterValue, float64(C.obs_output_get_total_bytes(o)), id, name)
		ch <- prometheus.MustNewConstMetric(c.DroppedFramesPerOutput, prometheus.CounterValue, float64(C.obs_output_get_frames_dropped(o)), id, name)
		ch <- prometheus.MustNewConstMetric(c.TotalFramesPerOutput, prometheus.GaugeValue, float64(C.obs_output_get_total_frames(o)), id, name)
		ch <- prometheus.MustNewConstMetric(c.WidthPerOutput, prometheus.GaugeValue, float64(C.obs_output_get_width(o)), id, name)
		ch <- prometheus.MustNewConstMetric(c.HeightPerOutput, prometheus.GaugeValue, float64(C.obs_output_get_height(o)), id, name)
		ch <- prometheus.MustNewConstMetric(c.CongestionPerOutput, prometheus.GaugeValue, float64(C.obs_output_get_congestion(o)), id, name)
		ch <- prometheus.MustNewConstMetric(c.ConnectTimePerOutput, prometheus.GaugeValue, float64(C.obs_output_get_connect_time_ms(o)), id, name)
		ch <- prometheus.MustNewConstMetric(c.ReconnectingPerOutput, prometheus.GaugeValue, obsBoolMetric(C.obs_output_reconnecting(o)), id, name)

		return C.bool(true)
	}
	C.obs_enum_outputs(C.mc_enum_outputs_proc(C.mc_enum_outputs_cb), nil)

	c.enumEncodersCB = func(v unsafe.Pointer, o *C.obs_encoder_t) C.bool {
		idC := C.obs_encoder_get_id(o)
		id := C.GoString(idC)
		name := C.GoString(C.obs_encoder_get_name(o))
		displayName := C.GoString(C.obs_encoder_get_display_name(idC))
		log.Println(id, name, displayName)

		ch <- prometheus.MustNewConstMetric(c.InfoPerEncoder, prometheus.GaugeValue, 1, id, name, displayName, C.GoString(C.obs_encoder_get_codec(o)))
		ch <- prometheus.MustNewConstMetric(c.WidthPerEncoder, prometheus.GaugeValue, float64(C.obs_encoder_get_width(o)), id, name)
		ch <- prometheus.MustNewConstMetric(c.HeightPerEncoder, prometheus.GaugeValue, float64(C.obs_encoder_get_height(o)), id, name)
		ch <- prometheus.MustNewConstMetric(c.SampleRatePerEncoder, prometheus.GaugeValue, float64(C.obs_encoder_get_sample_rate(o)), id, name)
		ch <- prometheus.MustNewConstMetric(c.ActivePerEncoder, prometheus.GaugeValue, obsBoolMetric(C.obs_encoder_active(o)), id, name)
		return C.bool(true)
	}
	C.obs_enum_encoders(C.mc_enum_encoders_proc(C.mc_enum_encoders_cb), nil)
}

func registerMetrics() {
	activeMetricCollector = NewMetricCollector()
	prometheus.MustRegister(activeMetricCollector)
}

//export obs_module_load
func obs_module_load() C.bool {
	registerMetrics()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "You have reached obs-studio-exporter. Please leave a message after the beep.")
	})
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Fatal(http.ListenAndServe(":9407", nil))
	}()
	return true
}

//export mc_enum_outputs_cb_go
func mc_enum_outputs_cb_go(f unsafe.Pointer, s *C.obs_output_t) C.bool {
	return activeMetricCollector.enumOutputsCB(f, s)
}

//export mc_enum_encoders_cb_go
func mc_enum_encoders_cb_go(f unsafe.Pointer, s *C.obs_encoder_t) C.bool {
	return activeMetricCollector.enumEncodersCB(f, s)
}
