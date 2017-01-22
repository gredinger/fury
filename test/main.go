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

package main

import (
	"fmt"
	"log"
	"time"

	"go.universe.tf/fury"
	"go.universe.tf/fury/dsl"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func dir(name string) fury.File {
	return fury.File{
		Path:  name,
		IsDir: true,
		Owner: "root",
		Group: "root",
		Mode:  0755,
	}
}

func file(name, contents string) fury.File {
	return fury.File{
		Path:     name,
		Owner:    "root",
		Group:    "root",
		Mode:     0644,
		Contents: []byte(contents),
	}
}

func main() {
	ssh, err := fury.NewSSH("mininet.home.universe.tf")
	if err != nil {
		log.Fatalln(err)
	}

	role := dsl.Role(func() {
		dsl.Package("ca-certificates")
		dsl.File(dir("/etc/caddy"))
		dsl.File(file("/etc/caddy/Caddyfile", "import /etc/caddy/sites.d/*"))
		dsl.PreRun(func(ctx *fury.RunContext) error {
			must(ctx.Run("hostname"))
			return nil
		})
	})

	now := time.Now()
	if err = fury.Apply(ssh, []*fury.Role{role}); err != nil {
		log.Fatalln(err)
	}
	fmt.Printf("Apply took %s\n", time.Now().Sub(now))
}
