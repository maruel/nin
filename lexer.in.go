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

//go:build nobuild

package ginga


func (l *Lexer) Error(message string, err *string) bool {
  // Compute line/column.
  line := 1
  line_start := input_.str_
  for (string p = input_.str_; p < last_token_; ++p) {
    if *p == '\n' {
      ++line
      line_start = p + 1
    }
  }
  col := last_token_ ? (int)(last_token_ - line_start) : 0

  char buf[1024]
  snprintf(buf, sizeof(buf), "%s:%d: ", filename_.AsString(), line)
  *err = buf
  *err += message + "\n"

  // Add some context to the message.
  const int kTruncateColumn = 72
  if col > 0 && col < kTruncateColumn {
    int len
    truncated := true
    for (len = 0; len < kTruncateColumn; ++len) {
      if line_start[len] == 0 || line_start[len] == '\n' {
        truncated = false
        break
      }
    }
    *err += string(line_start, len)
    if truncated != nil {
      *err += "..."
    }
    *err += "\n"
    *err += string(col, ' ')
    *err += "^ near here"
  }

  return false
}

Lexer::Lexer(string input) {
  Start("input", input)
}

func (l *Lexer) Start(filename StringPiece, input StringPiece) {
  filename_ = filename
  input_ = input
  ofs_ = input_.str_
  last_token_ = nil
}

func (l *Lexer) TokenName(t Token) string {
  switch (t) {
  case ERROR:    return "lexing error"
  case BUILD:    return "'build'"
  case COLON:    return "':'"
  case DEFAULT:  return "'default'"
  case EQUALS:   return "'='"
  case IDENT:    return "identifier"
  case INCLUDE:  return "'include'"
  case INDENT:   return "indent"
  case NEWLINE:  return "newline"
  case PIPE2:    return "'||'"
  case PIPE:     return "'|'"
  case POOL:     return "'pool'"
  case RULE:     return "'rule'"
  case SUBNINJA: return "'subninja'"
  case TEOF:     return "eof"
  }
  return nil  // not reached
}

func (l *Lexer) TokenErrorHint(expected Token) string {
  switch (expected) {
  case COLON:
    return " ($ also escapes ':')"
  default:
    return ""
  }
}

func (l *Lexer) DescribeLastError() string {
  if last_token_ {
    switch (last_token_[0]) {
    case '\t':
      return "tabs are not allowed, use spaces"
    }
  }
  return "lexing error"
}

func (l *Lexer) UnreadToken() {
  ofs_ = last_token_
}

Lexer::Token Lexer::ReadToken() {
  p := ofs_
  string q
  string start
  Lexer::Token token
  for (;;) {
    start = p
    /*!re2c
    re2c:define:YYCTYPE = "unsigned char"
    re2c:define:YYCURSOR = p
    re2c:define:YYMARKER = q
    re2c:yyfill:enable = 0

    nul = "\000"
    simple_varname = [a-zA-Z0-9_-]+
    varname = [a-zA-Z0-9_.-]+

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
    "||"       { token = PIPE2;    break; }
    "|"        { token = PIPE;     break; }
    "include"  { token = INCLUDE;  break; }
    "subninja" { token = SUBNINJA; break; }
    varname    { token = IDENT;    break; }
    nul        { token = TEOF;     break; }
    [^]        { token = ERROR;    break; }
    */
  }

  last_token_ = start
  ofs_ = p
  if token != NEWLINE && token != TEOF {
    EatWhitespace()
  }
  return token
}

func (l *Lexer) PeekToken(token Token) bool {
  t := ReadToken()
  if t == token {
    return true
  }
  UnreadToken()
  return false
}

func (l *Lexer) EatWhitespace() {
  p := ofs_
  string q
  for (;;) {
    ofs_ = p
    /*!re2c
    [ ]+    { continue; }
    "$\r\n" { continue; }
    "$\n"   { continue; }
    nul     { break; }
    [^]     { break; }
    */
  }
}

func (l *Lexer) ReadIdent(out *string) bool {
  p := ofs_
  string start
  for (;;) {
    start = p
    /*!re2c
    varname {
      out.assign(start, p - start)
      break
    }
    [^] {
      last_token_ = start
      return false
    }
    */
  }
  last_token_ = start
  ofs_ = p
  EatWhitespace()
  return true
}

func (l *Lexer) ReadEvalString(eval *EvalString, path bool, err *string) bool {
  p := ofs_
  string q
  string start
  for (;;) {
    start = p
    /*!re2c
    [^$ :\r\n|\000]+ {
      eval.AddText(StringPiece(start, p - start))
      continue
    }
    "\r\n" {
      if path != nil {
        p = start
      }
      break
    }
    [ :|\n] {
      if path != nil {
        p = start
        break
      } else {
        if *start == '\n' {
          break
        }
        eval.AddText(StringPiece(start, 1))
        continue
      }
    }
    "$$" {
      eval.AddText(StringPiece("$", 1))
      continue
    }
    "$ " {
      eval.AddText(StringPiece(" ", 1))
      continue
    }
    "$\r\n"[ ]* {
      continue
    }
    "$\n"[ ]* {
      continue
    }
    "${"varname"}" {
      eval.AddSpecial(StringPiece(start + 2, p - start - 3))
      continue
    }
    "$"simple_varname {
      eval.AddSpecial(StringPiece(start + 1, p - start - 1))
      continue
    }
    "$:" {
      eval.AddText(StringPiece(":", 1))
      continue
    }
    "$". {
      last_token_ = start
      return Error("bad $-escape (literal $ must be written as $$)", err)
    }
    nul {
      last_token_ = start
      return Error("unexpected EOF", err)
    }
    [^] {
      last_token_ = start
      return Error(DescribeLastError(), err)
    }
    */
  }
  last_token_ = start
  ofs_ = p
  if path != nil {
    EatWhitespace()
  }
  // Non-path strings end in newlines, so there's no whitespace to eat.
  return true
}

