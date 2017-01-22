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
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type Command struct {
	Path string
	Args []string
	// Add this to environment before running
	Env map[string]string

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type SSH struct {
	client *ssh.Client
}

func NewSSH(host string) (*SSH, error) {
	authSock := os.Getenv("SSH_AUTH_SOCK")
	if authSock == "" {
		return nil, fmt.Errorf("no SSH agent found, SSH_AUTH_SOCK not defined")
	}
	authConn, err := net.Dial("unix", authSock)
	if err != nil {
		return nil, fmt.Errorf("dialing SSH agent: %s", err)
	}
	defer authConn.Close()
	agent := agent.NewClient(authConn)
	signers, err := agent.Signers()
	if err != nil {
		return nil, fmt.Errorf("getting signers from SSH agent: %s", err)
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", host), &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signers...),
		},
		// TODO: host key verification
	})

	return &SSH{client}, err
}

func (s *SSH) Close() error {
	return s.client.Close()
}

func (s *SSH) Run(cmd Command) error {
	sess, err := s.client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()

	sess.Stdin = cmd.Stdin
	sess.Stdout = cmd.Stdout
	sess.Stderr = cmd.Stderr
	command := commandString(cmd)

	return sess.Run(command)
}

func commandString(cmd Command) string {
	var parts []string

	var env []string
	for e := range cmd.Env {
		env = append(env, e)
	}
	sort.Strings(env)
	for _, e := range env {
		parts = append(parts, fmt.Sprintf("%s=%s", e, shellEscape(cmd.Env[e])))
	}

	parts = append(parts, shellEscape(cmd.Path))
	for _, arg := range cmd.Args {
		parts = append(parts, shellEscape(arg))
	}

	return fmt.Sprintf("/bin/sh -c %s", shellEscape(strings.Join(parts, " ")))
}

// shellEscape escapes a value such that /bin/sh will interpret it
// completely literally, with no expansions at all.
func shellEscape(val string) string {
	return fmt.Sprintf("'%s'", strings.Replace(val, "'", `'\''`, -1))
}
