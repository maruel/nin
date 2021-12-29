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

package ginja

import "fmt"

func MSVCHelperUsage() {
	fmt.Printf("usage: ninja -t msvc [options] -- cl.exe /showIncludes /otherArgs\noptions:\n  -e ENVFILE load environment block from ENVFILE as environment\n  -o FILE    write output dependency information to FILE.d\n  -p STRING  localized prefix of msvc's /showIncludes output\n")
}

func PushPathIntoEnvironment(env_block string) {
	panic("TODO")
	/*
		as_str := env_block
		for as_str[0] {
			if _strnicmp(as_str, "path=", 5) == 0 {
				_putenv(as_str)
				return
			} else {
				as_str = &as_str[strlen(as_str)+1]
			}
		}
	*/
}

func WriteDepFileOrDie(object_path string, parse *CLParser) {
	panic("TODO")
	/*
		depfile_path := object_path + ".d"
		depfile, err := os.OpenFile(depfile_path, os.O_WRONLY, 0o666)
		if depfile == nil {
			os.Remove(object_path)
			Fatal("opening %s: %s", depfile_path, err)
		}
		if _, err := fmt.Fprintf(depfile, "%s: ", object_path); err != nil {
			os.Remove(object_path)
			depfile.Close()
			os.Remove(depfile_path)
			Fatal("writing %s", depfile_path)
		}
		headers := parse.includes_
		for i := range headers {
			if _, err := fmt.Fprintf(depfile, "%s\n", EscapeForDepfile(i)); err != nil {
				os.Remove(object_path)
				depfile.Close()
				os.Remove(depfile_path)
				Fatal("writing %s", depfile_path)
			}
		}
		depfile.Close()
	*/
}

func MSVCHelperMain(arg []string) int {
	panic("TODO")
	/*
		output_filename := nil
		envfile := nil

		//kLongOptions := {{ "help", no_argument, nil, 'h' }, { nil, 0, nil, 0 }}
		deps_prefix := ""
		for opt := getopt_long(argc, argv, "e:o:p:h", kLongOptions, nil); opt != -1; {
			switch opt {
			case 'e':
				envfile = optarg
				break
			case 'o':
				output_filename = optarg
				break
			case 'p':
				deps_prefix = optarg
				break
			case 'h':
			default:
				MSVCHelperUsage()
				return 0
			}
		}

		var env []byte
		if envfile != nil {
			env, err2 := ReadFile(envfile)
			if err2 != nil {
				Fatal("couldn't open %s: %s", envfile, err2)
			}
			PushPathIntoEnvironment(env)
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
		exit_code := cl.Run(command, &output)

		if output_filename {
			parser := NewCLParser()
			err := ""
			if !parser.Parse(output, deps_prefix, &output, &err) {
				Fatal("%s\n", err)
			}
			WriteDepFileOrDie(output_filename, parser)
		}

		if len(output) == 0 {
			return exit_code
		}

		// CLWrapper's output already as \r\n line endings, make sure the C runtime
		// doesn't expand this to \r\r\n.
		_setmode(_fileno(stdout), _O_BINARY)
		// Avoid printf and C strings, since the actual output might contain null
		// bytes like UTF-16 does (yuck).
		os.Stdout.Write(output)

		return exit_code
	*/
}
