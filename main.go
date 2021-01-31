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
#cgo darwin LDFLAGS: -L. -lobs.0
#cgo linux LDFLAGS: -L. -lobs
#cgo windows LDFLAGS: -L. -lobs
#include <obs-module.h>
#include <obs.h>

typedef bool (*mc_enum_sources_proc)(void*, obs_source_t*);
typedef bool (*mc_enum_outputs_proc)(void*, obs_output_t*);
typedef bool (*mc_enum_encoders_proc)(void*, obs_encoder_t*);

bool mc_enum_sources_cb(void*, obs_source_t*);
bool mc_enum_outputs_cb(void*, obs_output_t*);
bool mc_enum_encoders_cb(void*, obs_encoder_t*);
void mc_volmeter_updated(void*, const float[MAX_AUDIO_CHANNELS], const float[MAX_AUDIO_CHANNELS], const float[MAX_AUDIO_CHANNELS]);
*/
import "C"

import (
	"fmt"
	"log"
	"math"
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

const (
	// number chosen by fair dice roll
	circBufSamples = 32
	// Prometheus metrics namespace.
	namespace = "obs"
	// Prometheus metric subsystems
	encoderSubsystem = "encoder"
	globalSubsystem  = "global"
	outputSubsystem  = "output"
	sourceSubsystem  = "source"
)

type Source struct {
	ID       string
	CID      *C.char
	Name     string
	VolMeter *C.obs_volmeter_t
	Channels int

	mu        sync.Mutex
	Pos       int
	Magnitude [][circBufSamples]float64
	Peak      [][circBufSamples]float64
	InputPeak [][circBufSamples]float64
}

type MetricCollector struct {
	ActiveFPS          *prometheus.Desc
	AverageFrameTimeNS *prometheus.Desc
	TotalFrames        *prometheus.Desc
	LaggedFrames       *prometheus.Desc
	VideoTotalFrames   *prometheus.Desc
	VideoSkippedFrames *prometheus.Desc

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

	MagnitudePerSourceChannel *prometheus.Desc
	PeakPerSourceChannel      *prometheus.Desc
	InputPeakPerSourceChannel *prometheus.Desc

	mu      sync.Mutex
	sources map[string]*Source

	enumSourcesCB  func(unsafe.Pointer, *C.obs_source_t) C.bool
	enumOutputsCB  func(unsafe.Pointer, *C.obs_output_t) C.bool
	enumEncodersCB func(unsafe.Pointer, *C.obs_encoder_t) C.bool
}

func NewMetricCollector() *MetricCollector {
	return &MetricCollector{
		ActiveFPS: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, globalSubsystem, "active_fps"),
			"Active frames per second.",
			nil, prometheus.Labels{},
		),
		AverageFrameTimeNS: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, globalSubsystem, "average_frame_time_ns"),
			"Average time to render a frame in nanoseconds.",
			nil, prometheus.Labels{},
		),
		TotalFrames: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, globalSubsystem, "frames_total"),
			"Total frames generated.",
			nil, prometheus.Labels{},
		),
		LaggedFrames: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, globalSubsystem, "lagged_frames_total"),
			"Skipped frames due to encoding lag.",
			nil, prometheus.Labels{},
		),
		VideoTotalFrames: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, globalSubsystem, "video_frames_total"),
			"Total video frames generated.",
			nil, prometheus.Labels{},
		),
		VideoSkippedFrames: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, globalSubsystem, "video_skipped_frames_total"),
			"Frames missed due to rendering lab.",
			nil, prometheus.Labels{},
		),

		InfoPerOutput: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, outputSubsystem, "info"),
			"Information about this output.",
			[]string{"output_id", "output_name", "output_display_name"}, prometheus.Labels{},
		),
		OutputActivePerOutput: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, outputSubsystem, "active"),
			"Whether the output is active.",
			[]string{"output_id", "output_name"}, prometheus.Labels{},
		),
		TotalBytesPerOutput: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, outputSubsystem, "bytes_total"),
			"Total bytes sent to this output.", []string{"output_id", "output_name"}, prometheus.Labels{},
		),
		DroppedFramesPerOutput: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, outputSubsystem, "dropped_frames_total"),
			"Frames dropped by this output.", []string{"output_id", "output_name"}, prometheus.Labels{},
		),
		TotalFramesPerOutput: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, outputSubsystem, "frames"),
			"Total frames sent from this output.", []string{"output_id", "output_name"}, prometheus.Labels{},
		),
		WidthPerOutput: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, outputSubsystem, "video_width"),
			"Video width of this output.", []string{"output_id", "output_name"}, prometheus.Labels{},
		),
		HeightPerOutput: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, outputSubsystem, "video_height"),
			"Video height of this output.", []string{"output_id", "output_name"}, prometheus.Labels{},
		),
		CongestionPerOutput: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, outputSubsystem, "congestion"),
			"'Congestion' of this output.",
			[]string{"output_id", "output_name"}, prometheus.Labels{},
		),
		ConnectTimePerOutput: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, outputSubsystem, "connect_time_seconds"),
			"Time taken to connect in seconds for this output.",
			[]string{"output_id", "output_name"}, prometheus.Labels{},
		),
		ReconnectingPerOutput: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, outputSubsystem, "reconnecting"),
			"Whether the output is reconnecting.",
			[]string{"output_id", "output_name"}, prometheus.Labels{},
		),

		InfoPerEncoder: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, encoderSubsystem, "info"),
			"Information about this encoder.",
			[]string{"encoder_id", "encoder_name", "encoder_display_name", "encoder_codec"}, prometheus.Labels{},
		),
		WidthPerEncoder: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, encoderSubsystem, "width"),
			"Video width of this encoder.",
			[]string{"encoder_id", "encoder_name"}, prometheus.Labels{},
		),
		HeightPerEncoder: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, encoderSubsystem, "height"),
			"Video height of this encoder.",
			[]string{"encoder_id", "encoder_name"}, prometheus.Labels{},
		),
		SampleRatePerEncoder: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, encoderSubsystem, "sample_rate"),
			"Audio sample rate of this encoder.", []string{"encoder_id", "encoder_name"}, prometheus.Labels{},
		),
		ActivePerEncoder: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, globalSubsystem, "active"),
			"Whether the encoder is active.",
			[]string{"encoder_id", "encoder_name"}, prometheus.Labels{},
		),

		MagnitudePerSourceChannel: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, sourceSubsystem, "channel_magnitude"),
			"Max source channel magnitude.",
			[]string{"source_id", "source_name", "channel_id"}, prometheus.Labels{},
		),
		PeakPerSourceChannel: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, sourceSubsystem, "channel_peak"),
			"Max source channel peak.",
			[]string{"source_id", "source_name", "channel_id"}, prometheus.Labels{},
		),
		InputPeakPerSourceChannel: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, sourceSubsystem, "input_peak"),
			"Max source channel input peak.",
			[]string{"source_id", "source_name", "channel_id"}, prometheus.Labels{},
		),

		sources: map[string]*Source{},
	}
}

