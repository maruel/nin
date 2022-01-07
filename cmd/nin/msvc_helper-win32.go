// Copyright 2011 Google Inc. All Rights Reserved.
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

package main

import "strings"

func escapeForDepfile(path string) string {
	// Depfiles don't escape single \.
	return strings.ReplaceAll(path, " ", "\\ ")
}

func (c *clWrapper) Run(command string, output *string) int {
	panic("TODO")
	/*
		  SECURITY_ATTRIBUTES securityAttributes = {}
		  securityAttributes.nLength = sizeof(SECURITY_ATTRIBUTES)
		  securityAttributes.bInheritHandle = TRUE

		  // Must be inheritable so subprocesses can dup to children.
		  HANDLE nul =
		      CreateFileA("NUL", GENERIC_READ, FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE, &securityAttributes, OPEN_EXISTING, 0, nil)
		  if nul == INVALID_HANDLE_VALUE {
		    Fatal("couldn't open nul")
		  }

		  HANDLE stdoutRead, stdoutWrite
		  if !CreatePipe(&stdoutRead, &stdoutWrite, &securityAttributes, 0) {
		    Win32Fatal("CreatePipe")
		  }

		  if !SetHandleInformation(stdoutRead, HANDLE_FLAG_INHERIT, 0) {
		    Win32Fatal("SetHandleInformation")
		  }

		  PROCESS_INFORMATION processInfo = {}
		  STARTUPINFOA startupInfo = {}
		  startupInfo.cb = sizeof(STARTUPINFOA)
		  startupInfo.hStdInput = nul
		  startupInfo.hStdError = ::GetStdHandle(STD_ERROR_HANDLE)
		  startupInfo.hStdOutput = stdoutWrite
		  startupInfo.dwFlags |= STARTF_USESTDHANDLES

			// inherit handles = TRUE
		  if !CreateProcessA(nil, (char*)command, nil, nil,  TRUE, 0, c.envBlock, nil, &startupInfo, &processInfo) {
		    Win32Fatal("CreateProcess")
		  }

		  if !CloseHandle(nul) || !CloseHandle(stdoutWrite) {
		    Win32Fatal("CloseHandle")
		  }

		  // Read all output of the subprocess.
		  readLen := 1
		  for readLen {
		    char buf[64 << 10]
		    readLen = 0
		    if !::ReadFile(stdoutRead, buf, sizeof(buf), &readLen, nil) && GetLastError() != ERROR_BROKEN_PIPE {
		      Win32Fatal("ReadFile")
		    }
		    output.append(buf, readLen)
		  }

		  // Wait for it to exit and grab its exit code.
		  if WaitForSingleObject(processInfo.hProcess, INFINITE) == WAIT_FAILED {
		    Win32Fatal("WaitForSingleObject")
		  }
		  exitCode := 0
		  if !GetExitCodeProcess(processInfo.hProcess, &exitCode) {
		    Win32Fatal("GetExitCodeProcess")
		  }

		  if !CloseHandle(stdoutRead) || !CloseHandle(processInfo.hProcess) || !CloseHandle(processInfo.hThread) {
		    Win32Fatal("CloseHandle")
		  }

		  return exitCode
	*/
}
