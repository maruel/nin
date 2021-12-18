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


// Can answer questions about the manifest for the BuildLog.
type BuildLogUser struct {
}

// Store a log of every command ran for every build.
// It has a few uses:
//
// 1) (hashes of) command lines for existing output files, so we know
//    when we need to rebuild due to the command changing
// 2) timing information, perhaps for generating reports
// 3) restat information
type BuildLog struct {

  Entries typedef ExternalStringHashMap<LogEntry*>::Type

  entries_ Entries
  log_file_ *FILE
  log_file_path_ string
  needs_recompaction_ bool
}
type LogEntry struct {
  output string
  command_hash uint64
  start_time int
  end_time int
  mtime TimeStamp

  // Used by tests.
  bool operator==(const LogEntry& o) {
    return output == o.output && command_hash == o.command_hash &&
        start_time == o.start_time && end_time == o.end_time &&
        mtime == o.mtime
  }

  LogEntry(string output, uint64_t command_hash, int start_time, int end_time, TimeStamp restat_mtime)
  }
func (b *BuildLog) entries() *Entries {
	return b.entries_
}


// On AIX, inttypes.h gets indirectly included by build_log.h.
// It's easiest just to ask for the printf format macros right away.

// Implementation details:
// Each run's log appends to the log file.
// To load, we run through all log entries in series, throwing away
// older runs.
// Once the number of redundant entries exceeds a threshold, we write
// out a new file and replace the existing one with it.

const char kFileSignature[] = "# ninja log v%d\n"
const int kOldestSupportedVersion = 4
const int kCurrentVersion = 5

// 64bit MurmurHash2, by Austin Appleby
inline
func MurmurHash64A(key *void, len uint) uint64 {
  seed := 0xDECAFBADDECAFBADull
  m := BIG_CONSTANT(0xc6a4a7935bd1e995)
  r := 47
  uint64_t h = seed ^ (len * m)
  data := (const unsigned char*)key
  while len >= 8 {
    var k uint64
    memcpy(&k, data, sizeof k)
    k *= m
    k ^= k >> r
    k *= m
    h ^= k
    h *= m
    data += 8
    len -= 8
  }
  switch (len & 7)
  {
  case 7: h ^= uint64_t(data[6]) << 48
          NINJA_FALLTHROUGH
  case 6: h ^= uint64_t(data[5]) << 40
          NINJA_FALLTHROUGH
  case 5: h ^= uint64_t(data[4]) << 32
          NINJA_FALLTHROUGH
  case 4: h ^= uint64_t(data[3]) << 24
          NINJA_FALLTHROUGH
  case 3: h ^= uint64_t(data[2]) << 16
          NINJA_FALLTHROUGH
  case 2: h ^= uint64_t(data[1]) << 8
          NINJA_FALLTHROUGH
  case 1: h ^= uint64_t(data[0])
          h *= m
  }
  h ^= h >> r
  h *= m
  h ^= h >> r
  return h
}

// static
uint64_t BuildLog::LogEntry::HashCommand(string command) {
  return MurmurHash64A(command.str_, command.len_)
}

BuildLog::LogEntry::LogEntry(string output)
  : output(output) {}

BuildLog::LogEntry::LogEntry(string output, uint64_t command_hash, int start_time, int end_time, TimeStamp restat_mtime)
  : output(output), command_hash(command_hash),
    start_time(start_time), end_time(end_time), mtime(restat_mtime)
{}

BuildLog::BuildLog()
  : log_file_(nil), needs_recompaction_(false) {}

BuildLog::~BuildLog() {
  Close()
}

// Prepares writing to the log file without actually opening it - that will
// happen when/if it's needed
func (b *BuildLog) OpenForWrite(path string, user *BuildLogUser, err *string) bool {
  if b.needs_recompaction_ {
    if !Recompact(path, user, err) {
      return false
    }
  }

  if !!b.log_file_ { panic("oops") }
  b.log_file_path_ = path  // we don't actually open the file right now, but will
                          // do so on the first write attempt
  return true
}

func (b *BuildLog) RecordCommand(edge *Edge, start_time int, end_time int, mtime TimeStamp) bool {
  command := edge.EvaluateCommand(true)
  command_hash := LogEntry::HashCommand(command)
  for out := edge.outputs_.begin(); out != edge.outputs_.end(); out++ {
    path := (*out).path()
    i := b.entries_.find(path)
    var log_entry *LogEntry
    if i != b.entries_.end() {
      log_entry = i.second
    } else {
      log_entry = new LogEntry(path)
      b.entries_.insert(Entries::value_type(log_entry.output, log_entry))
    }
    log_entry.command_hash = command_hash
    log_entry.start_time = start_time
    log_entry.end_time = end_time
    log_entry.mtime = mtime

    if !OpenForWriteIfNeeded() {
      return false
    }
    if b.log_file_ {
      if !WriteEntry(b.log_file_, *log_entry) {
        return false
      }
      if fflush(b.log_file_) != 0 {
          return false
      }
    }
  }
  return true
}

