// Copyright 2024 Google LLC
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

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unsafe"
)

/*
#cgo CFLAGS: -Ithird_party/obs-studio/libobs
#include <obs-module.h>
#include <obs.h>

void blogit(int log_level, const char* prefix, const char* message) {
	blog(log_level, "[obs-studio-exporter] %s%s", prefix, message);
}
*/
import "C"

type OBSHandler struct {
	attrs  []string
	groups []string
}

func (h *OBSHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return true /* who can say */
}

func (h *OBSHandler) Handle(ctx context.Context, r slog.Record) error {
	var obsLevel C.int
	switch {
	case r.Level < slog.LevelInfo:
		obsLevel = C.LOG_DEBUG
	case r.Level < slog.LevelWarn:
		obsLevel = C.LOG_INFO
	case r.Level < slog.LevelError:
		obsLevel = C.LOG_WARNING
	default:
		obsLevel = C.LOG_ERROR
	}
	var prefix string
	if len(h.attrs) > 0 {
		prefix = strings.Join(h.attrs, " ")
	}
	prefixStr := C.CString(prefix)
	messageStr := C.CString(r.Message)
	C.blogit(obsLevel, prefixStr, messageStr)
	C.free(unsafe.Pointer(prefixStr))
	C.free(unsafe.Pointer(messageStr))
	return nil
}

func (h *OBSHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]string, len(h.attrs), len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	var groupPrefix string
	if len(h.groups) > 0 {
		groupPrefix = strings.Join(h.groups, ".") + "."
	}
	for _, attr := range attrs {
		newAttrs = append(newAttrs, fmt.Sprintf("%s%s=%s", groupPrefix, attr.Key, attr.Value.Resolve()))
	}
	return &OBSHandler{
		attrs:  newAttrs,
		groups: h.groups,
	}
}

func (h *OBSHandler) WithGroup(name string) slog.Handler {
	return &OBSHandler{
		attrs:  h.attrs,
		groups: append(h.groups, name),
	}
}
