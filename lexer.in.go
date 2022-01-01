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

type Lexer struct {
	filename_ string
	input_    string
	// In the original C++ code, these two are char pointers and are used to do
	// pointer arithmetics. Go doesn't allow pointer arithmetics so they are
	// indexes. ofs_ starts at 0. last_token_ is initially -1 to mark that it is
	// not yet set.
	ofs_        int
	last_token_ int
}

// Read a path (complete with $escapes).
// Returns false only on error, returned path may be empty if a delimiter
// (space, newline) is hit.
func (l *Lexer) ReadPath(path *EvalString, err *string) bool {
	return l.ReadEvalString(path, true, err)
}

// Read the value side of a var = value line (complete with $escapes).
// Returns false only on error.
func (l *Lexer) ReadVarValue(value *EvalString, err *string) bool {
	return l.ReadEvalString(value, false, err)
}

// Construct an error message with context.
func (l *Lexer) Error(message string, err *string) bool {
	// Compute line/column.
	line := 1
	line_start := 0
	for p := 0; p < l.last_token_; p++ {
		if l.input_[p] == '\n' {
			line++
			line_start = p + 1
		}
	}
	col := 0
	if l.last_token_ != -1 {
		col = l.last_token_ - line_start
	}

	*err = fmt.Sprintf("%s:%d: ", l.filename_, line)
	*err += message + "\n"
	// Add some context to the message.
	const kTruncateColumn = 72
	if col > 0 && col < kTruncateColumn {
		truncated := true
		length := 0
		for ; length < kTruncateColumn; length++ {
			if l.input_[line_start+length] == 0 || l.input_[line_start+length] == '\n' {
				truncated = false
				break
			}
		}
		*err += l.input_[line_start : line_start+length]
		if truncated {
			*err += "..."
		}
		*err += "\n"
		*err += strings.Repeat(" ", col)
		*err += "^ near here"
	}
	return false
}

// NewLexer is only used in tests.
func NewLexer(input string) Lexer {
	l := Lexer{}
	l.Start("input", input+"\x00")
	return l
}

// Start parsing some input.
func (l *Lexer) Start(filename, input string) {
	l.filename_ = filename
	if !strings.HasSuffix(input, "\x00") {
		panic("Requires hack with a trailing 0 byte")
	}
	l.input_ = input
	l.ofs_ = 0
	l.last_token_ = -1
}

// Return a human-readable form of a token, used in error messages.
func TokenName(t Token) string {
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

// Return a human-readable token hint, used in error messages.
func TokenErrorHint(expected Token) string {
	switch expected {
	case COLON:
		return " ($ also escapes ':')"
	default:
		return ""
	}
}

// If the last token read was an ERROR token, provide more info
// or the empty string.
func (l *Lexer) DescribeLastError() string {
	if l.last_token_ != -1 {
		switch l.input_[l.last_token_] {
		case '\t':
			return "tabs are not allowed, use spaces"
		}
	}
	return "lexing error"
}

// Rewind to the last read Token.
func (l *Lexer) UnreadToken() {
	l.ofs_ = l.last_token_
}

func (l *Lexer) ReadToken() Token {
	p := l.ofs_
	q := 0
	start := 0
	var token Token
	for {
		start = p
		/*!re2c
		    re2c:define:YYCTYPE = "byte";
		    re2c:define:YYCURSOR = "l.input_[p]";
				re2c:define:YYSKIP = "p++";
		    re2c:define:YYMARKER = q;
		    re2c:yyfill:enable = 0;
				re2c:flags:nested-ifs = 1;
		    re2c:define:YYPEEK = "l.input_[p]";
				re2c:define:YYBACKUP = "q = p";
				re2c:define:YYRESTORE = "p = q";

		    nul = "\000";
		    simple_varname = [a-zA-Z0-9_-]+;
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

	l.last_token_ = start
	l.ofs_ = p
	if token != NEWLINE && token != TEOF {
		l.EatWhitespace()
	}
	return token
}

// If the next token is \a token, read it and return true.
func (l *Lexer) PeekToken(token Token) bool {
	t := l.ReadToken()
	if t == token {
		return true
	}
	l.UnreadToken()
	return false
}

// Skip past whitespace (called after each read token/ident/etc.).
func (l *Lexer) EatWhitespace() {
	p := l.ofs_
	q := 0
	for {
		l.ofs_ = p
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
func (l *Lexer) ReadIdent(out *string) bool {
	p := l.ofs_
	start := 0
	for {
		start = p
		/*!re2c
		  varname {
				*out = l.input_[start:p]
		    break
		  }
		  [^] {
		    l.last_token_ = start
		    return false
		  }
		*/
	}
	l.last_token_ = start
	l.ofs_ = p
	l.EatWhitespace()
	return true
}

// Read a $-escaped string.
func (l *Lexer) ReadEvalString(eval *EvalString, path bool, err *string) bool {
	p := l.ofs_
	q := 0
	start := 0
	for {
		start = p
		/*!re2c
		  [^$ :\r\n|\000]+ {
				eval.AddText(l.input_[start: p])
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
		      if l.input_[start] == '\n' {
		        break
		      }
					eval.AddText(l.input_[start:start+1])
		      continue
		    }
		  }
		  "$$" {
		    eval.AddText("$")
		    continue
		  }
		  "$ " {
		    eval.AddText(" ")
		    continue
		  }
		  "$\r\n"[ ]* {
		    continue
		  }
		  "$\n"[ ]* {
		    continue
		  }
		  "${"varname"}" {
				eval.AddSpecial(l.input_[start + 2: p - 1])
		    continue
		  }
		  "$"simple_varname {
				eval.AddSpecial(l.input_[start + 1: p])
		    continue
		  }
		  "$:" {
		    eval.AddText(":")
		    continue
		  }
		  "$". {
		    l.last_token_ = start
		    return l.Error("bad $-escape (literal $ must be written as $$)", err)
		  }
		  nul {
		    l.last_token_ = start
		    return l.Error("unexpected EOF", err)
		  }
		  [^] {
		    l.last_token_ = start
		    return l.Error(l.DescribeLastError(), err)
		  }
		*/
	}
	l.last_token_ = start
	l.ofs_ = p
	if path {
		l.EatWhitespace()
	}
	// Non-path strings end in newlines, so there's no whitespace to eat.
	return true
}
