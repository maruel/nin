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

package ginja

import (
	"bytes"
	"os/exec"
	"runtime"
)

// The Go runtime already handles poll under the hood so this abstraction layer
// has to be replaced; unless we realize that the Go runtime is too slow.

type Subprocess interface {
	Done() bool
	Close() error
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

// SubprocessGeneric is the dumbest implmentation, just to get going.
type SubprocessGeneric struct {
	buf          bytes.Buffer
	cmd          *exec.Cmd
	done         bool
	use_console_ bool
}

func (s *SubprocessGeneric) Done() bool {
	return s.done
}

func (s *SubprocessGeneric) Close() error {
	return s.cmd.Wait()
}

func (s *SubprocessGeneric) Finish() ExitStatus {
	s.cmd.Wait()
	return s.cmd.ProcessState.ExitCode()
}

func (s *SubprocessGeneric) GetOutput() string {
	return s.buf.String()
}

type SubprocessSetGeneric struct {
	running_  []*SubprocessGeneric
	finished_ []*SubprocessGeneric // queue<Subprocess*>
	// TODO(maruel): In Go, we'll use a context.Context.
	interrupted_ int
}

func NewSubprocessSet() *SubprocessSetGeneric {
	return &SubprocessSetGeneric{}
}

func (s *SubprocessSetGeneric) Clear() {
	for _, p := range s.running_ {
		// TODO(maruel): This is incorrect, we want to use -pid for process group
		// on posix.
		if !p.use_console_ {
			p.cmd.Process.Kill()
		}
	}
	for _, p := range s.running_ {
		p.Close()
	}
	s.running_ = nil
}

func (s *SubprocessSetGeneric) Running() int {
	return len(s.running_)
}

func (s *SubprocessSetGeneric) Finished() int {
	return len(s.finished_)
}

func (s *SubprocessSetGeneric) Add(c string, use_console bool) Subprocess {
	// TODO(maruel): That's very bad. The goal is just to get going. Ninja
	// handles unparsed command lines but Go expects parsed command lines, so
	// care will be needed to make it fully work.
	shell := "bash"
	flag := "-c"
	if runtime.GOOS == "windows" {
		shell = "cmd.exe"
		flag = "/c"
	}
	subproc := &SubprocessGeneric{
		use_console_: use_console,
		cmd:          exec.Command(shell, flag, c),
	}
	// Ignore the parsed arguments on Windows and feedback the original string.
	if runtime.GOOS == "windows" {
		//subproc.cmd.SysProcAttr.CmdLine = c
	}
	subproc.cmd.Stdout = &subproc.buf
	subproc.cmd.Stderr = &subproc.buf
	if err := subproc.cmd.Start(); err != nil {
		// TODO(maruel): Error handing
		panic(err)
	}
	if subproc.cmd.ProcessState == nil {
		// This generally means that something bad happened. Calling Wait() seems
		// to initialize ProcessState.
		subproc.cmd.Wait()
		if subproc.cmd.ProcessState == nil {
			panic("expected ProcessState to be set")
		}
	}
	s.running_ = append(s.running_, subproc)
	return subproc
}

func (s *SubprocessSetGeneric) NextFinished() Subprocess {
	if len(s.finished_) == 0 {
		return nil
	}
	// TODO(maruel): The original code use a dequeue. Once in a while, the
	// pointer should be reset back to the top of the slice instead of being a
	// memory leak.
	//
	// On the other hand, the current worse case scenarios is 200k processes, so
	// that's 800KiB of RAM wasted at worst.
	subproc := s.finished_[0]
	s.finished_ = s.finished_[1:]
	return subproc
}

// DoWork should return on one of 3 events:
//
//  - Was interrupted, return true
//  - A process completed, return true
//  - A pipe got data, returns false
//
// In Go, the later can't happen. So hard block in an inefficient way (for
// now).
func (s *SubprocessSetGeneric) DoWork() bool {
	o := false
	for i := 0; i < len(s.running_); i++ {
		p := s.running_[i]
		if p.cmd.ProcessState.Exited() {
			o = true
			p.done = true
			s.finished_ = append(s.finished_, p)
			if i < len(s.running_)-1 {
				copy(s.running_[i:], s.running_[i+1:])
			}
			i--
			s.running_ = s.running_[:len(s.running_)-1]
		}
	}
	return o
}
