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

package nin

import (
	"fmt"
	"os"
	"runtime"
	"unsafe"
)

// Have a generic fall-through for different versions of C/C++.

// Log a fatalf message and exit.
func fatalf(msg string, s ...interface{}) {
	fmt.Fprintf(os.Stderr, "nin: fatal: ")
	fmt.Fprintf(os.Stderr, msg, s...)
	fmt.Fprintf(os.Stderr, "\n")
	// On Windows, some tools may inject extra threads.
	// exit() may block on locks held by those threads, so forcibly exit.
	_ = os.Stderr.Sync()
	_ = os.Stdout.Sync()
	os.Exit(1)
}

// Log a warning message.
func warningf(msg string, s ...interface{}) {
	fmt.Fprintf(os.Stderr, "nin: warning: ")
	fmt.Fprintf(os.Stderr, msg, s...)
	fmt.Fprintf(os.Stderr, "\n")
}

// Log an error message.
func errorf(msg string, s ...interface{}) {
	fmt.Fprintf(os.Stderr, "nin: error: ")
	fmt.Fprintf(os.Stderr, msg, s...)
	fmt.Fprintf(os.Stderr, "\n")
}

// Log an informational message.
func infof(msg string, s ...interface{}) {
	fmt.Fprintf(os.Stdout, "nin: ")
	fmt.Fprintf(os.Stdout, msg, s...)
	fmt.Fprintf(os.Stdout, "\n")
}

func isPathSeparator(c byte) bool {
	return c == '/' || c == '\\'
}

// Canonicalize a path like "foo/../bar.h" into just "bar.h".
func CanonicalizePath(path string) string {
	// TODO(maruel): Call site should be the lexers, so that it's done as a
	// single pass.
	// WARNING: this function is performance-critical; please benchmark
	// any changes you make to it.
	l := len(path)
	if l == 0 {
		return path
	}

	p := make([]byte, l+1)
	copy(p, path)
	// Tell the compiler that l is safe for p.
	_ = p[l]
	dst := 0
	src := 0

	if c := p[src]; c == '/' || c == '\\' {
		if runtime.GOOS == "windows" && l > 1 {
			// network path starts with //
			if c := p[src+1]; c == '/' || c == '\\' {
				src += 2
				dst += 2
			} else {
				src++
				dst++
			}
		} else {
			src++
			dst++
		}
	}

	var components [60]int
	for componentCount := 0; src < l; {
		if p[src] == '.' {
			// It is fine to read one byte past because p is l+1 in
			// length. It will be a 0 zero if so.
			c := p[src+1]
			if src+1 == l || (c == '/' || c == '\\') {
				// '.' component; eliminate.
				src += 2
				continue
			}
			if c == '.' {
				// It is fine to read one byte past because p is l+1 in
				// length. It will be a 0 zero if so.
				c := p[src+2]
				if src+2 == l || (c == '/' || c == '\\') {
					// '..' component.  Back up if possible.
					if componentCount > 0 {
						dst = components[componentCount-1]
						src += 3
						componentCount--
					} else {
						p[dst] = p[src]
						p[dst+1] = p[src+1]
						p[dst+2] = p[src+2]
						dst += 3
						src += 3
					}
					continue
				}
			}
		}

		if c := p[src]; c == '/' || c == '\\' {
			src++
			continue
		}

		if componentCount == len(components) {
			fatalf("path has too many components : %s", path)
		}
		components[componentCount] = dst
		componentCount++

		for src != l {
			c := p[src]
			if c == '/' || c == '\\' {
				break
			}
			p[dst] = c
			dst++
			src++
		}
		// Copy '/' or final \0 character as well.
		p[dst] = p[src]
		dst++
		src++
	}

	if dst == 0 {
		p[dst] = '.'
		dst += 2
	}
	p = p[:dst-1]
	if runtime.GOOS == "windows" {
		for i, c := range p {
			if c == '\\' {
				p[i] = '/'
			}
		}
	}
	return unsafeString(p)
}

