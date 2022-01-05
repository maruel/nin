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

import (
	"fmt"
	"strings"
)

type Token int32

const (
	ERROR Token = iota
	BUILD
	COLON
	DEFAULT
	EQUALS
	IDENT
	INCLUDE
	INDENT
	NEWLINE
	PIPE
	PIPE2
	PIPEAT
	POOL
	RULE
	SUBNINJA
	TEOF
)

// String() returns a human-readable form of a token, used in error messages.
func (t Token) String() string {
	switch t {
	case ERROR:
		return "lexing error"
	case BUILD:
		return "'build'"
	case COLON:
		return "':'"
	case DEFAULT:
		return "'default'"
	case EQUALS:
		return "'='"
	case IDENT:
		return "identifier"
	case INCLUDE:
		return "'include'"
	case INDENT:
		return "indent"
	case NEWLINE:
		return "newline"
	case PIPE2:
		return "'||'"
	case PIPE:
		return "'|'"
	case PIPEAT:
		return "'|@'"
	case POOL:
		return "'pool'"
	case RULE:
		return "'rule'"
	case SUBNINJA:
		return "'subninja'"
	case TEOF:
		return "eof"
	}
	return "" // not reached
}

// errorHint returns a human-readable token hint, used in error messages.
func (t Token) errorHint() string {
	if t == COLON {
		return " ($ also escapes ':')"
	}
	return ""
}

// lexerState is the offset of processing a token.
//
// It is meant to be saved when an error message may be printed after the
// parsing continued.
type lexerState struct {
	// In the original C++ code, these two are char pointers and are used to do
	// pointer arithmetics. Go doesn't allow pointer arithmetics so they are
	// indexes. ofs starts at 0. lastToken is initially -1 to mark that it is
	// not yet set.
	ofs       int
	lastToken int
}

// error constructs an error message with context.
func (l *lexerState) error(message, filename string, input []byte) error {
	// Compute line/column.
	line := 1
	lineStart := 0
	for p := 0; p < l.lastToken; p++ {
		if input[p] == '\n' {
			line++
			lineStart = p + 1
		}
	}
	col := 0
	if l.lastToken != -1 {
		col = l.lastToken - lineStart
	}

	// Add some context to the message.
	c := ""
	const truncateColumn = 72
	if col > 0 && col < truncateColumn {
		truncated := true
		length := 0
		for ; length < truncateColumn; length++ {
			if input[lineStart+length] == 0 || input[lineStart+length] == '\n' {
				truncated = false
				break
			}
		}
		c = unsafeString(input[lineStart : lineStart+length])
		if truncated {
			c += "..."
		}
		c += "\n"
		c += strings.Repeat(" ", col)
		c += "^ near here"
	}
	// TODO(maruel): There's a problem where the error is wrapped, thus the alignment doesn't work.
	return fmt.Errorf("%s:%d: %s\n%s", filename, line, message, c)
}

type lexer struct {
	// Immutable.
	filename string
	input    []byte

	// Mutable.
	lexerState
}

// Error constructs an error message with context.
func (l *lexer) Error(message string) error {
	return l.lexerState.error(message, l.filename, l.input)
}

// Start parsing some input.
func (l *lexer) Start(filename string, input []byte) {
	l.filename = filename
	if input[len(input)-1] != 0 {
		panic("Requires hack with a trailing 0 byte")
	}
	l.input = input
	l.ofs = 0
	l.lastToken = -1
}

// If the last token read was an ERROR token, provide more info
// or the empty string.
func (l *lexer) DescribeLastError() string {
	if l.lastToken != -1 {
		switch l.input[l.lastToken] {
		case '\t':
			return "tabs are not allowed, use spaces"
		}
	}
	return "lexing error"
}

// Rewind to the last read Token.
func (l *lexer) UnreadToken() {
	l.ofs = l.lastToken
}

func (l *lexer) ReadToken() Token {
	p := l.ofs
	q := 0
	start := 0
	var token Token
	for {
		start = p
		/*!re2c
		    re2c:define:YYCTYPE = "byte";
		    re2c:define:YYCURSOR = "l.input[p]";
				re2c:define:YYSKIP = "p++";
		    re2c:define:YYMARKER = q;
		    re2c:yyfill:enable = 0;
				re2c:flags:nested-ifs = 0;
		    re2c:define:YYPEEK = "l.input[p]";
				re2c:define:YYBACKUP = "q = p";
				re2c:define:YYRESTORE = "p = q";

		    nul = "\000";
		    simpleVarname = [a-zA-Z0-9_-]+;
		    varname = [a-zA-Z0-9_.-]+;

		    [ ]*"#"[^\000\n]*"\n" { continue; }
		    [ ]*"\r\n" { token = NEWLINE;  break; }
		    [ ]*"\n"   { token = NEWLINE;  break; }
		    [ ]+       { token = INDENT;   break; }
		    "build"    { token = BUILD;    break; }
		    "pool"     { token = POOL;     break; }
		    "rule"     { token = RULE;     break; }
		    "default"  { token = DEFAULT;  break; }
		    "="        { token = EQUALS;   break; }
		    ":"        { token = COLON;    break; }
				"|@"       { token = PIPEAT;   break; }
		    "||"       { token = PIPE2;    break; }
		    "|"        { token = PIPE;     break; }
		    "include"  { token = INCLUDE;  break; }
		    "subninja" { token = SUBNINJA; break; }
		    varname    { token = IDENT;    break; }
		    nul        { token = TEOF;     break; }
		    [^]        { token = ERROR;    break; }
		*/
	}

	l.lastToken = start
	l.ofs = p
	if token != NEWLINE && token != TEOF {
		l.eatWhitespace()
	}
	return token
}

