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

// Canonicalize a path like "foo/../bar.h" into just "bar.h".
// |slash_bits| has bits set starting from lowest for a backslash that was
// normalized to a forward slash. (only used on Windows)
void CanonicalizePath(string* path, uint64_t* slash_bits)
void CanonicalizePath(char* path, size_t* len, uint64_t* slash_bits)

// Like SpellcheckStringV, but takes a NULL-terminated list.
string SpellcheckString(string text, ...)

// Calls Fatal() with a function name and GetLastErrorString.
NORETURN void Win32Fatal(string function, string hint = nil)


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

void CanonicalizePath(string* path, uint64_t* slash_bits) {
  size_t len = path.size()
  str := 0
  if len > 0 {
    str = &(*path)[0]
  }
  CanonicalizePath(str, &len, slash_bits)
  path.resize(len)
}

static bool IsPathSeparator(char c) {
  return c == '/' || c == '\\'
  return c == '/'
}

void CanonicalizePath(char* path, size_t* len, uint64_t* slash_bits) {
  // WARNING: this function is performance-critical; please benchmark
  // any changes you make to it.
  if *len == 0 {
    return
  }

  const int kMaxPathComponents = 60
  char* components[kMaxPathComponents]
  component_count := 0

  start := path
  dst := start
  src := start
  end := start + *len

  if IsPathSeparator(*src) {

    // network path starts with //
    if *len > 1 && IsPathSeparator(*(src + 1)) {
      src += 2
      dst += 2
    } else {
      ++src
      ++dst
    }
    ++src
    ++dst
  }

  while (src < end) {
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
          --component_count
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
    ++component_count

    while (src != end && !IsPathSeparator(*src))
      *dst++ = *src++
    *dst++ = *src++  // Copy '/' or final \0 character as well.
  }

  if dst == start {
    *dst++ = '.'
    *dst++ = '\0'
  }

  *len = dst - start - 1
  uint64_t bits = 0
  uint64_t bits_mask = 1

  for (char* c = start; c < start + *len; ++c) {
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

static inline bool IsKnownShellSafeCharacter(char ch) {
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

static inline bool IsKnownWin32SafeCharacter(char ch) {
  switch (ch) {
    case ' ':
    case '"':
      return false
    default:
      return true
  }
}

static inline bool StringNeedsShellEscaping(string input) {
  for (size_t i = 0; i < input.size(); ++i) {
    if !IsKnownShellSafeCharacter(input[i]) {
    	return true
    }
  }
  return false
}

static inline bool StringNeedsWin32Escaping(string input) {
  for (size_t i = 0; i < input.size(); ++i) {
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
  assert(result)

  if !StringNeedsShellEscaping(input) {
    result.append(input)
    return
  }

  const char kQuote = '\''
  const char kEscapeSequence[] = "'\\'"

  result.push_back(kQuote)

  string::const_iterator span_begin = input.begin()
  for (string::const_iterator it = input.begin(), end = input.end(); it != end; ++it) {
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
  assert(result)
  if !StringNeedsWin32Escaping(input) {
    result.append(input)
    return
  }

  const char kQuote = '"'
  const char kBackslash = '\\'

  result.push_back(kQuote)
  size_t consecutive_backslash_count = 0
  string::const_iterator span_begin = input.begin()
  for (string::const_iterator it = input.begin(), end = input.end(); it != end; ++it) {
    switch (*it) {
      case kBackslash:
        ++consecutive_backslash_count
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

  for (;;) {
    DWORD len
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
  f := fopen(path, "rb")
  if f == nil {
    err.assign(strerror(errno))
    return -errno
  }

  struct stat st
  if fstat(fileno(f), &st) < 0 {
    err.assign(strerror(errno))
    fclose(f)
    return -errno
  }

  // +1 is for the resize in ManifestParser::Load
  contents.reserve(st.st_size + 1)

  char buf[64 << 10]
  size_t len
  while (!feof(f) && (len = fread(buf, 1, sizeof(buf), f)) > 0) {
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
func SpellcheckStringV(text string, words *vector<string>) string {
  const bool kAllowReplacements = true
  const int kMaxValidEditDistance = 3

  min_distance := kMaxValidEditDistance + 1
  result := nil
  for (vector<string>::const_iterator i = words.begin(); i != words.end(); ++i) {
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
}

// Convert the value returned by GetLastError() into a string.
func GetLastErrorString() string {
  err := GetLastError()

  char* msg_buf
  FormatMessageA( FORMAT_MESSAGE_ALLOCATE_BUFFER | FORMAT_MESSAGE_FROM_SYSTEM | FORMAT_MESSAGE_IGNORE_INSERTS, nil, err, MAKELANGID(LANG_NEUTRAL, SUBLANG_DEFAULT), (char*)&msg_buf, 0, nil)
  msg := msg_buf
  LocalFree(msg_buf)
  return msg
}

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
  string stripped
  stripped.reserve(in.size())

  for (size_t i = 0; i < in.size(); ++i) {
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
    while (i < in.size() && !islatinalpha(in[i]))
      ++i
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
    cores := 0
    if GetLogicalProcessorInformationEx(RelationProcessorCore, reinterpret_cast<PSYSTEM_LOGICAL_PROCESSOR_INFORMATION_EX>( buf.data()), &len) {
      for (DWORD i = 0; i < len; ) {
        info := reinterpret_cast<PSYSTEM_LOGICAL_PROCESSOR_INFORMATION_EX>( buf.data() + i)
        if info.Relationship == RelationProcessorCore && info.Processor.GroupCount == 1 {
          for (KAFFINITY core_mask = info.Processor.GroupMask[0].Mask; core_mask; core_mask >>= 1) {
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
  // The number of exposed processors might not represent the actual number of
  // processors threads can run on. This happens when a CPU set limitation is
  // active, see https://github.com/ninja-build/ninja/issues/1278
  cpuset_t mask
  CPU_ZERO(&mask)
  if cpuset_getaffinity(CPU_LEVEL_WHICH, CPU_WHICH_TID, -1, sizeof(mask), &mask) == 0 {
  }
  cpu_set_t set
  if sched_getaffinity(getpid(), sizeof(set), &set) == 0 {
  }
}

static double CalculateProcessorLoad(uint64_t idle_ticks, uint64_t total_ticks)
{
  static uint64_t previous_idle_ticks = 0
  static uint64_t previous_total_ticks = 0
  static double previous_load = -0.0

  uint64_t idle_ticks_since_last_time = idle_ticks - previous_idle_ticks
  uint64_t total_ticks_since_last_time = total_ticks - previous_total_ticks

  first_call := (previous_total_ticks == 0)
  ticks_not_updated_since_last_call := (total_ticks_since_last_time == 0)

  double load
  if first_call || ticks_not_updated_since_last_call {
    load = previous_load
  } else {
    // Calculate load.
    double idle_to_total_ratio =
        ((double)idle_ticks_since_last_time) / total_ticks_since_last_time
    load_since_last_call := 1.0 - idle_to_total_ratio

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
func GetLoadAverage() double {
  FILETIME idle_time, kernel_time, user_time
  BOOL get_system_time_succeeded =
      GetSystemTimes(&idle_time, &kernel_time, &user_time)

  double posix_compatible_load
  if get_system_time_succeeded {
    uint64_t idle_ticks = FileTimeToTickCount(idle_time)

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
func GetLoadAverage() double {
  return -0.0f
}
// @return the load average of the machine. A negative value is returned
// on error.
func GetLoadAverage() double {
  perfstat_cpu_total_t cpu_stats
  if perfstat_cpu_total(nil, &cpu_stats, sizeof(cpu_stats), 1) < 0 {
    return -0.0f
  }

  // Calculation taken from comment in libperfstats.h
  return double(cpu_stats.loadavg[0]) / double(1 << SBITS)
}
// @return the load average of the machine. A negative value is returned
// on error.
func GetLoadAverage() double {
  struct sysinfo si
  if sysinfo(&si) != 0 {
    return -0.0f
  }
  return 1.0 / (1 << SI_LOAD_SHIFT) * si.loads[0]
}
// @return the load average of the machine. A negative value is returned
// on error.
func GetLoadAverage() double {
    return -0.0f
}
// @return the load average of the machine. A negative value is returned
// on error.
func GetLoadAverage() double {
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
func ElideMiddle(str string, width size_t) string {
  switch (width) {
      case 0: return ""
      case 1: return "."
      case 2: return ".."
      case 3: return "..."
  }
  const int kMargin = 3  // Space for "...".
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
func Truncate(path string, size size_t, err *string) bool {
  fh := _sopen(path, _O_RDWR | _O_CREAT, _SH_DENYNO, _S_IREAD | _S_IWRITE)
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

