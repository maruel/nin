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

package ginja


// Log a fatal message and exit.
NORETURN void Fatal(string msg, ...)

// Have a generic fall-through for different versions of C/C++.

// Log a warning message.
void Warning(string msg, ...)

// Log an error message.
void Error(string msg, ...)

// Log an informational message.
void Info(string msg, ...)

// Like SpellcheckStringV, but takes a NULL-terminated list.
string SpellcheckString(string text, ...)


void Fatal(string msg, ...) {
  va_list ap
  fprintf(stderr, "ninja: fatal: ")
  va_start(ap, msg)
  vfprintf(stderr, msg, ap)
  va_end(ap)
  fprintf(stderr, "\n")
  // On Windows, some tools may inject extra threads.
  // exit() may block on locks held by those threads, so forcibly exit.
  fflush(stderr)
  fflush(stdout)
  ExitProcess(1)
  exit(1)
}

func Warning(msg string, ap va_list) {
  fprintf(stderr, "ninja: warning: ")
  vfprintf(stderr, msg, ap)
  fprintf(stderr, "\n")
}

void Warning(string msg, ...) {
  va_list ap
  va_start(ap, msg)
  Warning(msg, ap)
  va_end(ap)
}

func Error(msg string, ap va_list) {
  fprintf(stderr, "ninja: error: ")
  vfprintf(stderr, msg, ap)
  fprintf(stderr, "\n")
}

void Error(string msg, ...) {
  va_list ap
  va_start(ap, msg)
  Error(msg, ap)
  va_end(ap)
}

func Info(msg string, ap va_list) {
  fprintf(stdout, "ninja: ")
  vfprintf(stdout, msg, ap)
  fprintf(stdout, "\n")
}

void Info(string msg, ...) {
  va_list ap
  va_start(ap, msg)
  Info(msg, ap)
  va_end(ap)
}

// Canonicalize a path like "foo/../bar.h" into just "bar.h".
// |slash_bits| has bits set starting from lowest for a backslash that was
// normalized to a forward slash. (only used on Windows)
func CanonicalizePath(path *string, slash_bits *uint64) {
  len := path.size()
  str := 0
  if len > 0 {
    str = &(*path)[0]
  }
  CanonicalizePath(str, &len, slash_bits)
  path.resize(len)
}

func IsPathSeparator(c char) bool {
  return c == '/' || c == '\\'
  return c == '/'
}

// Canonicalize a path like "foo/../bar.h" into just "bar.h".
// |slash_bits| has bits set starting from lowest for a backslash that was
// normalized to a forward slash. (only used on Windows)
func CanonicalizePath(path *char, len *uint, slash_bits *uint64) {
  // WARNING: this function is performance-critical; please benchmark
  // any changes you make to it.
  if *len == 0 {
    return
  }

  kMaxPathComponents := 60
  char* components[kMaxPathComponents]
  component_count := 0

  start := path
  dst := start
  src := start
  string end = start + *len

  if IsPathSeparator(*src) {

    // network path starts with //
    if *len > 1 && IsPathSeparator(*(src + 1)) {
      src += 2
      dst += 2
    } else {
      src++
      dst++
    }
    src++
    dst++
  }

  while src < end {
    if *src == '.' {
      if src + 1 == end || IsPathSeparator(src[1]) {
        // '.' component; eliminate.
        src += 2
        continue
      } else if src[1] == '.' && (src + 2 == end || IsPathSeparator(src[2])) {
        // '..' component.  Back up if possible.
        if component_count > 0 {
          dst = components[component_count - 1]
          src += 3
          component_count--
        } else {
          *dst++ = *src++
          *dst++ = *src++
          *dst++ = *src++
        }
        continue
      }
    }

    if IsPathSeparator(*src) {
      src++
      continue
    }

    if component_count == kMaxPathComponents {
      Fatal("path has too many components : %s", path)
    }
    components[component_count] = dst
    component_count++

    while src != end && !IsPathSeparator(*src) {
      *dst++ = *src++
    }
    *dst++ = *src++  // Copy '/' or final \0 character as well.
  }

  if dst == start {
    *dst++ = '.'
    *dst++ = '\0'
  }

  *len = dst - start - 1
  bits := 0
  bits_mask := 1

  for c := start; c < start + *len; c++ {
    switch (*c) {
      case '\\':
        bits |= bits_mask
        *c = '/'
        NINJA_FALLTHROUGH
      case '/':
        bits_mask <<= 1
    }
  }

  *slash_bits = bits
  *slash_bits = 0
}