// If the next token is \a token, read it and return true.
func (l *lexer) PeekToken(token Token) bool {
	t := l.ReadToken()
	if t == token {
		return true
	}
	l.UnreadToken()
	return false
}

// Skip past whitespace (called after each read token/ident/etc.).
func (l *lexer) eatWhitespace() {
	p := l.ofs
	q := 0
	for {
		l.ofs = p
		/*!re2c
		  [ ]+    { continue; }
		  "$\r\n" { continue; }
		  "$\n"   { continue; }
		  nul     { break; }
		  [^]     { break; }
		*/
	}
}

// Read a simple identifier (a rule or variable name).
// Returns false if a name can't be read.
func (l *lexer) readIdent() string {
	out := ""
	p := l.ofs
	start := 0
	for {
		start = p
		/*!re2c
		  varname {
				out = unsafeString(l.input[start:p])
		    break
		  }
		  [^] {
		    l.lastToken = start
		    return ""
		  }
		*/
	}
	l.lastToken = start
	l.ofs = p
	l.eatWhitespace()
	return out
}

// readEvalString reads a $-escaped string.
//
// If path is true, read a path (complete with $escapes).
//
// If path is false, read the value side of a var = value line (complete with
// $escapes).
//
// Returned path may be empty if a delimiter (space, newline) is hit.
func (l *lexer) readEvalString(path bool) (EvalString, error) {
	// Do two passes, first to count the number of tokens, then to act on it. It
	// is because some strings may contain a fairly large number of tokens,
	// causing a fair amount of runtime.growslice() calls.
	p := l.ofs
	q := 0
	start := 0
	tokens := 0
	for {
		start = p
		/*!re2c
		  [^$ :\r\n|\000]+ {
				tokens++
		    continue
		  }
		  "\r\n" {
		    if path {
		      p = start
		    }
		    break
		  }
		  [ :|\n] {
		    if path {
		      p = start
		      break
		    } else {
		      if l.input[start] == '\n' {
		        break
		      }
				tokens++
		      continue
		    }
		  }
		  "$$" {
				tokens++
		    continue
		  }
		  "$ " {
				tokens++
		    continue
		  }
		  "$\r\n"[ ]* {
		    continue
		  }
		  "$\n"[ ]* {
		    continue
		  }
		  "${"varname"}" {
				tokens++
		    continue
		  }
		  "$"simpleVarname {
				tokens++
		    continue
		  }
		  "$:" {
				tokens++
		    continue
		  }
		  "$". {
		    l.lastToken = start
		    return EvalString{}, l.Error("bad $-escape (literal $ must be written as $$)")
		  }
		  nul {
		    l.lastToken = start
		    return EvalString{}, l.Error("unexpected EOF")
		  }
		  [^] {
		    l.lastToken = start
		    return EvalString{}, l.Error(l.DescribeLastError())
		  }
		*/
	}

	// One side effect is that the string has been validated, so the second loop
	// can skip on error checking.

	eval := EvalString{Parsed: make([]TokenListItem, 0, tokens)}
	p = l.ofs
	q = 0
	start = 0
	for {
		start = p
		/*!re2c
		  [^$ :\r\n|\000]+ {
				eval.Parsed = append(eval.Parsed, EvalStringToken{unsafeString(l.input[start: p]), false})
		    continue
		  }
		  "\r\n" {
		    if path {
		      p = start
		    }
		    break
		  }
		  [ :|\n] {
		    if path {
		      p = start
		      break
		    } else {
		      if l.input[start] == '\n' {
		        break
		      }
					eval.Parsed = append(eval.Parsed, EvalStringToken{unsafeString(l.input[start:start+1]), false})
		      continue
		    }
		  }
		  "$$" {
				eval.Parsed = append(eval.Parsed, EvalStringToken{"$", false})
		    continue
		  }
		  "$ " {
				eval.Parsed = append(eval.Parsed, EvalStringToken{" ", false})
		    continue
		  }
		  "$\r\n"[ ]* {
		    continue
		  }
		  "$\n"[ ]* {
		    continue
		  }
		  "${"varname"}" {
				eval.Parsed = append(eval.Parsed, EvalStringToken{unsafeString(l.input[start + 2: p - 1]), true})
		    continue
		  }
		  "$"simpleVarname {
				eval.Parsed = append(eval.Parsed, EvalStringToken{unsafeString(l.input[start + 1: p]), true})
		    continue
		  }
		  "$:" {
				eval.Parsed = append(eval.Parsed, EvalStringToken{":", false})
		    continue
		  }
		*/
	}
	l.lastToken = start
	l.ofs = p
	if path {
		l.eatWhitespace()
	}
	// Non-path strings end in newlines, so there's no whitespace to eat.
	return eval, nil
}
