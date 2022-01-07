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

import (
	"fmt"

	"github.com/maruel/nin"
)

func msvcHelperUsage() {
	fmt.Printf("usage: ninja -t msvc [options] -- cl.exe /showIncludes /otherArgs\noptions:\n  -e ENVFILE load environment block from ENVFILE as environment\n  -o FILE    write output dependency information to FILE.d\n  -p STRING  localized prefix of msvc's /showIncludes output\n")
}

func pushPathIntoEnvironment(envBlock string) {
	panic("TODO")
	/*
		asStr := envBlock
		for asStr[0] {
			if Strnicmp(asStr, "path=", 5) == 0 {
				Putenv(asStr)
				return
			} else {
				asStr = &asStr[strlen(asStr)+1]
			}
		}
	*/
}

func writeDepFileOrDie(objectPath string, parse *nin.CLParser) {
	panic("TODO")
	/*
		depfilePath := objectPath + ".d"
		depfile, err := os.OpenFile(depfilePath, os.O_WRONLY, 0o666)
		if depfile == nil {
			os.Remove(objectPath)
			Fatal("opening %s: %s", depfilePath, err)
		}
		if _, err := fmt.Fprintf(depfile, "%s: ", objectPath); err != nil {
			os.Remove(objectPath)
			depfile.Close()
			os.Remove(depfilePath)
			Fatal("writing %s", depfilePath)
		}
		headers := parse.includes
		for i := range headers {
			if _, err := fmt.Fprintf(depfile, "%s\n", EscapeForDepfile(i)); err != nil {
				os.Remove(objectPath)
				depfile.Close()
				os.Remove(depfilePath)
				Fatal("writing %s", depfilePath)
			}
		}
		depfile.Close()
	*/
}

func msvcHelperMain(arg []string) int {
	panic("TODO")
	/*
		outputFilename := nil
		envfile := nil

		longOptions := {{ "help", noArgument, nil, 'h' }, { nil, 0, nil, 0 }}
		depsPrefix := ""
		for opt := getoptLong(argc, argv, "e:o:p:h", longOptions, nil); opt != -1; {
			switch opt {
			case 'e':
				envfile = optarg
				break
			case 'o':
				outputFilename = optarg
				break
			case 'p':
				depsPrefix = optarg
				break
			case 'h':
			default:
				msvcHelperUsage()
				return 0
			}
		}

		var env []byte
		if envfile != nil {
			env, err2 := ReadFile(envfile)
			if err2 != nil {
				Fatal("couldn't open %s: %s", envfile, err2)
			}
			pushPathIntoEnvironment(env)
		}

		command := GetCommandLineA()
		command = strstr(command, " -- ")
		if command == nil {
			Fatal("expected command line to end with \" -- command args\"")
		}
		command += 4

		cl := NewCLWrapper()
		if len(env) != 0 {
			cl.SetEnvBlock(env)
		}
		output := ""
		exitCode := cl.Run(command, &output)

		if outputFilename {
			parser := nin.NewCLParser()
			err := ""
			if !parser.Parse(output, depsPrefix, &output, &err) {
				Fatal("%s\n", err)
			}
			writeDepFileOrDie(outputFilename, parser)
		}

		if len(output) == 0 {
			return exitCode
		}

		// CLWrapper's output already as \r\n line endings, make sure the C runtime
		// doesn't expand this to \r\r\n.
		Setmode(Fileno(stdout), _O_BINARY)
		// Avoid printf and C strings, since the actual output might contain null
		// bytes like UTF-16 does (yuck).
		os.Stdout.Write(output)

		return exitCode
	*/
}
