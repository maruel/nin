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
	"context"
	"os/exec"
	"strings"
	"syscall"
)

func createCmd(ctx context.Context, c string, useConsole, enableSkipShell bool) *exec.Cmd {
	// The commands being run use shell redirection. The C++ version uses
	// system() which always uses the default shell.
	//
	// Determine if we use the experimental shell skipping fast track mode,
	// saving an unnecessary exec(). Only use this when we detect no quote, no
	// shell redirection character.
	// TODO(maruel): This is incorrect and temporary.
	skipShell := true || (enableSkipShell && !strings.ContainsAny(c, "%><&|^"))

	ex := ""
	var args []string
	if skipShell {
		// Ignore the parsed arguments on Windows and feedback the original string.
		//
		// TODO(maruel): Handle quoted space. It's only necessary from the
		// perspective of finding the primary executable to run.
		i := strings.IndexByte(c, ' ')
		if i == -1 {
			// A single executable with no argument.
			ex = c
		} else {
			ex = c[:i]
		}
		args = []string{c}
	} else {
		ex = "cmd.exe"
		args = []string{"/c", c}
	}
	var cmd *exec.Cmd
	if useConsole {
		cmd = exec.Command(ex, args...)
	} else {
		cmd = exec.CommandContext(ctx, ex, args...)
	}

	// Ignore the parsed arguments on Windows and feed back the original string.
	// See https://pkg.go.dev/os/exec#Command for an explanation.
	if skipShell {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			CmdLine: c,
		}
		cmd.Args = nil
	}
	if useConsole {
		cmd.SysProcAttr.CreationFlags = syscall.CREATE_NEW_PROCESS_GROUP
	}

	// TODO(maruel): CTRL_C_EVENT and CTRL_BREAK_EVENT handling with
	// GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT) when canceling plus
	// PostQueuedCompletionStatus(CreateIoCompletionPort()) via SetConsoleCtrlHandler(fn, FALSE).
	return cmd
}
