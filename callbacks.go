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

package main

/*
#cgo CFLAGS: -Iobs-studio/libobs
#cgo darwin LDFLAGS: -L. -lobs.0
#cgo linux LDFLAGS: -L. -lobs
#cgo windows LDFLAGS: -L. -lobs
#include <obs-module.h>
#include <obs.h>

bool mc_enum_sources_cb(void* f, obs_output_t* s) {
	bool mc_enum_sources_cb_go(void*, obs_output_t*);
	return mc_enum_sources_cb_go(f, s);
}
bool mc_enum_outputs_cb(void* f, obs_output_t* s) {
	bool mc_enum_outputs_cb_go(void*, obs_output_t*);
	return mc_enum_outputs_cb_go(f, s);
}
bool mc_enum_encoders_cb(void* f, obs_encoder_t* s) {
	bool mc_enum_encoders_cb_go(void*, obs_encoder_t*);
	return mc_enum_encoders_cb_go(f, s);
}
void mc_volmeter_updated(void* f, const float magnitude[MAX_AUDIO_CHANNELS], const float peak[MAX_AUDIO_CHANNELS], const float input_peak[MAX_AUDIO_CHANNELS]) {
	void mc_volmeter_updated_go(void*, const float[MAX_AUDIO_CHANNELS], const float[MAX_AUDIO_CHANNELS], const float[MAX_AUDIO_CHANNELS]);
	mc_volmeter_updated_go(f, magnitude, peak, input_peak);
}
*/
import "C"