// Canonicalize a path like "foo/../bar.h" into just "bar.h".
//
// Returns a bits set starting from lowest for a backslash that was
// normalized to a forward slash. (only used on Windows)
func CanonicalizePathBits(path string) (string, uint64) {
	// TODO(maruel): Call site should be the lexers, so that it's done as a
	// single pass.
	// WARNING: this function is performance-critical; please benchmark
	// any changes you make to it.
	l := len(path)
	if l == 0 {
		return path, 0
	}

	p := make([]byte, l+1)
	copy(p, path)
	// Tell the compiler that l is safe for p.
	_ = p[l]
	dst := 0
	src := 0

	if c := p[src]; c == '/' || c == '\\' {
		if runtime.GOOS == "windows" && l > 1 {
			// network path starts with //
			if c := p[src+1]; c == '/' || c == '\\' {
				src += 2
				dst += 2
			} else {
				src++
				dst++
			}
		} else {
			src++
			dst++
		}
	}

	var components [60]int
	for componentCount := 0; src < l; {
		if p[src] == '.' {
			// It is fine to read one byte past because p is l+1 in
			// length. It will be a 0 zero if so.
			c := p[src+1]
			if src+1 == l || (c == '/' || c == '\\') {
				// '.' component; eliminate.
				src += 2
				continue
			}
			if c == '.' {
				// It is fine to read one byte past because p is l+1 in
				// length. It will be a 0 zero if so.
				c := p[src+2]
				if src+2 == l || (c == '/' || c == '\\') {
					// '..' component.  Back up if possible.
					if componentCount > 0 {
						dst = components[componentCount-1]
						src += 3
						componentCount--
					} else {
						p[dst] = p[src]
						p[dst+1] = p[src+1]
						p[dst+2] = p[src+2]
						dst += 3
						src += 3
					}
					continue
				}
			}
		}

		if c := p[src]; c == '/' || c == '\\' {
			src++
			continue
		}

		if componentCount == len(components) {
			fatalf("path has too many components : %s", path)
		}
		components[componentCount] = dst
		componentCount++

		for src != l {
			c := p[src]
			if c == '/' || c == '\\' {
				break
			}
			p[dst] = c
			dst++
			src++
		}
		// Copy '/' or final \0 character as well.
		p[dst] = p[src]
		dst++
		src++
	}

	if dst == 0 {
		p[dst] = '.'
		dst += 2
	}
	p = p[:dst-1]
	bits := uint64(0)
	if runtime.GOOS == "windows" {
		bitsMask := uint64(1)
		for i, c := range p {
			switch c {
			case '\\':
				bits |= bitsMask
				p[i] = '/'
				fallthrough
			case '/':
				bitsMask <<= 1
			}
		}
	}
	return unsafeString(p), bits
}

func stringNeedsShellEscaping(input string) bool {
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || '0' <= ch && ch <= '9' {
			continue
		}
		switch ch {
		case '_', '+', '-', '.', '/':
		default:
			return true
		}
	}
	return false
}

func stringNeedsWin32Escaping(input string) bool {
	for i := 0; i < len(input); i++ {
		switch input[i] {
		case ' ', '"':
			return true
		default:
		}
	}
	return false
}

// Escapes the item for bash.
func getShellEscapedString(input string) string {
	if !stringNeedsShellEscaping(input) {
		return input
	}

	const quote = byte('\'')
	// Do one pass to calculate the ending size.
	l := len(input) + 2
	for i := 0; i != len(input); i++ {
		if input[i] == quote {
			l += 3
		}
	}

	out := make([]byte, l)
	out[0] = quote
	offset := 1
	for i := 0; i < len(input); i++ {
		c := input[i]
		out[offset] = c
		if c == quote {
			offset++
			out[offset] = '\\'
			offset++
			out[offset] = '\''
			offset++
			out[offset] = '\''
		}
		offset++
	}
	out[offset] = quote
	return unsafeString(out)
}

// Escapes the item for Windows's CommandLineToArgvW().
func getWin32EscapedString(input string) string {
	if !stringNeedsWin32Escaping(input) {
		return input
	}

	result := "\""
	consecutiveBackslashCount := 0
	spanBegin := 0
	for it, c := range input {
		switch c {
		case '\\':
			consecutiveBackslashCount++
		case '"':
			result += input[spanBegin:it]
			for j := 0; j < consecutiveBackslashCount+1; j++ {
				result += "\\"
			}
			spanBegin = it
			consecutiveBackslashCount = 0
		default:
			consecutiveBackslashCount = 0
		}
	}
	result += input[spanBegin:]
	for j := 0; j < consecutiveBackslashCount; j++ {
		result += "\\"
	}
	result += "\""
	return result
}

/*
// Read a file to a string (in text mode: with CRLF conversion
// on Windows).
// Returns -errno and fills in \a err on error.
func ReadFile(path string, contents *string, err *string) int {
  // This makes a ninja run on a set of 1500 manifest files about 4% faster
  // than using the generic fopen code below.
  err = nil
  f := ::CreateFileA(path, GENERIC_READ, FILE_SHARE_READ, nil, OPEN_EXISTING, FILE_FLAG_SEQUENTIAL_SCAN, nil)
  if f == INVALID_HANDLE_VALUE {
    err.assign(GetLastErrorString())
    return -ENOENT
  }

  for ; ;  {
    var len2 DWORD
    char buf[64 << 10]
    if !::ReadFile(f, buf, sizeof(buf), &len2, nil) {
      err.assign(GetLastErrorString())
      contents = nil
      return -1
    }
    if len2 == 0 {
      break
    }
    contents.append(buf, len2)
  }
  ::CloseHandle(f)
  return 0
}
*/