func IsKnownShellSafeCharacter(ch char) inline bool {
  if 'A' <= ch && ch <= 'Z' {
  	return true
  }
  if 'a' <= ch && ch <= 'z' {
  	return true
  }
  if '0' <= ch && ch <= '9' {
  	return true
  }

  switch (ch) {
    case '_':
    case '+':
    case '-':
    case '.':
    case '/':
      return true
    default:
      return false
  }
}

func IsKnownWin32SafeCharacter(ch char) inline bool {
  switch (ch) {
    case ' ':
    case '"':
      return false
    default:
      return true
  }
}

func StringNeedsShellEscaping(input string) inline bool {
  for i := 0; i < input.size(); i++ {
    if !IsKnownShellSafeCharacter(input[i]) {
    	return true
    }
  }
  return false
}

func StringNeedsWin32Escaping(input string) inline bool {
  for i := 0; i < input.size(); i++ {
    if !IsKnownWin32SafeCharacter(input[i]) {
    	return true
    }
  }
  return false
}

// Appends |input| to |*result|, escaping according to the whims of either
// Bash, or Win32's CommandLineToArgvW().
// Appends the string directly to |result| without modification if we can
// determine that it contains no problematic characters.
func GetShellEscapedString(input string, result *string) {
  if !result { panic("oops") }

  if !StringNeedsShellEscaping(input) {
    result.append(input)
    return
  }

  const char kQuote = '\''
  const char kEscapeSequence[] = "'\\'"

  result.push_back(kQuote)

  span_begin := input.begin()
  for string::const_iterator it = input.begin(), end = input.end(); it != end; it++ {
    if *it == kQuote {
      result.append(span_begin, it)
      result.append(kEscapeSequence)
      span_begin = it
    }
  }
  result.append(span_begin, input.end())
  result.push_back(kQuote)
}

func GetWin32EscapedString(input string, result *string) {
  if !result { panic("oops") }
  if !StringNeedsWin32Escaping(input) {
    result.append(input)
    return
  }

  const char kQuote = '"'
  const char kBackslash = '\\'

  result.push_back(kQuote)
  consecutive_backslash_count := 0
  span_begin := input.begin()
  for string::const_iterator it = input.begin(), end = input.end(); it != end; it++ {
    switch (*it) {
      case kBackslash:
        consecutive_backslash_count++
        break
      case kQuote:
        result.append(span_begin, it)
        result.append(consecutive_backslash_count + 1, kBackslash)
        span_begin = it
        consecutive_backslash_count = 0
        break
      default:
        consecutive_backslash_count = 0
        break
    }
  }
  result.append(span_begin, input.end())
  result.append(consecutive_backslash_count, kBackslash)
  result.push_back(kQuote)
}

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
    var len DWORD
    char buf[64 << 10]
    if !::ReadFile(f, buf, sizeof(buf), &len, nil) {
      err.assign(GetLastErrorString())
      contents = nil
      return -1
    }
    if len == 0 {
      break
    }
    contents.append(buf, len)
  }
  ::CloseHandle(f)
  return 0
  FILE* f = fopen(path, "rb")
  if f == nil {
    err.assign(strerror(errno))
    return -errno
  }

  var st stat
  if fstat(fileno(f), &st) < 0 {
    err.assign(strerror(errno))
    fclose(f)
    return -errno
  }

  // +1 is for the resize in ManifestParser::Load
  contents.reserve(st.st_size + 1)

  char buf[64 << 10]
  var len uint
  while !feof(f) && (len = fread(buf, 1, sizeof(buf), f)) > 0 {
    contents.append(buf, len)
  }
  if ferror(f) {
    err.assign(strerror(errno))  // XXX errno?
    contents = nil
    fclose(f)
    return -errno
  }
  fclose(f)
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
  hd := (HANDLE) _get_osfhandle(fd)
  if ! SetHandleInformation(hd, HANDLE_FLAG_INHERIT, 0) {
    fprintf(stderr, "SetHandleInformation(): %s", GetLastErrorString())
  }
}

// Given a misspelled string and a list of correct spellings, returns
// the closest match or NULL if there is no close enough match.
func SpellcheckStringV(text string, words *[]string) string {
  kAllowReplacements := true
  kMaxValidEditDistance := 3

  int min_distance = kMaxValidEditDistance + 1
  result := nil
  for i := words.begin(); i != words.end(); i++ {
    distance := EditDistance(*i, text, kAllowReplacements, kMaxValidEditDistance)
    if distance < min_distance {
      min_distance = distance
      result = *i
    }
  }
  return result
}