func (c *MetricCollector) Describe(ch chan<- *prometheus.Desc) {
	obsLock.Lock()
	defer obsLock.Unlock()

	ch <- c.ActiveFPS
	ch <- c.AverageFrameTimeNS
	ch <- c.TotalFrames
	ch <- c.LaggedFrames
	ch <- c.VideoTotalFrames
	ch <- c.VideoSkippedFrames

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

	ch <- c.MagnitudePerSourceChannel
	ch <- c.PeakPerSourceChannel
	ch <- c.InputPeakPerSourceChannel
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

	vid := C.obs_get_video()
	ch <- prometheus.MustNewConstMetric(c.VideoTotalFrames, prometheus.CounterValue, float64(C.video_output_get_total_frames(vid)))
	ch <- prometheus.MustNewConstMetric(c.VideoSkippedFrames, prometheus.CounterValue, float64(C.video_output_get_skipped_frames(vid)))

	c.mu.Lock()
	seenSources := map[string]bool{}
	c.enumSourcesCB = func(v unsafe.Pointer, o *C.obs_source_t) C.bool {
		idC := C.obs_source_get_id(o)
		id := C.GoString(idC)
		name := C.GoString(C.obs_source_get_name(o))

		if seenSources[id] {
			return C.bool(true)
		}
		seenSources[id] = true

		src, ok := c.sources[id]
		if !ok {
			src = &Source{
				ID:   id,
				Name: name,
				CID:  C.CString(id),
			}
			negInf := math.Inf(-1)
			vm := C.obs_volmeter_create(C.OBS_FADER_CUBIC)
			if vm == nil {
				log.Printf("failed to create volmeter for source %v/%v", id, name)
				return C.bool(true)
			}
			src.VolMeter = vm
			if ok := bool(C.obs_volmeter_attach_source(vm, o)); !ok {
				log.Printf("failed to attach source %v/%v to volmeter", id, name)
				C.obs_volmeter_destroy(vm)
				return C.bool(true)
			}
			C.obs_volmeter_set_update_interval(vm, 1000)
			C.obs_volmeter_add_callback(vm, C.obs_volmeter_updated_t(C.mc_volmeter_updated), unsafe.Pointer(src.CID))

			//src.Channels = int(C.obs_volmeter_get_nr_channels(vm))
			src.Channels = 2
			src.Magnitude = make([][circBufSamples]float64, src.Channels)
			src.Peak = make([][circBufSamples]float64, src.Channels)
			src.InputPeak = make([][circBufSamples]float64, src.Channels)
			for ch := 0; ch < src.Channels; ch++ {
				var magnitude, peak, inputPeak [circBufSamples]float64
				for n := 0; n < circBufSamples; n++ {
					magnitude[n] = negInf
					peak[n] = negInf
					inputPeak[n] = negInf
				}
				src.Magnitude[ch] = magnitude
				src.Peak[ch] = peak
				src.InputPeak[ch] = inputPeak
			}

			c.sources[id] = src
		} else {
			ninf := math.Inf(-1)
			for chn := 0; chn < src.Channels; chn++ {
				magnitude := ninf
				peak := ninf
				inputPeak := ninf
				for n := 0; n < circBufSamples; n++ {
					magnitude = math.Max(magnitude, src.Magnitude[chn][n])
					peak = math.Max(peak, src.Peak[chn][n])
					inputPeak = math.Max(inputPeak, src.InputPeak[chn][n])
				}
				chnstr := fmt.Sprintf("%d", chn)
				ch <- prometheus.MustNewConstMetric(c.MagnitudePerSourceChannel, prometheus.GaugeValue, magnitude, src.ID, src.Name, chnstr)
				ch <- prometheus.MustNewConstMetric(c.PeakPerSourceChannel, prometheus.GaugeValue, peak, src.ID, src.Name, chnstr)
				ch <- prometheus.MustNewConstMetric(c.InputPeakPerSourceChannel, prometheus.GaugeValue, inputPeak, src.ID, src.Name, chnstr)
			}
		}
		return C.bool(true)
	}
	C.obs_enum_sources(C.mc_enum_sources_proc(C.mc_enum_sources_cb), nil)
	for id, s := range c.sources {
		if seenSources[id] {
			continue
		}
		// Delete s.
		delete(c.sources, id)
		C.free(unsafe.Pointer(s.CID))
		if s.VolMeter != nil {
			C.obs_volmeter_destroy(s.VolMeter)
		}
	}
	c.mu.Unlock()

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
		ch <- prometheus.MustNewConstMetric(c.ConnectTimePerOutput, prometheus.GaugeValue, float64(C.obs_output_get_connect_time_ms(o))/1000.0, id, name)
		ch <- prometheus.MustNewConstMetric(c.ReconnectingPerOutput, prometheus.GaugeValue, obsBoolMetric(C.obs_output_reconnecting(o)), id, name)

		return C.bool(true)
	}
	C.obs_enum_outputs(C.mc_enum_outputs_proc(C.mc_enum_outputs_cb), nil)

	c.enumEncodersCB = func(v unsafe.Pointer, o *C.obs_encoder_t) C.bool {
		idC := C.obs_encoder_get_id(o)
		id := C.GoString(idC)
		name := C.GoString(C.obs_encoder_get_name(o))
		displayName := C.GoString(C.obs_encoder_get_display_name(idC))
		isAudioEncoder := C.obs_encoder_get_type(o) == C.OBS_ENCODER_AUDIO

		ch <- prometheus.MustNewConstMetric(c.InfoPerEncoder, prometheus.GaugeValue, 1, id, name, displayName, C.GoString(C.obs_encoder_get_codec(o)))
		ch <- prometheus.MustNewConstMetric(c.ActivePerEncoder, prometheus.GaugeValue, obsBoolMetric(C.obs_encoder_active(o)), id, name)

		if isAudioEncoder {
			ch <- prometheus.MustNewConstMetric(c.WidthPerEncoder, prometheus.GaugeValue, 0, id, name)
			ch <- prometheus.MustNewConstMetric(c.HeightPerEncoder, prometheus.GaugeValue, 0, id, name)
			ch <- prometheus.MustNewConstMetric(c.SampleRatePerEncoder, prometheus.GaugeValue, float64(C.obs_encoder_get_sample_rate(o)), id, name)
		} else {
			ch <- prometheus.MustNewConstMetric(c.WidthPerEncoder, prometheus.GaugeValue, float64(C.obs_encoder_get_width(o)), id, name)
			ch <- prometheus.MustNewConstMetric(c.HeightPerEncoder, prometheus.GaugeValue, float64(C.obs_encoder_get_height(o)), id, name)
			ch <- prometheus.MustNewConstMetric(c.SampleRatePerEncoder, prometheus.GaugeValue, 0, id, name)
		}

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
		for port := 9407; port < 9500; port++ {
			log.Println("Trying port %d...", port)
			log.Println(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
		}
		// Don't crash OBS because we couldn't listen on the port.
	}()
	return true
}