func (b *BuildLog) Close() {
  OpenForWriteIfNeeded()  // create the file even if nothing has been recorded
  if b.log_file_ {
    fclose(b.log_file_)
  }
  b.log_file_ = nil
}

// Should be called before using log_file_. When false is returned, errno
// will be set.
func (b *BuildLog) OpenForWriteIfNeeded() bool {
  if b.log_file_ || b.log_file_path_.empty() {
    return true
  }
  b.log_file_ = fopen(b.log_file_path_, "ab")
  if !b.log_file_ {
    return false
  }
  if setvbuf(b.log_file_, nil, _IOLBF, BUFSIZ) != 0 {
    return false
  }
  SetCloseOnExec(fileno(b.log_file_))

  // Opening a file in append mode doesn't set the file pointer to the file's
  // end on Windows. Do that explicitly.
  fseek(b.log_file_, 0, SEEK_END)

  if ftell(b.log_file_) == 0 {
    if fprintf(b.log_file_, kFileSignature, kCurrentVersion) < 0 {
      return false
    }
  }
  return true
}

type LineReader struct {
  explicit LineReader(FILE* file)
    : file_(file), buf_end_(buf_), line_start_(buf_), line_end_(nil) {
      memset(buf_, 0, sizeof(buf_))
  }

  file_ *FILE
  char buf_[256 << 10]
  buf_end_ *char  // Points one past the last valid byte in |buf_|.

  line_start_ *char
  // Points at the next \n in buf_ after line_start, or NULL.
  line_end_ *char
}
// Reads a \n-terminated line from the file passed to the constructor.
// On return, *line_start points to the beginning of the next line, and
// *line_end points to the \n at the end of the line. If no newline is seen
// in a fixed buffer size, *line_end is set to NULL. Returns false on EOF.
func (l *LineReader) ReadLine(line_start *char*, line_end *char*) bool {
  if l.line_start_ >= l.buf_end_ || !l.line_end_ {
    // Buffer empty, refill.
    size_read := fread(l.buf_, 1, sizeof(l.buf_), l.file_)
    if !size_read {
      return false
    }
    l.line_start_ = l.buf_
    l.buf_end_ = l.buf_ + size_read
  } else {
    // Advance to next line in buffer.
    l.line_start_ = l.line_end_ + 1
  }

  l.line_end_ = (char*)memchr(l.line_start_, '\n', l.buf_end_ - l.line_start_)
  if !l.line_end_ {
    // No newline. Move rest of data to start of buffer, fill rest.
    size_t already_consumed = l.line_start_ - l.buf_
    size_t size_rest = (l.buf_end_ - l.buf_) - already_consumed
    memmove(l.buf_, l.line_start_, size_rest)

    size_t read = fread(l.buf_ + size_rest, 1, sizeof(l.buf_) - size_rest, l.file_)
    l.buf_end_ = l.buf_ + size_rest + read
    l.line_start_ = l.buf_
    l.line_end_ = (char*)memchr(l.line_start_, '\n', l.buf_end_ - l.line_start_)
  }

  *line_start = l.line_start_
  *line_end = l.line_end_
  return true
}

