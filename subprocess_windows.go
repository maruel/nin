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

// Subprocess wraps a single async subprocess.  It is entirely
// passive: it expects the caller to notify it when its fds are ready
// for reading, as well as call Finish() to reap the child once done()
// is true.
type SubprocessImpl struct {
	buf_            string
	child_          HANDLE
	pipe_           HANDLE
	overlapped_     OVERLAPPED
	overlapped_buf_ [4 << 10]byte
	is_reading_     bool
	use_console_    bool
}

// SubprocessSet runs a ppoll/pselect() loop around a set of Subprocesses.
// DoWork() waits for any state change in subprocesses; finished_
// is a queue of subprocesses as they finish.
type SubprocessSet struct {
	running_  []*Subprocess
	finished_ []*Subprocess // queue<Subprocess*>

	// Windows
	//static HANDLE ioport_

	// POSIX
	// Store the signal number that causes the interruption.
	// 0 if not interruption.
	//static int interrupted_

	//static bool IsInterrupted() { return interrupted_ != 0; }
	/*
	  struct sigaction old_int_act_
	  struct sigaction old_term_act_
	  struct sigaction old_hup_act_
	  sigset_t old_mask_
	*/
}

func NewSubprocessOS(use_console bool) *Subprocess {
	return &Subprocess{
		use_console_: use_console,
	}
}

func (s *Subprocess) Close() error {
	panic("TODO")
	/*
		  if s.pipe_ != nil {
		    if (!CloseHandle(s.pipe_))
		      Win32Fatal("CloseHandle")
		  }
		  // Reap child if forgotten.
		  if s.child_ != nil {
		    s.Finish()
			}
	*/
}

type HANDLE uintptr

func (s *Subprocess) SetupPipe(ioport HANDLE) HANDLE {
	panic("TODO")
	/*
	  char pipe_name[100]
	  snprintf(pipe_name, sizeof(pipe_name), "\\\\.\\pipe\\ninja_pid%lu_sp%p", GetCurrentProcessId(), this)

	  s.pipe_ = ::CreateNamedPipeA(pipe_name, PIPE_ACCESS_INBOUND | FILE_FLAG_OVERLAPPED, PIPE_TYPE_BYTE, PIPE_UNLIMITED_INSTANCES, 0, 0, INFINITE, nil)
	  if s.pipe_ == INVALID_HANDLE_VALUE {
	    Win32Fatal("CreateNamedPipe")
	  }

	  if !CreateIoCompletionPort(s.pipe_, ioport, (ULONG_PTR)this, 0) {
	    Win32Fatal("CreateIoCompletionPort")
	  }

	  memset(&s.overlapped_, 0, sizeof(s.overlapped_))
	  if !ConnectNamedPipe(s.pipe_, &s.overlapped_) && GetLastError() != ERROR_IO_PENDING {
	    Win32Fatal("ConnectNamedPipe")
	  }

	  // Get the write end of the pipe as a handle inheritable across processes.
	  HANDLE output_write_handle =
	      CreateFileA(pipe_name, GENERIC_WRITE, 0, nil, OPEN_EXISTING, 0, nil)
	  var output_write_child HANDLE
	  if !DuplicateHandle(GetCurrentProcess(), output_write_handle, GetCurrentProcess(), &output_write_child, 0, TRUE, DUPLICATE_SAME_ACCESS) {
	    Win32Fatal("DuplicateHandle")
	  }
	  CloseHandle(output_write_handle)

	  return output_write_child
	*/
}