// Given a misspelled string and a list of correct spellings, returns
// the closest match or "" if there is no close enough match.
func SpellcheckString(text string, words ...string) string {
	const maxValidEditDistance = 3

	minDistance := maxValidEditDistance + 1
	result := ""
	for _, i := range words {
		distance := editDistance(i, text, true, maxValidEditDistance)
		if distance < minDistance {
			minDistance = distance
			result = i
		}
	}
	return result
}

func islatinalpha(c byte) bool {
	// isalpha() is locale-dependent.
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

/*
func calculateProcessorLoad(idleTicks, totalTicks uint64) float64 {
  static uint64T previousIdleTicks = 0
  static uint64T previousTotalTicks = 0
  static double previousLoad = -0.0

  uint64T idleTicksSinceLastTime = idleTicks - previousIdleTicks
  uint64T totalTicksSinceLastTime = totalTicks - previousTotalTicks

  bool firstCall = (previousTotalTicks == 0)
  bool ticksNotUpdatedSinceLastCall = (totalTicksSinceLastTime == 0)

  double load
  if (firstCall || ticksNotUpdatedSinceLastCall) {
    load = previousLoad
  } else {
    // Calculate load.
    double idleToTotalRatio =
        ((double)idleTicksSinceLastTime) / totalTicksSinceLastTime
    double loadSinceLastCall = 1.0 - idleToTotalRatio

    // Filter/smooth result when possible.
    if(previousLoad > 0) {
      load = 0.9 * previousLoad + 0.1 * loadSinceLastCall
    } else {
      load = loadSinceLastCall
    }
  }

  previousLoad = load
  previousTotalTicks = totalTicks
  previousIdleTicks = idleTicks

  return load
}

uint64T FileTimeToTickCount(const FILETIME & ft)
{
  uint64T high = (((uint64T)(ft.dwHighDateTime)) << 32)
  uint64T low  = ft.dwLowDateTime
  return (high | low)
}
*/

// @return the load average of the machine. A negative value is returned
// on error.
func getLoadAverage() float64 {
	/*
	  FILETIME idleTime, kernelTime, userTime
	  BOOL getSystemTimeSucceeded =
	      GetSystemTimes(&idleTime, &kernelTime, &userTime)

	  posixCompatibleLoad := 0.
	  if getSystemTimeSucceeded {
	    idleTicks := FileTimeToTickCount(idleTime)

	    // kernelTime from GetSystemTimes already includes idleTime.
	    uint64T totalTicks =
	        FileTimeToTickCount(kernelTime) + FileTimeToTickCount(userTime)

	    processorLoad := calculateProcessorLoad(idleTicks, totalTicks)
	    posixCompatibleLoad = processorLoad * GetProcessorCount()

	  } else {
	    posixCompatibleLoad = -0.0
	  }

	  return posixCompatibleLoad
	*/
	return 0
}

/*
// @return the load average of the machine. A negative value is returned
// on error.
func getLoadAverage() float64 {
  return -0.0f
}

// @return the load average of the machine. A negative value is returned
// on error.
func getLoadAverage() float64 {
  var cpuStats perfstatCpuTotalT
  if perfstatCpuTotal(nil, &cpuStats, sizeof(cpuStats), 1) < 0 {
    return -0.0f
  }

  // Calculation taken from comment in libperfstats.h
  return double(cpuStats.loadavg[0]) / double(1 << SBITS)
}

// @return the load average of the machine. A negative value is returned
// on error.
func getLoadAverage() float64 {
  var si sysinfo
  if sysinfo(&si) != 0 {
    return -0.0f
  }
  return 1.0 / (1 << SI_LOAD_SHIFT) * si.loads[0]
}

// @return the load average of the machine. A negative value is returned
// on error.
func getLoadAverage() float64 {
    return -0.0f
}
*/

// Elide the given string @a str with '...' in the middle if the length
// exceeds @a width.
func elideMiddle(str string, width int) string {
	switch width {
	case 0:
		return ""
	case 1:
		return "."
	case 2:
		return ".."
	case 3:
		return "..."
	}
	const margin = 3 // Space for "...".
	result := str
	if len(result) > width {
		elideSize := (width - margin) / 2
		result = result[0:elideSize] + "..." + result[len(result)-elideSize:]
	}
	return result
}

// unsafeString performs an unsafe conversion from a []byte to a string. The
// returned string will share the underlying memory with the []byte which thus
// allows the string to be mutable through the []byte. We're careful to use
// this method only in situations in which the []byte will not be modified.
//
// A workaround for the absence of https://github.com/golang/go/issues/2632.
func unsafeString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
