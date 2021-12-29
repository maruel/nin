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
	use_console_ bool
}

func (s *SubprocessGeneric) Done() bool {
	if s.cmd.ProcessState == nil {
		// Process failed?
		return true
	}
	return s.cmd.ProcessState.Exited()
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
	s.running_ = append(s.running_, subproc)
	return subproc
}

func (s *SubprocessSetGeneric) NextFinished() Subprocess {
	if len(s.finished_) == 0 {
		return nil
	}
	subproc := s.finished_[0]
	s.finished_ = s.finished_[1:]
	return subproc
}

func (s *SubprocessSetGeneric) DoWork() bool {
	o := false
	for i := 0; i < len(s.running_); i++ {
		p := s.running_[i]
		if p.Done() {
			o = true
			s.finished_ = append(s.finished_, p)
			i--
			if i < len(s.running_)-1 {
				copy(s.running_[i:], s.running_[i+1:])
			}
			s.running_ = s.running_[:len(s.running_)-1]
			continue
		}
	}
	return o
}
