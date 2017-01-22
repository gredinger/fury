// Copyright 2016 Google Inc.
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

package dsl

import "go.universe.tf/fury"

var currentRole *fury.Role

func Role(f func()) *fury.Role {
	currentRole = &fury.Role{}
	f()
	ret := currentRole
	currentRole = nil
	return ret
}

func Package(pkg string) {
	currentRole.Packages = append(currentRole.Packages, pkg)
}

func Packages(pkgs ...string) {
	currentRole.Packages = append(currentRole.Packages, pkgs...)
}

func File(f fury.File) {
	currentRole.Files = append(currentRole.Files, f)
}

func PreRun(f func(*fury.RunContext) error) {
	currentRole.PreRun = f
}

func PostRun(f func(*fury.RunContext) error) {
	currentRole.PostRun = f
}