// Load the on-disk log.
func (b *BuildLog) Load(path string, err *string) LoadStatus {
  METRIC_RECORD(".ninja_log load")
  FILE* file = fopen(path, "r")
  if file == nil {
    if errno == ENOENT {
      return LOAD_NOT_FOUND
    }
    *err = strerror(errno)
    return LOAD_ERROR
  }

  log_version := 0
  unique_entry_count := 0
  total_entry_count := 0

  LineReader reader(file)
  line_start := 0
  line_end := 0
  while reader.ReadLine(&line_start, &line_end) {
    if !log_version {
      sscanf(line_start, kFileSignature, &log_version)

      if log_version < kOldestSupportedVersion {
        *err = ("build log version invalid, perhaps due to being too old; " "starting over")
        fclose(file)
        unlink(path)
        // Don't report this as a failure.  An empty build log will cause
        // us to rebuild the outputs anyway.
        return LOAD_SUCCESS
      }
    }

    // If no newline was found in this chunk, read the next.
    if !line_end {
      continue
    }

    const char kFieldSeparator = '\t'

    start := line_start
    char* end = (char*)memchr(start, kFieldSeparator, line_end - start)
    if end == nil {
      continue
    }
    *end = 0

    int start_time = 0, end_time = 0
    restat_mtime := 0

    start_time = atoi(start)
    start = end + 1

    end = (char*)memchr(start, kFieldSeparator, line_end - start)
    if end == nil {
      continue
    }
    *end = 0
    end_time = atoi(start)
    start = end + 1

    end = (char*)memchr(start, kFieldSeparator, line_end - start)
    if end == nil {
      continue
    }
    *end = 0
    restat_mtime = strtoll(start, nil, 10)
    start = end + 1

    end = (char*)memchr(start, kFieldSeparator, line_end - start)
    if end == nil {
      continue
    }
    string output = string(start, end - start)

    start = end + 1
    end = line_end

    var entry *LogEntry
    i := b.entries_.find(output)
    if i != b.entries_.end() {
      entry = i.second
    } else {
      entry = new LogEntry(output)
      b.entries_.insert(Entries::value_type(entry.output, entry))
      unique_entry_count++
    }
    total_entry_count++

    entry.start_time = start_time
    entry.end_time = end_time
    entry.mtime = restat_mtime
    if log_version >= 5 {
      char c = *end; *end = '\0'
      entry.command_hash = (uint64_t)strtoull(start, nil, 16)
      *end = c
    } else {
      entry.command_hash = LogEntry::HashCommand(string(start, end - start))
    }
  }
  fclose(file)

  if !line_start {
    return LOAD_SUCCESS // file was empty
  }

  // Decide whether it's time to rebuild the log:
  // - if we're upgrading versions
  // - if it's getting large
  kMinCompactionEntryCount := 100
  kCompactionRatio := 3
  if log_version < kCurrentVersion {
    b.needs_recompaction_ = true
  } else if total_entry_count > kMinCompactionEntryCount && total_entry_count > unique_entry_count * kCompactionRatio {
    b.needs_recompaction_ = true
  }

  return LOAD_SUCCESS
}

// Lookup a previously-run command by its output path.
func (b *BuildLog) LookupByOutput(path string) *BuildLog::LogEntry {
  i := b.entries_.find(path)
  if i != b.entries_.end() {
    return i.second
  }
  return nil
}

// Serialize an entry into a log file.
func (b *BuildLog) WriteEntry(f *FILE, entry *LogEntry) bool {
  return fprintf(f, "%d\t%d\t%" PRId64 "\t%s\t%" PRIx64 "\n", entry.start_time, entry.end_time, entry.mtime, entry.output, entry.command_hash) > 0
}

// Rewrite the known log entries, throwing away old data.
func (b *BuildLog) Recompact(path string, user *BuildLogUser, err *string) bool {
  METRIC_RECORD(".ninja_log recompact")

  Close()
  string temp_path = path + ".recompact"
  FILE* f = fopen(temp_path, "wb")
  if f == nil {
    *err = strerror(errno)
    return false
  }

  if fprintf(f, kFileSignature, kCurrentVersion) < 0 {
    *err = strerror(errno)
    fclose(f)
    return false
  }

  var dead_outputs []string
  for i := b.entries_.begin(); i != b.entries_.end(); i++ {
    if user.IsPathDead(i.first) {
      dead_outputs.push_back(i.first)
      continue
    }

    if !WriteEntry(f, *i.second) {
      *err = strerror(errno)
      fclose(f)
      return false
    }
  }

  for i := 0; i < dead_outputs.size(); i++ {
    b.entries_.erase(dead_outputs[i])
  }

  fclose(f)
  if unlink(path) < 0 {
    *err = strerror(errno)
    return false
  }

  if rename(temp_path, path) < 0 {
    *err = strerror(errno)
    return false
  }

  return true
}

// Restat all outputs in the log
func (b *BuildLog) Restat(path string, disk_interface *DiskInterface, output_count int, outputs *char*, err *string) bool {
  METRIC_RECORD(".ninja_log restat")

  Close()
  string temp_path = path.AsString() + ".restat"
  FILE* f = fopen(temp_path, "wb")
  if f == nil {
    *err = strerror(errno)
    return false
  }

  if fprintf(f, kFileSignature, kCurrentVersion) < 0 {
    *err = strerror(errno)
    fclose(f)
    return false
  }
  for i := b.entries_.begin(); i != b.entries_.end(); i++ {
    skip := output_count > 0
    for j := 0; j < output_count; j++ {
      if i.second.output == outputs[j] {
        skip = false
        break
      }
    }
    if skip == nil {
      mtime := disk_interface.Stat(i.second.output, err)
      if mtime == -1 {
        fclose(f)
        return false
      }
      i.second.mtime = mtime
    }

    if !WriteEntry(f, *i.second) {
      *err = strerror(errno)
      fclose(f)
      return false
    }
  }

  fclose(f)
  if unlink(path.str_) < 0 {
    *err = strerror(errno)
    return false
  }

  if rename(temp_path, path.str_) < 0 {
    *err = strerror(errno)
    return false
  }

  return true
}

