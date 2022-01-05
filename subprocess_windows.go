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
	"os/exec"
	"syscall"
)

func (s *subprocess) osSpecific(cmd *exec.Cmd, c string, useConsole bool) {
	// Ignore the parsed arguments on Windows and feed back the original string.
	// See https://pkg.go.dev/os/exec#Command for an explanation.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: c,
	}
	if useConsole {
		cmd.SysProcAttr.CreationFlags = syscall.CREATE_NEW_PROCESS_GROUP
	}
	cmd.Args = nil

	// TODO(maruel): CTRL_C_EVENT and CTRL_BREAK_EVENT handling with
	// GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT) when canceling plus
	// PostQueuedCompletionStatus(CreateIoCompletionPort()) via SetConsoleCtrlHandler(fn, FALSE).
}
