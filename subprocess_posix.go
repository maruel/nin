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

//go:build !windows
// +build !windows

package nin

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// Subprocess wraps a single async subprocess.  It is entirely
// passive: it expects the caller to notify it when its fds are ready
// for reading, as well as call Finish() to reap the child once done()
// is true.
type SubprocessImpl struct {
	buf_        string
	fd_         int
	pid_        int
	useConsole_ bool
}

// SubprocessSet runs a ppoll/pselect() loop around a set of Subprocesses.
// DoWork() waits for any state change in subprocesses; finished_
// is a queue of subprocesses as they finish.
type SubprocessSetImpl struct {
	running_  []*SubprocessImpl
	finished_ []*SubprocessImpl // queue<Subprocess*>

	// Was static.
	// Store the signal number that causes the interruption.
	// 0 if not interruption.
	interrupted_ int

	/*
	  struct sigaction oldIntAct_
	  struct sigaction oldTermAct_
	  struct sigaction oldHupAct_
	  sigsetT oldMask_
	*/
}

func NewSubprocessOS(useConsole bool) *SubprocessImpl {
	return &SubprocessImpl{
		//fd_:-1, pid_:-1,
		useConsole_: useConsole,
	}
}

func (s *SubprocessSetImpl) IsInterrupted() bool { return s.interrupted_ != 0 }

func (s *SubprocessImpl) Close() error {
	/*
		if s.fd_ >= 0 {
			close(fd_)
		}
	*/
	// Reap child if forgotten.
	if s.pid_ != -1 {
		s.Finish()
	}
	return errors.New("implement me")
}

func (s *SubprocessImpl) Start(set *SubprocessSetImpl, command string) bool {
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	/*
			s.fd_ = r
		  // If available, we use ppoll in DoWork(); otherwise we use pselect
		  // and so must avoid overly-large FDs.
		  if s.fd_ >= staticCast<int>(FD_SETSIZE) {
		    Fatal("pipe: %s", strerror(EMFILE))
		  }
		  SetCloseOnExec(s.fd_)

		  var action posixSpawnFileActionsT
		  err := posixSpawnFileActionsInit(&action)
		  if err != 0 {
		    Fatal("posix_spawn_file_actions_init: %s", strerror(err))
		  }

		  err = posixSpawnFileActionsAddclose(&action, outputPipe[0])
		  if err != 0 {
		    Fatal("posix_spawn_file_actions_addclose: %s", strerror(err))
		  }

		  var attr posixSpawnattrT
		  err = posixSpawnattrInit(&attr)
		  if err != 0 {
		    Fatal("posix_spawnattr_init: %s", strerror(err))
		  }

		  flags := 0

		  flags |= POSIX_SPAWN_SETSIGMASK
		  err = posixSpawnattrSetsigmask(&attr, &set.oldMask_)
		  if err != 0 {
		    Fatal("posix_spawnattr_setsigmask: %s", strerror(err))
		  }
		  // Signals which are set to be caught in the calling process image are set to
		  // default action in the new process image, so no explicit
		  // POSIX_SPAWN_SETSIGDEF parameter is needed.

		  if !s.useConsole_ {
		    // Put the child in its own process group, so ctrl-c won't reach it.
		    flags |= POSIX_SPAWN_SETPGROUP
		    // No need to posixSpawnattrSetpgroup(&attr, 0), it's the default.

		    // Open /dev/null over stdin.
		    err = posixSpawnFileActionsAddopen(&action, 0, "/dev/null", O_RDONLY, 0)
		    if err != 0 {
		      Fatal("posix_spawn_file_actions_addopen: %s", strerror(err))
		    }

		    err = posixSpawnFileActionsAdddup2(&action, outputPipe[1], 1)
		    if err != 0 {
		      Fatal("posix_spawn_file_actions_adddup2: %s", strerror(err))
		    }
		    err = posixSpawnFileActionsAdddup2(&action, outputPipe[1], 2)
		    if err != 0 {
		      Fatal("posix_spawn_file_actions_adddup2: %s", strerror(err))
		    }
		    err = posixSpawnFileActionsAddclose(&action, outputPipe[1])
		    if err != 0 {
		      Fatal("posix_spawn_file_actions_addclose: %s", strerror(err))
		    }
		    // In the console case, outputPipe is still inherited by the child and
		    // closed when the subprocess finishes, which then notifies ninja.
		  }
		  flags |= POSIX_SPAWN_USEVFORK

		  err = posixSpawnattrSetflags(&attr, flags)
		  if err != 0 {
		    Fatal("posix_spawnattr_setflags: %s", strerror(err))
		  }

		  string spawnedArgs[] = { "/bin/sh", "-c", command, nil }
		  err = posixSpawn(&s.pid_, "/bin/sh", &action, &attr, constCast<char**>(spawnedArgs), environ)
		  if err != 0 {
		    Fatal("posix_spawn: %s", strerror(err))
		  }

		  err = posixSpawnattrDestroy(&attr)
		  if err != 0 {
		    Fatal("posix_spawnattr_destroy: %s", strerror(err))
		  }
		  err = posixSpawnFileActionsDestroy(&action)
		  if err != 0 {
		    Fatal("posix_spawn_file_actions_destroy: %s", strerror(err))
		  }
	*/
	_ = r.Close()
	_ = w.Close()
	panic("TODO")
	//return true
}

