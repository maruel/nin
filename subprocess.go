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
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

// The Go runtime already handles poll under the hood so this abstraction layer
// has to be replaced; unless we realize that the Go runtime is too slow.

type Subprocess interface {
	// Query if the process is done.
	Done() bool
	// Only to be called after the process is done.
	Finish() ExitStatus
	GetOutput() string
}

type SubprocessSet interface {
	Clear()
	Running() int
	Finished() int
	Add(command string, use_console bool) Subprocess
	NextFinished() Subprocess
	DoWork() bool
}

// SubprocessGeneric is the dumbest implementation, just to get going.
type SubprocessGeneric struct {
	done     int32
	exitCode int32
	buf      string
}

// Done is only used in tests.
func (s *SubprocessGeneric) Done() bool {
	return atomic.LoadInt32(&s.done) != 0
}

func (s *SubprocessGeneric) Finish() ExitStatus {
	return ExitStatus(s.exitCode)
}

func (s *SubprocessGeneric) GetOutput() string {
	return s.buf
}

func (s *SubprocessGeneric) run(ctx context.Context, c string, use_console bool) {
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
	if use_console {
		cmd = exec.Command(ex, args...)
	} else {
		cmd = exec.CommandContext(ctx, ex, args...)
	}
	buf := bytes.Buffer{}
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	s.osSpecific(cmd, c)
	_ = cmd.Run()
	// Skip a memory copy.
	s.buf = unsafeString(buf.Bytes())
	// TODO(maruel): For compatibility with ninja, use ExitInterrupted (2) for
	// interrupted?
	s.exitCode = int32(cmd.ProcessState.ExitCode())
}

type SubprocessSetGeneric struct {
	ctx       context.Context
	cancel    func()
	wg        sync.WaitGroup
	procDone  chan *SubprocessGeneric
	mu        sync.Mutex
	running_  []*SubprocessGeneric
	finished_ []*SubprocessGeneric
}

func NewSubprocessSet() *SubprocessSetGeneric {
	ctx, cancel := context.WithCancel(context.Background())
	return &SubprocessSetGeneric{
		ctx:      ctx,
		cancel:   cancel,
		procDone: make(chan *SubprocessGeneric),
	}
}

func (s *SubprocessSetGeneric) Clear() {
	s.cancel()
	s.wg.Wait()
	// TODO(maruel): This is still broken, since the goroutines are stuck on
	// s.procDone <- subproc.
}

func (s *SubprocessSetGeneric) Running() int {
	s.mu.Lock()
	r := len(s.running_)
	s.mu.Unlock()
	return r
}

func (s *SubprocessSetGeneric) Finished() int {
	s.mu.Lock()
	f := len(s.finished_)
	s.mu.Unlock()
	return f
}

func (s *SubprocessSetGeneric) Add(c string, use_console bool) Subprocess {
	subproc := &SubprocessGeneric{}
	s.wg.Add(1)
	go s.enqueue(subproc, c, use_console)
	s.mu.Lock()
	s.running_ = append(s.running_, subproc)
	s.mu.Unlock()
	return subproc
}

func (s *SubprocessSetGeneric) enqueue(subproc *SubprocessGeneric, c string, use_console bool) {
	subproc.run(s.ctx, c, use_console)
	// Do it before sending the channel because procDone is a blocking channel
	// and the caller relies on Running() == 0 && Finished() == 0. Otherwise
	// Clear() would hang.
	s.wg.Done()
	s.procDone <- subproc
}

func (s *SubprocessSetGeneric) NextFinished() Subprocess {
	s.mu.Lock()
	var subproc Subprocess
	if len(s.finished_) != 0 {
		// LIFO queue.
		subproc = s.finished_[len(s.finished_)-1]
		s.finished_ = s.finished_[:len(s.finished_)-1]
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
func (s *SubprocessSetGeneric) DoWork() bool {
	o := false
	for {
		select {
		case p := <-s.procDone:
			// TODO(maruel): Do a perf compare with a map[*SubprocessGeneric]struct{}.
			s.mu.Lock()
			i := 0
			for i = range s.running_ {
				if s.running_[i] == p {
					break
				}
			}
			s.finished_ = append(s.finished_, p)
			if i < len(s.running_)-1 {
				copy(s.running_[i:], s.running_[i+1:])
			}
			s.running_ = s.running_[:len(s.running_)-1]
			s.mu.Unlock()
			// The unit tests expect that Subprocess.Done() is only true once the
			// subprocess has been added to finished.
			atomic.StoreInt32(&p.done, 1)
		default:
			return o
		}
	}
}