func (s *Subprocess) Start(set *SubprocessSet, command string) bool {
	panic("TODO")
	/*
		  child_pipe := SetupPipe(set.ioport_)

		  var security_attributes SECURITY_ATTRIBUTES
		  memset(&security_attributes, 0, sizeof(SECURITY_ATTRIBUTES))
		  security_attributes.nLength = sizeof(SECURITY_ATTRIBUTES)
		  security_attributes.bInheritHandle = TRUE
		  // Must be inheritable so subprocesses can dup to children.
		  HANDLE nul =
		      CreateFileA("NUL", GENERIC_READ, FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE, &security_attributes, OPEN_EXISTING, 0, nil)
		  if nul == INVALID_HANDLE_VALUE {
		    Fatal("couldn't open nul")
		  }

		  var startup_info STARTUPINFOA
		  memset(&startup_info, 0, sizeof(startup_info))
		  startup_info.cb = sizeof(STARTUPINFO)
		  if !s.use_console_ {
		    startup_info.dwFlags = STARTF_USESTDHANDLES
		    startup_info.hStdInput = nul
		    startup_info.hStdOutput = child_pipe
		    startup_info.hStdError = child_pipe
		  }
		  // In the console case, child_pipe is still inherited by the child and closed
		  // when the subprocess finishes, which then notifies ninja.

		  var process_info PROCESS_INFORMATION
		  memset(&process_info, 0, sizeof(process_info))

		  // Ninja handles ctrl-c, except for subprocesses in console pools.
		  DWORD process_flags = s.use_console_ ? 0 : CREATE_NEW_PROCESS_GROUP

		  // Do not prepend 'cmd /c' on Windows, this breaks command
		  // lines greater than 8,191 chars.
			// inherit_handles = TRUE
		  if !CreateProcessA(nil, (char*)command, nil, nil, TRUE, process_flags, nil, nil, &startup_info, &process_info) {
		    error := GetLastError()
		    if error == ERROR_FILE_NOT_FOUND {
		      // File (program) not found error is treated as a normal build
		      // action failure.
		      if child_pipe {
		        CloseHandle(child_pipe)
		      }
		      CloseHandle(s.pipe_)
		      CloseHandle(nul)
		      s.pipe_ = nil
		      // child_ is already NULL;
		      s.buf_ = "CreateProcess failed: The system cannot find the file specified.\n"
		      return true
		    } else {
		      fmt.Fprintf(os.Stderr, "\nCreateProcess failed. Command attempted:\n\"%s\"\n", command)
		      hint := nil
		      // ERROR_INVALID_PARAMETER means the command line was formatted
		      // incorrectly. This can be caused by a command line being too long or
		      // leading whitespace in the command. Give extra context for this case.
		      if error == ERROR_INVALID_PARAMETER {
		        if command.length() > 0 && (command[0] == ' ' || command[0] == '\t') {
		          hint = "command contains leading whitespace"
		        } else {
		          hint = "is the command line too long?"
		        }
		      }
		      Win32Fatal("CreateProcess", hint)
		    }
		  }

		  // Close pipe channel only used by the child.
		  if child_pipe {
		    CloseHandle(child_pipe)
		  }
		  CloseHandle(nul)

		  CloseHandle(process_info.hThread)
		  s.child_ = process_info.hProcess

		  return true
	*/
}

func (s *Subprocess) OnPipeReady() {
	panic("TODO")
	/*
	  var bytes DWORD
	  if !GetOverlappedResult(s.pipe_, &s.overlapped_, &bytes, TRUE) {
	    if GetLastError() == ERROR_BROKEN_PIPE {
	      CloseHandle(s.pipe_)
	      s.pipe_ = nil
	      return
	    }
	    Win32Fatal("GetOverlappedResult")
	  }

	  if s.is_reading_ && bytes {
	    s.buf_.append(s.overlapped_buf_, bytes)
	  }

	  memset(&s.overlapped_, 0, sizeof(s.overlapped_))
	  s.is_reading_ = true
	  if !::ReadFile(s.pipe_, s.overlapped_buf_, sizeof(s.overlapped_buf_), &bytes, &s.overlapped_) {
	    if GetLastError() == ERROR_BROKEN_PIPE {
	      CloseHandle(s.pipe_)
	      s.pipe_ = nil
	      return
	    }
	    if GetLastError() != ERROR_IO_PENDING {
	      Win32Fatal("ReadFile")
	    }
	  }

	  // Even if we read any bytes in the readfile call, we'll enter this
	  // function again later and get them at that point.
	*/
}