//export mc_enum_sources_cb_go
func mc_enum_sources_cb_go(f unsafe.Pointer, s *C.obs_source_t) C.bool {
	return activeMetricCollector.enumSourcesCB(f, s)
}

//export mc_enum_outputs_cb_go
func mc_enum_outputs_cb_go(f unsafe.Pointer, s *C.obs_output_t) C.bool {
	return activeMetricCollector.enumOutputsCB(f, s)
}

//export mc_enum_encoders_cb_go
func mc_enum_encoders_cb_go(f unsafe.Pointer, s *C.obs_encoder_t) C.bool {
	return activeMetricCollector.enumEncodersCB(f, s)
}

func genSlice(inp unsafe.Pointer) []float64 {
	out := make([]float64, C.MAX_AUDIO_CHANNELS)
	for n := 0; n < C.MAX_AUDIO_CHANNELS; n++ {
		out[n] = float64(*(*C.float)(unsafe.Pointer(uintptr(inp) + uintptr(n)*unsafe.Sizeof(C.float(0)))))
	}
	return out
}

//export mc_volmeter_updated_go
func mc_volmeter_updated_go(f unsafe.Pointer, magnitude, peak, inputPeak unsafe.Pointer) {
	id := C.GoString((*C.char)(f))

	activeMetricCollector.mu.Lock()
	src, ok := activeMetricCollector.sources[id]
	if !ok {
		log.Printf("unknown source %v", id)
		activeMetricCollector.mu.Unlock()
		return
	}
	activeMetricCollector.mu.Unlock()

	src.mu.Lock()
	defer src.mu.Unlock()

	omagnitude := genSlice(magnitude)
	opeak := genSlice(peak)
	oinputPeak := genSlice(inputPeak)
	for ch := 0; ch < src.Channels; ch++ {
		src.Magnitude[ch][src.Pos] = omagnitude[ch]
		src.Peak[ch][src.Pos] = opeak[ch]
		src.InputPeak[ch][src.Pos] = oinputPeak[ch]
	}
	src.Pos = (src.Pos + 1) % circBufSamples
}
