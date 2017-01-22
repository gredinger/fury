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

package fury // import "go.universe.tf/fury"

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"sync"
	"time"
)

type Executer interface {
	Run(Command) error
}

type File struct {
	Path     string
	IsDir    bool
	Owner    string
	Group    string
	Mode     int64
	Contents []byte
}

// The thing that provides command running capability, I guess.
type RunContext struct {
	e         Executer
	logOutput bool

	mu     sync.Mutex
	log    io.Writer
	runNum int
}

type syncWriter struct {
	sync.Mutex
	w io.Writer
}

func (s *syncWriter) Write(b []byte) (int, error) {
	s.Lock()
	defer s.Unlock()
	return s.w.Write(b)
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

func (r *RunContext) Log(msg string, args ...interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Fprintf(r.log, msg+"\n", args...)
}

func (r *RunContext) LogOutput(log bool) {
	r.logOutput = log
}

type Role struct {
	PreRun   func(*RunContext) error
	Packages []string
	Files    []File
	PostRun  func(*RunContext) error
}

func Apply(exec Executer, roles []*Role) error {
	pkgs, files, err := mergeOps(roles)
	if err != nil {
		return err
	}

	// TODO: validate that files are sensical

	ctx := &RunContext{
		e:         exec,
		log:       os.Stdout,
		logOutput: true,
	}

	for _, role := range roles {
		if role.PreRun != nil {
			if err = role.PreRun(ctx); err != nil {
				return err
			}
		}
	}

	if err := ctx.Run(append([]string{"apt-get", "-y", "install"}, pkgs...)...); err != nil {
		return err
	}

	pr, pw := io.Pipe()
	tarCmd := Command{
		Path:  "tar",
		Args:  []string{"-z", "-x", "-v", "-f-", "-P", "-C/"},
		Stdin: pr,
	}
	ch := make(chan error, 1)
	go func() { ch <- streamTarGz(pw, files) }()
	if err := ctx.RunCommand(tarCmd); err != nil {
		pr.Close()
		return err
	}
	if err := <-ch; err != nil {
		return err
	}

	for _, role := range roles {
		if role.PostRun != nil {
			if err = role.PostRun(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

// mergeOps merges the Packages and Files from the given Roles.
func mergeOps(roles []*Role) ([]string, []File, error) {
	pkgMap := map[string]bool{}
	fileMap := map[string]File{}
	for _, role := range roles {
		for _, pkg := range role.Packages {
			pkgMap[pkg] = true
		}
		for _, file := range role.Files {
			if existing, ok := fileMap[file.Path]; ok {
				if !reflect.DeepEqual(existing, file) {
					return nil, nil, fmt.Errorf("redefinition of path %q", file.Path)
				}
			}
			fileMap[file.Path] = file
		}
	}

	pkgs := []string{}
	for pkg := range pkgMap {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)

	filenames := []string{}
	for filename := range fileMap {
		filenames = append(filenames, filename)
	}
	sort.Strings(filenames)

	files := []File{}
	for _, filename := range filenames {
		files = append(files, fileMap[filename])
	}

	return pkgs, files, nil
}

func streamTarGz(out io.WriteCloser, files []File) error {
	defer out.Close()
	gzOut := gzip.NewWriter(out)
	defer gzOut.Close()
	tarOut := tar.NewWriter(gzOut)
	defer tarOut.Close()

	now := time.Now()

	for _, file := range files {
		if file.IsDir {
			if err := tarOut.WriteHeader(&tar.Header{
				Name:     file.Path,
				Mode:     file.Mode,
				Uname:    file.Owner,
				Gname:    file.Group,
				ModTime:  now,
				Typeflag: tar.TypeDir,
			}); err != nil {
				return err
			}
		} else {
			if err := tarOut.WriteHeader(&tar.Header{
				Name:     file.Path,
				Mode:     file.Mode,
				Uname:    file.Owner,
				Gname:    file.Group,
				ModTime:  now,
				Size:     int64(len(file.Contents)),
				Typeflag: tar.TypeReg,
			}); err != nil {
				return err
			}
			if _, err := tarOut.Write(file.Contents); err != nil {
				return err
			}
		}
	}

	return nil
}
