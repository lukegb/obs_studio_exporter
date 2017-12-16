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
#cgo LDFLAGS: -L. -lobs
#include <obs-module.h>
#include <obs.h>

bool mc_enum_outputs_cb(void* f, obs_output_t* s) {
	bool mc_enum_outputs_cb_go(void*, obs_output_t*);
	return mc_enum_outputs_cb_go(f, s);
}
bool mc_enum_encoders_cb(void* f, obs_encoder_t* s) {
	bool mc_enum_encoders_cb_go(void*, obs_encoder_t*);
	return mc_enum_encoders_cb_go(f, s);
}
*/
import "C"
