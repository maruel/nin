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
	"strings"
	"unsafe"
)

// Have a generic fall-through for different versions of C/C++.

// Log a fatal message and exit.
func Fatal(msg string, s ...interface{}) {
	fmt.Fprintf(os.Stderr, "ninja: fatal: ")
	fmt.Fprintf(os.Stderr, msg, s...)
	fmt.Fprintf(os.Stderr, "\n")
	// On Windows, some tools may inject extra threads.
	// exit() may block on locks held by those threads, so forcibly exit.
	_ = os.Stderr.Sync()
	_ = os.Stdout.Sync()
	os.Exit(1)
}

// Log a warning message.
func Warning(msg string, s ...interface{}) {
	fmt.Fprintf(os.Stderr, "ninja: warning: ")
	fmt.Fprintf(os.Stderr, msg, s...)
	fmt.Fprintf(os.Stderr, "\n")
}

// Log an error message.
func Error(msg string, s ...interface{}) {
	fmt.Fprintf(os.Stderr, "ninja: error: ")
	fmt.Fprintf(os.Stderr, msg, s...)
	fmt.Fprintf(os.Stderr, "\n")
}

// Log an informational message.
func Info(msg string, s ...interface{}) {
	fmt.Fprintf(os.Stdout, "ninja: ")
	fmt.Fprintf(os.Stdout, msg, s...)
	fmt.Fprintf(os.Stdout, "\n")
}

func IsPathSeparator(c byte) bool {
	return c == '/' || c == '\\'
}

