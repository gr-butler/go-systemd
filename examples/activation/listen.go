// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build ignore
// +build ignore

// Activation example used by the activation unit tests.
package main

import (
	"fmt"
	"os"

	"github.com/gr-butler/go-systemd/v22/activation"
)

func fixListenPid() {
	if os.Getenv("FIX_LISTEN_PID") != "" {
		// HACK: real systemd would set LISTEN_PID before exec'ing but
		// this is too difficult in golang for the purpose of a test.
		// Do not do this in real code.
		os.Setenv("LISTEN_PID", fmt.Sprintf("%d", os.Getpid()))
	}
}

func main() {
	fixListenPid()

	listenersWithNames, err := activation.ListenersWithNames()
	if err != nil {
		panic(err)
	}

	if os.Getenv("LISTEN_PID") != "" || os.Getenv("LISTEN_FDS") != "" || os.Getenv("LISTEN_FDNAMES") != "" {
		panic("Can not unset envs")
	}

	c0, _ := listenersWithNames["fd1"][0].Accept()
	c1, _ := listenersWithNames["fd2"][0].Accept()

	// Write out the expected strings to the two pipes
	c0.Write([]byte("Hello world: fd1"))
	c1.Write([]byte("Goodbye world: fd2"))

	return
}