func (s *SubprocessImpl) OnPipeReady() {
	panic("TODO")
	/*
	  char buf[4 << 10]
	  len2 := read(s.fd_, buf, sizeof(buf))
	  if len2 > 0 {
	    s.buf_.append(buf, len2)
	  } else {
	    if len2 < 0 {
	      Fatal("read: %s", strerror(errno))
	    }
	    close(s.fd_)
	    s.fd_ = -1
	  }
	*/
}

func (s *SubprocessImpl) Finish() ExitStatus {
	panic("TODO")
	/*
	  if s.pid_ == -1 { panic("oops") }
	  status := 0
	  if waitpid(s.pid_, &status, 0) < 0 {
	    Fatal("waitpid(%d): %s", s.pid_, strerror(errno))
	  }
	  s.pid_ = -1

	  if WIFEXITED(status) && WEXITSTATUS(status) & 0x80 {
	    // Map the shell's exit code used for signal failure (128 + signal) to the
	    // status code expected by AIX WIFSIGNALED and WTERMSIG macros which, unlike
	    // other systems, uses a different bit layout.
	    signal := WEXITSTATUS(status) & 0x7f
	    status = (signal << 16) | signal
	  }

	  if WIFEXITED(status) {
	    exit := WEXITSTATUS(status)
	    if exit == 0 {
	      return ExitSuccess
	    }
	  } else if WIFSIGNALED(status) {
	    if WTERMSIG(status) == SIGINT || WTERMSIG(status) == SIGTERM || WTERMSIG(status) == SIGHUP {
	      return ExitInterrupted
	    }
	  }
	  return ExitFailure
	*/
}

func (s *SubprocessImpl) Done() bool {
	return s.fd_ == -1
}

func (s *SubprocessImpl) GetOutput() string {
	return s.buf_
}

/* We'll use context.Context instead.
int SubprocessSet::interrupted_

func (s *SubprocessSet) SetInterruptedFlag(signum int) {
  s.interrupted_ = signum
}

func (s *SubprocessSet) HandlePendingInterruption() {
  var pending sigsetT
  sigemptyset(&pending)
  if sigpending(&pending) == -1 {
    perror("ninja: sigpending")
    return
  }
  if sigismember(&pending, SIGINT) {
    s.interrupted_ = SIGINT
  } else if sigismember(&pending, SIGTERM) {
    s.interrupted_ = SIGTERM
  } else if sigismember(&pending, SIGHUP) {
    s.interrupted_ = SIGHUP
  }
}
*/

