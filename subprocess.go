// Copyright 2012 Google Inc. All Rights Reserved.
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

package nin

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

// The Go runtime already handles poll under the hood so this abstraction layer
// has to be replaced; unless we realize that the Go runtime is too slow.

// subprocess is the dumbest implementation, just to get going.
type subprocess struct {
	done     int32
	exitCode int32
	buf      string
}

// Done queries if the process is done.
//
// Only used in tests.
func (s *subprocess) Done() bool {
	return atomic.LoadInt32(&s.done) != 0
}

// Finish returns the exit code. Must only to be called after the process is
// done.
func (s *subprocess) Finish() ExitStatus {
	return ExitStatus(s.exitCode)
}

func (s *subprocess) GetOutput() string {
	return s.buf
}

func (s *subprocess) run(ctx context.Context, c string, useConsole bool) {
	ex := ""
	var args []string
	if runtime.GOOS == "windows" {
		// TODO(maruel): Handle quoted space. It's only necessary from the
		// perspective of finding the primary executable to run.
		i := strings.IndexByte(c, ' ')
		if i == -1 {
			ex = c
		} else {
			ex = c[:i]
		}
		args = []string{c}
	} else {
		// The commands being run use shell redirection. The C++ version uses
		// system() which will use the default shell. I'd like to try to have a
		// fast-track mode where if no shell escape characters are used, the
		// command is ran without a shell.
		ex = "/bin/sh"
		args = []string{"-c", c}
	}
	// Ignore the parsed arguments on Windows and feedback the original string.
	var cmd *exec.Cmd
	if useConsole {
		cmd = exec.Command(ex, args...)
	} else {
		cmd = exec.CommandContext(ctx, ex, args...)
	}
	// TODO(maruel): The C++ code is fairly involved in its way to setup the
	// process, the code here is fairly naive.
	// TODO(maruel): When useConsole is false, it should be in a new process
	// group on posix.
	buf := bytes.Buffer{}
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if useConsole {
		cmd.Stdin = os.Stdin
	}
	s.osSpecific(cmd, c, useConsole)
	_ = cmd.Run()
	// Skip a memory copy.
	s.buf = unsafeString(buf.Bytes())
	// TODO(maruel): For compatibility with ninja, use ExitInterrupted (2) for
	// interrupted?
	s.exitCode = int32(cmd.ProcessState.ExitCode())
}

type subprocessSet struct {
	ctx      context.Context
	cancel   func()
	wg       sync.WaitGroup
	procDone chan *subprocess
	mu       sync.Mutex
	running  []*subprocess
	finished []*subprocess
}

func NewSubprocessSet() *subprocessSet {
	ctx, cancel := context.WithCancel(context.Background())
	return &subprocessSet{
		ctx:      ctx,
		cancel:   cancel,
		procDone: make(chan *subprocess),
	}
}

// Clear interrupts all the children processes.
//
// TODO(maruel): Use a context instead.
func (s *subprocessSet) Clear() {
	s.cancel()
	s.wg.Wait()
	// TODO(maruel): This is still broken, since the goroutines are stuck on
	// s.procDone <- subproc.
}

// Running returns the number of running processes.
func (s *subprocessSet) Running() int {
	s.mu.Lock()
	r := len(s.running)
	s.mu.Unlock()
	return r
}

// Finished returns the number of processes to parse their output.
func (s *subprocessSet) Finished() int {
	s.mu.Lock()
	f := len(s.finished)
	s.mu.Unlock()
	return f
}

// Add starts a new child process.
func (s *subprocessSet) Add(c string, useConsole bool) *subprocess {
	subproc := &subprocess{}
	s.wg.Add(1)
	go s.enqueue(subproc, c, useConsole)
	s.mu.Lock()
	s.running = append(s.running, subproc)
	s.mu.Unlock()
	return subproc
}

func (s *subprocessSet) enqueue(subproc *subprocess, c string, useConsole bool) {
	subproc.run(s.ctx, c, useConsole)
	// Do it before sending the channel because procDone is a blocking channel
	// and the caller relies on Running() == 0 && Finished() == 0. Otherwise
	// Clear() would hang.
	s.wg.Done()
	s.procDone <- subproc
}

// NextFinished returns the next finished child process.
func (s *subprocessSet) NextFinished() *subprocess {
	s.mu.Lock()
	var subproc *subprocess
	if len(s.finished) != 0 {
		// LIFO queue.
		subproc = s.finished[len(s.finished)-1]
		s.finished = s.finished[:len(s.finished)-1]
	}
	s.mu.Unlock()
	return subproc
}

// DoWork should return on one of 3 events:
//
//  - Was interrupted, return true
//  - A process completed, return false
//  - A pipe got data, returns false
//
// In Go, the later can't happen.
func (s *subprocessSet) DoWork() bool {
	o := false
	for {
		select {
		case p := <-s.procDone:
			// TODO(maruel): Do a perf compare with a map[*Subprocess]struct{}.
			s.mu.Lock()
			i := 0
			for i = range s.running {
				if s.running[i] == p {
					break
				}
			}
			s.finished = append(s.finished, p)
			if i < len(s.running)-1 {
				copy(s.running[i:], s.running[i+1:])
			}
			s.running = s.running[:len(s.running)-1]
			s.mu.Unlock()
			// The unit tests expect that Subprocess.Done() is only true once the
			// subprocess has been added to finished.
			atomic.StoreInt32(&p.done, 1)
		default:
			return o
		}
	}
}