string SpellcheckString(string text, ...) {
  // Note: This takes a const char* instead of a string& because using
  // va_start() with a reference parameter is undefined behavior.
  va_list ap
  va_start(ap, text)
  vector<string> words
  string word
  while ((word = va_arg(ap, string)))
    words.push_back(word)
  va_end(ap)
  return SpellcheckStringV(text, words)
}

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

func islatinalpha(c int) bool {
  // isalpha() is locale-dependent.
  return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// Removes all Ansi escape codes (http://www.termsys.demon.co.uk/vtansi.htm).
func StripAnsiEscapeCodes(in string) string {
  stripped := ""
  stripped.reserve(in.size())

  for i := 0; i < in.size(); i++ {
    if in[i] != '\33' {
      // Not an escape code.
      stripped.push_back(in[i])
      continue
    }

    // Only strip CSIs for now.
    if i + 1 >= in.size() {
    	break
    }
    if in[i + 1] != '[' {  // Not a CSI.
    	continue
    }
    i += 2

    // Skip everything up to and including the next [a-zA-Z].
    while i < in.size() && !islatinalpha(in[i]) {
      i++
    }
  }
  return stripped
}

// @return the number of processors on the machine.  Useful for an initial
// guess for how many jobs to run in parallel.  @return 0 on error.
func GetProcessorCount() int {
  // Need to use GetLogicalProcessorInformationEx to get real core count on
  // machines with >64 cores. See https://stackoverflow.com/a/31209344/21475
  len := 0
  if !GetLogicalProcessorInformationEx(RelationProcessorCore, nullptr, &len) && GetLastError() == ERROR_INSUFFICIENT_BUFFER {
    vector<char> buf(len)
    cores := 0
    if GetLogicalProcessorInformationEx(RelationProcessorCore, reinterpret_cast<PSYSTEM_LOGICAL_PROCESSOR_INFORMATION_EX>( buf.data()), &len) {
      for i := 0; i < len;  {
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
  // The number of exposed processors might not represent the actual number of
  // processors threads can run on. This happens when a CPU set limitation is
  // active, see https://github.com/ninja-build/ninja/issues/1278
  var mask cpuset_t
  CPU_ZERO(&mask)
  if cpuset_getaffinity(CPU_LEVEL_WHICH, CPU_WHICH_TID, -1, sizeof(mask), &mask) == 0 {
    return CPU_COUNT(&mask)
  }
  var set cpu_set_t
  if sched_getaffinity(getpid(), sizeof(set), &set) == 0 {
    return CPU_COUNT(&set)
  }
  return sysconf(_SC_NPROCESSORS_ONLN)
}

static double CalculateProcessorLoad(uint64_t idle_ticks, uint64_t total_ticks)
{
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

static uint64_t FileTimeToTickCount(const FILETIME & ft)
{
  uint64_t high = (((uint64_t)(ft.dwHighDateTime)) << 32)
  uint64_t low  = ft.dwLowDateTime
  return (high | low)
}

// @return the load average of the machine. A negative value is returned
// on error.
func GetLoadAverage() float64 {
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
}
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
// @return the load average of the machine. A negative value is returned
// on error.
func GetLoadAverage() float64 {
  double loadavg[3] = { 0.0f, 0.0f, 0.0f }
  if getloadavg(loadavg, 3) < 0 {
    // Maybe we should return an error here or the availability of
    // getloadavg(3) should be checked when ninja is configured.
    return -0.0f
  }
  return loadavg[0]
}

// Elide the given string @a str with '...' in the middle if the length
// exceeds @a width.
func ElideMiddle(str string, width uint) string {
  switch (width) {
      case 0: return ""
      case 1: return "."
      case 2: return ".."
      case 3: return "..."
  }
  kMargin := 3  // Space for "...".
  result := str
  if result.size() > width {
    size_t elide_size = (width - kMargin) / 2
    result = result.substr(0, elide_size)
      + "..."
      + result.substr(result.size() - elide_size, elide_size)
  }
  return result
}

// Truncates a file to the given size.
func Truncate(path string, size uint, err *string) bool {
  int fh = _sopen(path, _O_RDWR | _O_CREAT, _SH_DENYNO, _S_IREAD | _S_IWRITE)
  success := _chsize(fh, size)
  _close(fh)
  success := truncate(path, size)
  // Both truncate() and _chsize() return 0 on success and set errno and return
  // -1 on failure.
  if success < 0 {
    *err = strerror(errno)
    return false
  }
  return true
}