func (s *Subprocess) Finish() ExitStatus {
	panic("TODO")
	/*
	  if !s.child_ {
	    return ExitFailure
	  }

	  // TODO: add error handling for all of these.
	  WaitForSingleObject(s.child_, INFINITE)

	  exit_code := 0
	  GetExitCodeProcess(s.child_, &exit_code)

	  CloseHandle(s.child_)
	  s.child_ = nil

	  return exit_code == 0              ? ExitSuccess :
	         exit_code == CONTROL_C_EXIT ? ExitInterrupted :
	                                       ExitFailure
	*/
}

func (s *Subprocess) Done() bool {
	panic("TODO")
	//return s.pipe_ == nil
}

func (s *Subprocess) GetOutput() string {
	return s.buf_
}

//HANDLE SubprocessSet::ioport_

func NewSubprocessSetOS() *SubprocessSet {
	panic("TODO")
	/*
		  ioport_ = ::CreateIoCompletionPort(INVALID_HANDLE_VALUE, nil, 0, 1)
		  if ioport_ == nil {
		    Win32Fatal("CreateIoCompletionPort")
			}
		  if !SetConsoleCtrlHandler(NotifyInterrupted, TRUE) {
		    Win32Fatal("SetConsoleCtrlHandler")
			}
	*/
}

func (s *SubprocessSet) Close() error {
	s.Clear()
	panic("TODO")
	//SetConsoleCtrlHandler(NotifyInterrupted, FALSE)
	//CloseHandle(ioport_)
}

/*
func (s *SubprocessSet) NotifyInterrupted(dwCtrlType DWORD) BOOL WINAPI {
  if dwCtrlType == CTRL_C_EVENT || dwCtrlType == CTRL_BREAK_EVENT {
    if !PostQueuedCompletionStatus(s.ioport_, 0, 0, nil) {
      Win32Fatal("PostQueuedCompletionStatus")
    }
    return TRUE
  }

  return FALSE
}
*/

func (s *SubprocessSet) Add(command string, use_console bool) *Subprocess {
	subprocess := NewSubprocessOS(use_console)
	if !subprocess.Start(s, command) {
		subprocess.Close()
		return nil
	}
	panic("TODO")
	/*
		if subprocess.child_ != nil {
			s.running_ = append(s.running_, subprocess)
		} else {
			s.finished_ = append(s.finished_, subprocess)
		}
		return subprocess
	*/
}

func (s *SubprocessSet) DoWork() bool {
	panic("TODO")
	/*
	  var bytes_read DWORD
	  var subproc *Subprocess
	  var overlapped *OVERLAPPED

	  if !GetQueuedCompletionStatus(s.ioport_, &bytes_read, (PULONG_PTR)&subproc, &overlapped, INFINITE) {
	    if GetLastError() != ERROR_BROKEN_PIPE {
	      Win32Fatal("GetQueuedCompletionStatus")
	    }
	  }

	  if subproc == nil { // A NULL subproc indicates that we were interrupted and is
	                // delivered by NotifyInterrupted above.
	  }
	    return true

	  subproc.OnPipeReady()

	  if subproc.Done() {
	    vector<Subprocess*>::iterator end =
	        remove(s.running_.begin(), s.running_.end(), subproc)
	    if s.running_.end() != end {
	      s.finished_.push(subproc)
	      s.running_.resize(end - s.running_.begin())
	    }
	  }

	  return false
	*/
}

func (s *SubprocessSet) NextFinished() *Subprocess {
	if len(s.finished_) == 0 {
		return nil
	}
	subproc := s.finished_[0]
	s.finished_ = s.finished_[1:]
	return subproc
}

func (s *SubprocessSet) Clear() {
	panic("TODO")
	/*
		for _, i := range s.running_ {
			// Since the foreground process is in our process group, it will receive a
			// CTRL_C_EVENT or CTRL_BREAK_EVENT at the same time as us.
			if i.child_ != nil && !i.use_console_ {
				if !GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, GetProcessId((*i).child_)) {
					Win32Fatal("GenerateConsoleCtrlEvent")
				}
			}
		}
		for _, i := range s.running_ {
			i.Close()
		}
		s.running_ = nil
	*/
}
