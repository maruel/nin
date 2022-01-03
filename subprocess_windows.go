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
	"os/exec"
	"syscall"
)

// Subprocess wraps a single async subprocess.  It is entirely
// passive: it expects the caller to notify it when its fds are ready
// for reading, as well as call Finish() to reap the child once done()
// is true.
type SubprocessImpl struct {
	buf_ bytes.Buffer
	/*
		child_          HANDLE
		pipe_           HANDLE
		overlapped_     OVERLAPPED
		overlappedBuf_ [4 << 10]byte
		isReading_     bool
	*/
	useConsole_ bool
}

// SubprocessSet runs a ppoll/pselect() loop around a set of Subprocesses.
// DoWork() waits for any state change in subprocesses; finished_
// is a queue of subprocesses as they finish.
type SubprocessSetImpl struct {
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
	  struct sigaction oldIntAct_
	  struct sigaction oldTermAct_
	  struct sigaction oldHupAct_
	  sigsetT oldMask_
	*/
}

func NewSubprocessOS(useConsole bool) *SubprocessImpl {
	return &SubprocessImpl{
		useConsole_: useConsole,
	}
}

func (s *SubprocessImpl) Close() error {
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

func (s *SubprocessImpl) SetupPipe(ioport HANDLE) HANDLE {
	panic("TODO")
	/*
	  char pipeName[100]
	  snprintf(pipeName, sizeof(pipeName), "\\\\.\\pipe\\ninja_pid%lu_sp%p", GetCurrentProcessId(), this)

	  s.pipe_ = ::CreateNamedPipeA(pipeName, PIPE_ACCESS_INBOUND | FILE_FLAG_OVERLAPPED, PIPE_TYPE_BYTE, PIPE_UNLIMITED_INSTANCES, 0, 0, INFINITE, nil)
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
	  HANDLE outputWriteHandle =
	      CreateFileA(pipeName, GENERIC_WRITE, 0, nil, OPEN_EXISTING, 0, nil)
	  var outputWriteChild HANDLE
	  if !DuplicateHandle(GetCurrentProcess(), outputWriteHandle, GetCurrentProcess(), &outputWriteChild, 0, TRUE, DUPLICATE_SAME_ACCESS) {
	    Win32Fatal("DuplicateHandle")
	  }
	  CloseHandle(outputWriteHandle)

	  return outputWriteChild
	*/
}

func (s *SubprocessImpl) Start(set *SubprocessSetImpl, command string) bool {
	panic("TODO")
	/*
		  childPipe := SetupPipe(set.ioport_)

		  var securityAttributes SECURITY_ATTRIBUTES
		  memset(&securityAttributes, 0, sizeof(SECURITY_ATTRIBUTES))
		  securityAttributes.nLength = sizeof(SECURITY_ATTRIBUTES)
		  securityAttributes.bInheritHandle = TRUE
		  // Must be inheritable so subprocesses can dup to children.
		  HANDLE nul =
		      CreateFileA("NUL", GENERIC_READ, FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE, &securityAttributes, OPEN_EXISTING, 0, nil)
		  if nul == INVALID_HANDLE_VALUE {
		    Fatal("couldn't open nul")
		  }

		  var startupInfo STARTUPINFOA
		  memset(&startupInfo, 0, sizeof(startupInfo))
		  startupInfo.cb = sizeof(STARTUPINFO)
		  if !s.useConsole_ {
		    startupInfo.dwFlags = STARTF_USESTDHANDLES
		    startupInfo.hStdInput = nul
		    startupInfo.hStdOutput = childPipe
		    startupInfo.hStdError = childPipe
		  }
		  // In the console case, childPipe is still inherited by the child and closed
		  // when the subprocess finishes, which then notifies ninja.

		  var processInfo PROCESS_INFORMATION
		  memset(&processInfo, 0, sizeof(processInfo))

		  // Ninja handles ctrl-c, except for subprocesses in console pools.
		  DWORD processFlags = s.useConsole_ ? 0 : CREATE_NEW_PROCESS_GROUP

		  // Do not prepend 'cmd /c' on Windows, this breaks command
		  // lines greater than 8,191 chars.
			// inheritHandles = TRUE
		  if !CreateProcessA(nil, (char*)command, nil, nil, TRUE, processFlags, nil, nil, &startupInfo, &processInfo) {
		    error := GetLastError()
		    if error == ERROR_FILE_NOT_FOUND {
		      // File (program) not found error is treated as a normal build
		      // action failure.
		      if childPipe {
		        CloseHandle(childPipe)
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
		  if childPipe {
		    CloseHandle(childPipe)
		  }
		  CloseHandle(nul)

		  CloseHandle(processInfo.hThread)
		  s.child_ = processInfo.hProcess

		  return true
	*/
}

func (s *SubprocessImpl) OnPipeReady() {
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

	  if s.isReading_ && bytes {
	    s.buf_.append(s.overlappedBuf_, bytes)
	  }

	  memset(&s.overlapped_, 0, sizeof(s.overlapped_))
	  s.isReading_ = true
	  if !::ReadFile(s.pipe_, s.overlappedBuf_, sizeof(s.overlappedBuf_), &bytes, &s.overlapped_) {
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

func (s *SubprocessImpl) Finish() ExitStatus {
	panic("TODO")
	/*
	  if !s.child_ {
	    return ExitFailure
	  }

	  // TODO: add error handling for all of these.
	  WaitForSingleObject(s.child_, INFINITE)

	  exitCode := 0
	  GetExitCodeProcess(s.child_, &exitCode)

	  CloseHandle(s.child_)
	  s.child_ = nil

	  return exitCode == 0              ? ExitSuccess :
	         exitCode == CONTROL_C_EXIT ? ExitInterrupted :
	                                       ExitFailure
	*/
}

func (s *SubprocessImpl) Done() bool {
	panic("TODO")
	//return s.pipe_ == nil
}

func (s *SubprocessImpl) GetOutput() string {
	return s.buf_.String()
}

//HANDLE SubprocessSet::ioport_

func NewSubprocessSetOS() *SubprocessSetImpl {
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

func (s *SubprocessSetImpl) Close() error {
	s.Clear()
	panic("TODO")
	//SetConsoleCtrlHandler(NotifyInterrupted, FALSE)
	//CloseHandle(ioport_)
}

/*
func (s *SubprocessSetImpl) NotifyInterrupted(dwCtrlType DWORD) BOOL WINAPI {
  if dwCtrlType == CTRL_C_EVENT || dwCtrlType == CTRL_BREAK_EVENT {
    if !PostQueuedCompletionStatus(s.ioport_, 0, 0, nil) {
      Win32Fatal("PostQueuedCompletionStatus")
    }
    return TRUE
  }

  return FALSE
}
*/

func (s *SubprocessSetImpl) Add(command string, useConsole bool) Subprocess {
	subprocess := NewSubprocessOS(useConsole)
	if !subprocess.Start(s, command) {
		_ = subprocess.Close()
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

func (s *SubprocessSetImpl) DoWork() bool {
	panic("TODO")
	/*
	  var bytesRead DWORD
	  var subproc *Subprocess
	  var overlapped *OVERLAPPED

	  if !GetQueuedCompletionStatus(s.ioport_, &bytesRead, (PULONG_PTR)&subproc, &overlapped, INFINITE) {
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

func (s *SubprocessSetImpl) NextFinished() *Subprocess {
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
			// Since the foreground process is in our process group, it will receive a
			// CTRL_C_EVENT or CTRL_BREAK_EVENT at the same time as us.
			if i.child_ != nil && !i.useConsole_ {
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

func (s *SubprocessGeneric) osSpecific(cmd *exec.Cmd, c string) {
	// Ignore the parsed arguments on Windows and feed back the original string.
	// See https://pkg.go.dev/os/exec#Command for an explanation.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		//CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		CmdLine: c,
	}
	cmd.Args = nil
}
