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
	"context"
	"os/exec"
	"syscall"
)

func createCmd(ctx context.Context, c string, useConsole, enableSkipShell bool) *exec.Cmd {
	// The commands being run use shell redirection. The C++ version uses
	// system() which always uses the default shell.
	//
	// Determine if we use the experimental shell skipping fast track mode,
	// saving an unnecessary exec(). Only use this when we detect no quote, no
	// shell redirection character.

	// TODO(maruel): skipShell := enableSkipShell && !strings.ContainsAny(c, "$><&|")

	ex := "/bin/sh"
	args := []string{"-c", c}
	var cmd *exec.Cmd
	if useConsole {
		cmd = exec.Command(ex, args...)
	} else {
		cmd = exec.CommandContext(ctx, ex, args...)
	}

	// When useConsole is false, it is a new process group on posix.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: !useConsole,
	}
	return cmd
}