// Canonicalize a path like "foo/../bar.h" into just "bar.h".
// |slashBits| has bits set starting from lowest for a backslash that was
// normalized to a forward slash. (only used on Windows)
func CanonicalizePath(path string, slashBits *uint64) string {
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
	for component_count := 0; src < l; {
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
					if component_count > 0 {
						dst = components[component_count-1]
						src += 3
						component_count--
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

		if component_count == len(components) {
			Fatal("path has too many components : %s", path)
		}
		components[component_count] = dst
		component_count++

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
		bits := uint64(0)
		bits_mask := uint64(1)
		for i, c := range p {
			switch c {
			case '\\':
				bits |= bits_mask
				p[i] = '/'
				fallthrough
			case '/':
				bits_mask <<= 1
			}
		}
		*slashBits = bits
	}
	return unsafeString(p)
}

func StringNeedsShellEscaping(input string) bool {
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

func StringNeedsWin32Escaping(input string) bool {
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
func GetShellEscapedString(input string) string {
	if !StringNeedsShellEscaping(input) {
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
func GetWin32EscapedString(input string) string {
	if !StringNeedsWin32Escaping(input) {
		return input
	}

	kQuote := '"'
	kBackslash := '\\'

	result := string(kQuote)
	consecutive_backslash_count := 0
	span_begin := 0
	for it, c := range input {
		switch c {
		case kBackslash:
			consecutive_backslash_count++
		case kQuote:
			result += input[span_begin:it]
			for j := 0; j < consecutive_backslash_count+1; j++ {
				result += string(kBackslash)
			}
			span_begin = it
			consecutive_backslash_count = 0
		default:
			consecutive_backslash_count = 0
		}
	}
	result += input[span_begin:]
	for j := 0; j < consecutive_backslash_count; j++ {
		result += string(kBackslash)
	}
	result += string(kQuote)
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

// Mark a file descriptor to not be inherited on exec()s.
func SetCloseOnExec(fd int) {
  flags := fcntl(fd, F_GETFD)
  if flags < 0 {
    perror("fcntl(F_GETFD)")
  } else {
    if fcntl(fd, F_SETFD, flags | FD_CLOEXEC) < 0 {
      perror("fcntl(F_SETFD)")
    }
  }
}
*/

// Given a misspelled string and a list of correct spellings, returns
// the closest match or "" if there is no close enough match.
func SpellcheckString(text string, words ...string) string {
	kAllowReplacements := true
	kMaxValidEditDistance := 3

	min_distance := kMaxValidEditDistance + 1
	result := ""
	for _, i := range words {
		distance := EditDistance(i, text, kAllowReplacements, kMaxValidEditDistance)
		if distance < min_distance {
			min_distance = distance
			result = i
		}
	}
	return result
}

/*
// Convert the value returned by GetLastError() into a string.
func GetLastErrorString() string {
  err := GetLastError()

  var msg_buf *char
  FormatMessageA( FORMAT_MESSAGE_ALLOCATE_BUFFER | FORMAT_MESSAGE_FROM_SYSTEM | FORMAT_MESSAGE_IGNORE_INSERTS, nil, err, MAKELANGID(LANG_NEUTRAL, SUBLANG_DEFAULT), (char*)&msg_buf, 0, nil)
  msg := msg_buf
  LocalFree(msg_buf)
  return msg
}

// Calls Fatal() with a function name and GetLastErrorString.
func Win32Fatal(function string, hint string) {
  if hint != nil {
    Fatal("%s: %s (%s)", function, GetLastErrorString(), hint)
  } else {
    Fatal("%s: %s", function, GetLastErrorString())
  }
}
*/

func islatinalpha(c byte) bool {
	// isalpha() is locale-dependent.
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// Removes all Ansi escape codes (http://www.termsys.demon.co.uk/vtansi.htm).
func StripAnsiEscapeCodes(in string) string {
	if strings.IndexByte(in, '\x1B') == -1 {
		return in
	}
	stripped := ""
	//stripped.reserve(in.size())

	for i := 0; i < len(in); i++ {
		if in[i] != '\x1B' {
			// Not an escape code.
			stripped += string(in[i])
			continue
		}

		// Only strip CSIs for now.
		if i+1 >= len(in) {
			break
		}
		if in[i+1] != '[' { // Not a CSI.
			continue
		}
		i += 2

		// Skip everything up to and including the next [a-zA-Z].
		for i < len(in) && !islatinalpha(in[i]) {
			i++
		}
	}
	return stripped
}

// @return the number of processors on the machine.  Useful for an initial
// guess for how many jobs to run in parallel.  @return 0 on error.
func GetProcessorCount() int {
	return runtime.NumCPU()
	/*
	   // Need to use GetLogicalProcessorInformationEx to get real core count on
	   // machines with >64 cores. See https://stackoverflow.com/a/31209344/21475
	   len2 := 0
	   if !GetLogicalProcessorInformationEx(RelationProcessorCore, nullptr, &len2) && GetLastError() == ERROR_INSUFFICIENT_BUFFER {
	     vector<char> buf(len2)
	     cores := 0
	     if GetLogicalProcessorInformationEx(RelationProcessorCore, reinterpret_cast<PSYSTEM_LOGICAL_PROCESSOR_INFORMATION_EX>( buf.data()), &len2) {
	       for i := 0; i < len2;  {
	         auto info = reinterpret_cast<PSYSTEM_LOGICAL_PROCESSOR_INFORMATION_EX>( buf.data() + i)
	         if info.Relationship == RelationProcessorCore && info.Processor.GroupCount == 1 {
	           for core_mask := info.Processor.GroupMask[0].Mask; core_mask; core_mask >>= 1 {
	             cores += (core_mask & 1)
	           }
	         }
	         i += info.Size
	       }
	       if cores != 0 {
	         return cores
	       }
	     }
	   }
	   return GetActiveProcessorCount(ALL_PROCESSOR_GROUPS)
	   return sysconf(_SC_NPROCESSORS_ONLN)
	*/
}

/*
func CalculateProcessorLoad(idle_ticks, total_ticks uint64) float64 {
  static uint64_t previous_idle_ticks = 0
  static uint64_t previous_total_ticks = 0
  static double previous_load = -0.0

  uint64_t idle_ticks_since_last_time = idle_ticks - previous_idle_ticks
  uint64_t total_ticks_since_last_time = total_ticks - previous_total_ticks

  bool first_call = (previous_total_ticks == 0)
  bool ticks_not_updated_since_last_call = (total_ticks_since_last_time == 0)

  double load
  if (first_call || ticks_not_updated_since_last_call) {
    load = previous_load
  } else {
    // Calculate load.
    double idle_to_total_ratio =
        ((double)idle_ticks_since_last_time) / total_ticks_since_last_time
    double load_since_last_call = 1.0 - idle_to_total_ratio

    // Filter/smooth result when possible.
    if(previous_load > 0) {
      load = 0.9 * previous_load + 0.1 * load_since_last_call
    } else {
      load = load_since_last_call
    }
  }

  previous_load = load
  previous_total_ticks = total_ticks
  previous_idle_ticks = idle_ticks

  return load
}

uint64_t FileTimeToTickCount(const FILETIME & ft)
{
  uint64_t high = (((uint64_t)(ft.dwHighDateTime)) << 32)
  uint64_t low  = ft.dwLowDateTime
  return (high | low)
}
*/

// @return the load average of the machine. A negative value is returned
// on error.
func GetLoadAverage() float64 {
	/*
	  FILETIME idle_time, kernel_time, user_time
	  BOOL get_system_time_succeeded =
	      GetSystemTimes(&idle_time, &kernel_time, &user_time)

	  posix_compatible_load := 0.
	  if get_system_time_succeeded {
	    idle_ticks := FileTimeToTickCount(idle_time)

	    // kernel_time from GetSystemTimes already includes idle_time.
	    uint64_t total_ticks =
	        FileTimeToTickCount(kernel_time) + FileTimeToTickCount(user_time)

	    processor_load := CalculateProcessorLoad(idle_ticks, total_ticks)
	    posix_compatible_load = processor_load * GetProcessorCount()

	  } else {
	    posix_compatible_load = -0.0
	  }

	  return posix_compatible_load
	*/
	return 0
}

/*
// @return the load average of the machine. A negative value is returned
// on error.
func GetLoadAverage() float64 {
  return -0.0f
}

// @return the load average of the machine. A negative value is returned
// on error.
func GetLoadAverage() float64 {
  var cpu_stats perfstat_cpu_total_t
  if perfstat_cpu_total(nil, &cpu_stats, sizeof(cpu_stats), 1) < 0 {
    return -0.0f
  }

  // Calculation taken from comment in libperfstats.h
  return double(cpu_stats.loadavg[0]) / double(1 << SBITS)
}

// @return the load average of the machine. A negative value is returned
// on error.
func GetLoadAverage() float64 {
  var si sysinfo
  if sysinfo(&si) != 0 {
    return -0.0f
  }
  return 1.0 / (1 << SI_LOAD_SHIFT) * si.loads[0]
}

// @return the load average of the machine. A negative value is returned
// on error.
func GetLoadAverage() float64 {
    return -0.0f
}
*/

// Elide the given string @a str with '...' in the middle if the length
// exceeds @a width.
func ElideMiddle(str string, width int) string {
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
	kMargin := 3 // Space for "...".
	result := str
	if len(result) > width {
		elide_size := (width - kMargin) / 2
		result = result[0:elide_size] + "..." + result[len(result)-elide_size:]
	}
	return result
}

/*
// Truncates a file to the given size.
func Truncate(path string, size uint, err *string) bool {
  int fh = _sopen(path, _O_RDWR | _O_CREAT, _SH_DENYNO, _S_IREAD | _S_IWRITE)
  success := _chsize(fh, size)
  _close(fh)
  // Both truncate() and _chsize() return 0 on success and set errno and return
  // -1 on failure.
  if success < 0 {
    *err = strerror(errno)
    return false
  }
  return true
}
*/

// unsafeString performs an unsafe conversion from a []byte to a string. The
// returned string will share the underlying memory with the []byte which thus
// allows the string to be mutable through the []byte. We're careful to use
// this method only in situations in which the []byte will not be modified.
//
// A workaround for the absence of https://github.com/golang/go/issues/2632.
func unsafeString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
