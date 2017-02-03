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

package fury

import (
	"bufio"
	"fmt"
	"io"
	"sync"
)

// The thing that provides command running capability, I guess.
type RunContext struct {
	e         Executer
	logOutput bool

	mu     sync.Mutex
	log    io.Writer
	runNum int
}

func (r *RunContext) RunCommand(cmd Command) error {
	if !r.logOutput {
		return r.e.Run(cmd)
	}

	cmd2 := cmd

	// Grab a run number, which we'll use to log stuff about the
	// execution.
	r.mu.Lock()
	runNum := r.runNum
	r.runNum++
	r.mu.Unlock()

	var wg sync.WaitGroup
	wg.Add(2)

	logLines := func(rd io.Reader, kind string) {
		s := bufio.NewScanner(rd)
		for s.Scan() {
			r.Log("<%6d> <stdout> %s", runNum, s.Text())
		}
		if s.Err() != nil {
			r.Log("<%6d> <stdout> Error while reading stdout: %s", runNum, s.Err())
		}
		wg.Done()
	}

	stdoutr, stdoutw := io.Pipe()
	go logLines(stdoutr, "stdout")
	if cmd2.Stdout != nil {
		cmd2.Stdout = io.MultiWriter(stdoutw, cmd2.Stdout)
	} else {
		cmd2.Stdout = stdoutw
	}

	stderrr, stderrw := io.Pipe()
	go logLines(stderrr, "stderr")
	if cmd2.Stderr != nil {
		cmd2.Stderr = io.MultiWriter(stderrw, cmd2.Stderr)
	} else {
		cmd2.Stderr = stderrw
	}
	err := r.e.Run(cmd2)
	stdoutw.Close()
	stderrw.Close()

	wg.Wait()
	return err
}

func (r *RunContext) Run(argv ...string) error {
	return r.RunCommand(Command{
		Path: argv[0],
		Args: argv[1:],
	})
}

func (r *RunContext) ReadFile(path string) (File, error) {
}

func (r *RunContext) Log(msg string, args ...interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Fprintf(r.log, msg+"\n", args...)
}

func (r *RunContext) LogOutput(log bool) {
	r.logOutput = log
}
