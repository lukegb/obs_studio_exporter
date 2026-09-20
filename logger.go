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
	"fmt"
	"log"
	"strings"
)

func logDebug(msg string, kv ...any) { logWithLevel("DEBUG", msg, kv...) }
func logInfo(msg string, kv ...any)  { logWithLevel("INFO", msg, kv...) }
func logWarn(msg string, kv ...any)  { logWithLevel("WARN", msg, kv...) }
func logError(msg string, kv ...any) { logWithLevel("ERROR", msg, kv...) }

func logWithLevel(level, msg string, kv ...any) {
	if len(kv)%2 != 0 {
		kv = append(kv, "<missing>")
	}
	var b strings.Builder
	b.WriteString("[obs-studio-exporter] ")
	b.WriteString(level)
	b.WriteString(": ")
	b.WriteString(msg)
	for i := 0; i < len(kv); i += 2 {
		b.WriteString(" ")
		b.WriteString(fmt.Sprintf("%v=%v", kv[i], kv[i+1]))
	}
	log.Print(b.String())
}