func NewSubprocessSetOS() SubprocessSet {
	panic("TODO")
	/*
	  sigsetT set
	  sigemptyset(&set)
	  sigaddset(&set, SIGINT)
	  sigaddset(&set, SIGTERM)
	  sigaddset(&set, SIGHUP)
	  if (sigprocmask(SIG_BLOCK, &set, &oldMask_) < 0)
	    Fatal("sigprocmask: %s", strerror(errno))

	  struct sigaction act
	  memset(&act, 0, sizeof(act))
	  act.saHandler = SetInterruptedFlag
	  if (sigaction(SIGINT, &act, &oldIntAct_) < 0)
	    Fatal("sigaction: %s", strerror(errno))
	  if (sigaction(SIGTERM, &act, &oldTermAct_) < 0)
	    Fatal("sigaction: %s", strerror(errno))
	  if (sigaction(SIGHUP, &act, &oldHupAct_) < 0)
	    Fatal("sigaction: %s", strerror(errno))
	*/
}

func (s *SubprocessSetImpl) Close() error {
	s.Clear()
	panic("TODO")
	/*
	   if (sigaction(SIGINT, &oldIntAct_, 0) < 0)
	     Fatal("sigaction: %s", strerror(errno))
	   if (sigaction(SIGTERM, &oldTermAct_, 0) < 0)
	     Fatal("sigaction: %s", strerror(errno))
	   if (sigaction(SIGHUP, &oldHupAct_, 0) < 0)
	     Fatal("sigaction: %s", strerror(errno))
	   if (sigprocmask(SIG_SETMASK, &oldMask_, 0) < 0)
	     Fatal("sigprocmask: %s", strerror(errno))
	*/
}

func (s *SubprocessSetImpl) Add(command string, useConsole bool) Subprocess {
	subprocess := NewSubprocessOS(useConsole)
	if !subprocess.Start(s, command) {
		_ = subprocess.Close()
		return nil
	}
	s.running_ = append(s.running_, subprocess)
	return subprocess
}

func (s *SubprocessSetImpl) DoWork() bool {
	panic("TODO")
	/*
		// TODO(maruel): Do not reallocate at every call.
		fds := make([]pollfd, 0, len(s.running_))
		for _, i := range s.running_ {
			fd := i.fd_
			if fd < 0 {
				continue
			}
			fds = append(fds, pollfd{fd, POLLIN | POLLPRI, 0})
		}

		s.interrupted_ = 0
		ret := syscall.Poll(&fds[0], len(fds), nil, &s.oldMask_)
		if ret == -1 {
			if errno != EINTR {
				perror("ninja: ppoll")
				return false
			}
			return s.IsInterrupted()
		}
		s.HandlePendingInterruption()

		if s.IsInterrupted() {
			return true
		}

		curNfd := 0
		for x := 0; x < len(s.running_); x++ {
			i := s.running_[x]
			fd := i.fd_
			if fd < 0 {
				continue
			}
			if fd != fds[curNfd].fd {
				panic("oops")
			}
			n := curNfd
			curNfd++
			if fds[n].revents {
				i.OnPipeReady()
				if i.Done() {
					s.finished_ = append(s.finished, i)
					x--
					if i < len(s.running_)-1 {
						copy(s.running_[i:], s.running_[i+1:])
					}
					s.running_ = s.running_[:len(s.running_)-1]
					continue
				}
			}
		}
		return s.IsInterrupted()
	*/
}

func (s *SubprocessSetImpl) NextFinished() Subprocess {
	if len(s.finished_) == 0 {
		return nil
	}
	subproc := s.finished_[0]
	s.finished_ = s.finished_[1:]
	return subproc
}

func (s *SubprocessSetImpl) Clear() {
	panic("TODO")
	/*
		for _, i := range s.running_ {
			// Since the foreground process is in our process group, it will receive
			// the interruption signal (i.e. SIGINT or SIGTERM) at the same time as us.
			if !i.useConsole_ {
				os.Kill(-i.pid_, s.interrupted_)
			}
		}
		for _, i := range s.running_ {
			i.Close()
		}
		s.running_ = nil
	*/
}

func (s *SubprocessGeneric) osSpecific(cmd *exec.Cmd, c string) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		//Setpgid: true,
	}
}
