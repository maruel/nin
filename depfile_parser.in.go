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

//go:build neverbuild
// +build neverbuild

package nin

import "errors"

// DepfileParser is the parser for the dependency information emitted by gcc's
// -M flags.
type DepfileParser struct {
	outs []string
	ins  []string
}

// Parse parses a dependency file.
//
// content must contain a terminating zero byte.
//
// Warning: mutate the slice content in-place.
//
// A note on backslashes in Makefiles, from reading the docs:
// Backslash-newline is the line continuation character.
// Backslash-# escapes a # (otherwise meaningful as a comment start).
// Backslash-% escapes a % (otherwise meaningful as a special).
// Finally, quoting the GNU manual, "Backslashes that are not in danger
// of quoting ‘%’ characters go unmolested."
// How do you end a line with a backslash?  The netbsd Make docs suggest
// reading the result of a shell command echoing a backslash!
//
// Rather than implement all of above, we follow what GCC/Clang produces:
// Backslashes escape a space or hash sign.
// When a space is preceded by 2N+1 backslashes, it is represents N backslashes
// followed by space.
// When a space is preceded by 2N backslashes, it represents 2N backslashes at
// the end of a filename.
// A hash sign is escaped by a single backslash. All other backslashes remain
// unchanged.
//
// If anyone actually has depfiles that rely on the more complicated
// behavior we can adjust this.
func (d *DepfileParser) Parse(content []byte) error {
	// in: current parser input point.
	// end: end of input.
	// parsingTargets: whether we are parsing targets or dependencies.
	in := 0
	end := len(content)
	if end > 0 && content[len(content)-1] != 0 {
		panic("internal error")
	}
	haveTarget := false
	parsingTargets := true
	poisonedInput := false
	for in < end {
		haveNewline := false
		// out: current output point (typically same as in, but can fall behind
		// as we de-escape backslashes).
		out := in
		// filename: start of the current parsed filename.
		filename := out
		backup := 0
		for {
			// start: beginning of the current parsed span.
			start := in
			yymarker := 0
			/*
			 re2c:define:YYCTYPE = "byte";
			 re2c:define:YYCURSOR = "l.input[p]";
			 re2c:define:YYMARKER = q;
			 re2c:yyfill:enable = 0;
			 re2c:flags:nested-ifs = 1;
			*/

			/*!re2c
			re2c:define:YYCTYPE = "byte";
			re2c:define:YYCURSOR = content[in];
			re2c:define:YYLIMIT = end;
			re2c:define:YYMARKER = yymarker;
			re2c:define:YYBACKUP = "backup = yymarker";
			re2c:define:YYRESTORE = "yymarker = backup";
			re2c:define:YYPEEK = "content[in]";
			re2c:define:YYSKIP = "in++";

			re2c:yyfill:enable = 0;

			re2c:indent:top = 2;
			re2c:indent:string = "  ";

			nul = "\000";
			newline = '\r'?'\n';

			'\\\\'* '\\ ' {
				// 2N+1 backslashes plus space -> N backslashes plus space.
				l := in - start
				n := l / 2 - 1
				if out < start {
					for i := 0; i < n; i++ {
						content[out+i] = '\\'
					}
				}
				out += n
				content[out] = ' '
				out++
				continue
			}
			'\\\\'+ ' ' {
				// 2N backslashes plus space -> 2N backslashes, end of filename.
				l := in - start
				if out < start {
					for i := 0; i < l-1; i++ {
						content[out+i] = '\\'
					}
				}
				out += l - 1
				break
			}
			'\\'+ '#' {
				// De-escape hash sign, but preserve other leading backslashes.
				l := in - start
				if l > 2 && out < start {
					for i := 0; i < l-2; i++ {
						content[out+i] = '\\'
					}
				}
				out += l - 2
				content[out] = '#'
				out++
				continue
			}
			'\\'+ ':' [\x00\x20\r\n\t] {
				// Backslash followed by : and whitespace.
				// It is therefore normal text and not an escaped colon
				l := in - start - 1
				// Need to shift it over if we're overwriting backslashes.
				if out < start {
					copy(content[out:out+l], content[start:start+l])
				}
				out += l
				if content[in - 1] == '\n' {
					haveNewline = true
				}
				break
			}
			'\\'+ ':' {
				// De-escape colon sign, but preserve other leading backslashes.
				// Regular expression uses lookahead to make sure that no whitespace
				// nor EOF follows. In that case it'd be the : at the end of a target
				l := in - start
				if l > 2 && out < start {
					for i := 0; i < l-2; i++ {
						content[out+i] = '\\'
					}
				}
				out += l - 2
				content[out] = ':'
				out++
				continue
			}
			'$$' {
				// De-escape dollar character.
				content[out] = '$'
				out++
				continue
			}
			'\\'+ [^\000\r\n] | [a-zA-Z0-9+,/_:.~()}{%=@\x5B\x5D!\x80-\xFF-]+ {
				// Got a span of plain text.
				l := in - start
				// Need to shift it over if we're overwriting backslashes.
				if out < start {
					copy(content[out:out+l], content[start:start+l])
				}
				out += l
				continue
			}
			nul {
				break
			}
			'\\' newline {
				// A line continuation ends the current file name.
				break
			}
			newline {
				// A newline ends the current file name and the current rule.
				haveNewline = true
				break
			}
			[^] {
				// For any other character (e.g. whitespace), swallow it here,
				// allowing the outer logic to loop around again.
				break
			}
			*/
		}

		l := out - filename
		isDependency := !parsingTargets
		if l > 0 && content[filename+l-1] == ':' {
			l-- // Strip off trailing colon, if any.
			parsingTargets = false
			haveTarget = true
		}

		if l > 0 {
			piece := unsafeString(content[filename : filename+l])
			// If we've seen this as an input before, skip it.
			// TODO(maruel): Use a map[string]struct{} while constructing.
			pos := -1
			for i, v := range d.ins {
				if piece == v {
					pos = i
					break
				}
			}
			if pos == -1 {
				if isDependency {
					if poisonedInput {
						return errors.New("inputs may not also have inputs")
					}
					// New input.
					d.ins = append(d.ins, piece)
				} else {
					// Check for a new output.
					pos = -1
					for i, v := range d.outs {
						if piece == v {
							pos = i
							break
						}
					}
					if pos == -1 {
						d.outs = append(d.outs, piece)
					}
				}
			} else if !isDependency {
				// We've passed an input on the left side; reject new inputs.
				poisonedInput = true
			}
		}

		if haveNewline {
			// A newline ends a rule so the next filename will be a new target.
			parsingTargets = true
			poisonedInput = false
		}
	}
	if !haveTarget {
		return errors.New("expected ':' in depfile")
	}
	return nil
}
