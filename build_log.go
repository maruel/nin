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
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"unsafe"
)

// Can answer questions about the manifest for the BuildLog.
type BuildLogUser interface {
	IsPathDead(s string) bool
}

// Store a log of every command ran for every build.
// It has a few uses:
//
// 1) (hashes of) command lines for existing output files, so we know
//    when we need to rebuild due to the command changing
// 2) timing information, perhaps for generating reports
// 3) restat information
type BuildLog struct {
	entries_            Entries
	log_file_           *os.File
	log_file_path_      string
	needs_recompaction_ bool
}

func NewBuildLog() BuildLog {
	return BuildLog{entries_: Entries{}}
}

type LogEntry struct {
	output       string
	command_hash uint64
	start_time   int32
	end_time     int32
	mtime        TimeStamp
}

// Used by tests.
func (l *LogEntry) Equal(r *LogEntry) bool {
	return l.output == r.output && l.command_hash == r.command_hash &&
		l.start_time == r.start_time && l.end_time == r.end_time &&
		l.mtime == r.mtime
}

//type Entries ExternalStringHashMap<LogEntry*>::Type
type Entries map[string]*LogEntry

func (b *BuildLog) entries() Entries {
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

const BuildLogFileSignature = "# ninja log v%d\n"
const BuildLogOldestSupportedVersion = 4
const BuildLogCurrentVersion = 5

// 64bit MurmurHash2, by Austin Appleby
func MurmurHash64A(data []byte) uint64 {
	seed := uint64(0xDECAFBADDECAFBAD)
	const m = 0xc6a4a7935bd1e995
	r := 47
	len2 := uint64(len(data))
	h := seed ^ (len2 * m)
	for ; len2 >= 8; len2 -= 8 {
		// TODO(maruel): Assumes little endian.
		k := *(*uint64)(unsafe.Pointer(&data[0]))
		k *= m
		k ^= k >> r
		k *= m
		h ^= k
		h *= m
		data = data[8:]
	}
	switch len2 & 7 {
	case 7:
		h ^= uint64(data[6]) << 48
		fallthrough
	case 6:
		h ^= uint64(data[5]) << 40
		fallthrough
	case 5:
		h ^= uint64(data[4]) << 32
		fallthrough
	case 4:
		h ^= uint64(data[3]) << 24
		fallthrough
	case 3:
		h ^= uint64(data[2]) << 16
		fallthrough
	case 2:
		h ^= uint64(data[1]) << 8
		fallthrough
	case 1:
		h ^= uint64(data[0])
		h *= m
	}
	h ^= h >> r
	h *= m
	h ^= h >> r
	return h
}

func HashCommand(command string) uint64 {
	// TODO(maruel): Memory copy.
	return MurmurHash64A([]byte(command))
}

// Prepares writing to the log file without actually opening it - that will
// happen when/if it's needed
func (b *BuildLog) OpenForWrite(path string, user BuildLogUser, err *string) bool {
	if b.needs_recompaction_ {
		if !b.Recompact(path, user, err) {
			return false
		}
	}

	if b.log_file_ != nil {
		panic("oops")
	}
	b.log_file_path_ = path
	// we don't actually open the file right now, but will
	// do so on the first write attempt
	return true
}

func (b *BuildLog) RecordCommand(edge *Edge, start_time, end_time int32, mtime TimeStamp) bool {
	command := edge.EvaluateCommand(true)
	command_hash := HashCommand(command)
	for _, out := range edge.outputs_ {
		path := out.path()
		i, ok := b.entries_[path]
		var log_entry *LogEntry
		if ok {
			log_entry = i
		} else {
			log_entry = &LogEntry{output: path}
			b.entries_[log_entry.output] = log_entry
		}
		log_entry.command_hash = command_hash
		log_entry.start_time = start_time
		log_entry.end_time = end_time
		log_entry.mtime = mtime

		if !b.OpenForWriteIfNeeded() {
			return false
		}
		if b.log_file_ != nil {
			if err2 := WriteEntry(b.log_file_, log_entry); err2 != nil {
				return false
			}
			/* TODO(maruel): Too expensive.
			if err := b.log_file_Sync(); err != nil {
				return false
			}
			*/
		}
	}
	return true
}

func (b *BuildLog) Close() error {
	b.OpenForWriteIfNeeded() // create the file even if nothing has been recorded
	if b.log_file_ != nil {
		_ = b.log_file_.Close()
	}
	b.log_file_ = nil
	return nil
}

// Should be called before using log_file_. When false is returned, errno
// will be set.
func (b *BuildLog) OpenForWriteIfNeeded() bool {
	if b.log_file_ != nil || b.log_file_path_ == "" {
		return true
	}
	b.log_file_, _ = os.OpenFile(b.log_file_path_, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o0666)
	if b.log_file_ == nil {
		return false
	}
	/*if setvbuf(b.log_file_, nil, _IOLBF, BUFSIZ) != 0 {
		return false
	}
	SetCloseOnExec(fileno(b.log_file_))
	*/

	// Opening a file in append mode doesn't set the file pointer to the file's
	// end on Windows. Do that explicitly.
	p, _ := b.log_file_.Seek(0, os.SEEK_END)

	if p == 0 {
		if _, err := fmt.Fprintf(b.log_file_, BuildLogFileSignature, BuildLogCurrentVersion); err != nil {
			return false
		}
	}
	return true
}

/*
type LineReader struct {

  file_ *FILE
  char buf_[256 << 10]
  buf_end_ *char  // Points one past the last valid byte in |buf_|.

  line_start_ *char
  // Points at the next \n in buf_ after line_start, or NULL.
  line_end_ *char
}
func NewLineReader(file *FILE) LineReader {
	return LineReader{
		file_: file,
		buf_end_: buf_,
		line_start_: buf_,
		line_end_: nil,
	}
	{ memset(buf_, 0, sizeof(buf_)); }
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
*/

// Load the on-disk log.
func (b *BuildLog) Load(path string, err *string) LoadStatus {
	defer METRIC_RECORD(".ninja_log load")()
	file, err2 := ioutil.ReadFile(path)
	if file == nil {
		if os.IsNotExist(err2) {
			return LOAD_NOT_FOUND
		}
		*err = err2.Error()
		return LOAD_ERROR
	}

	if len(file) == 0 {
		return LOAD_SUCCESS // file was empty
	}

	log_version := 0
	unique_entry_count := 0
	total_entry_count := 0

	// TODO(maruel): The LineReader implementation above is significantly faster
	// because it modifies the data in-place.
	reader := bytes.NewBuffer(file)
	for {
		line, e := reader.ReadString('\n')
		if e != nil {
			break
		}
		line = line[:len(line)-1]
		if log_version == 0 {
			_, _ = fmt.Sscanf(line, BuildLogFileSignature, &log_version)

			if log_version < BuildLogOldestSupportedVersion {
				*err = "build log version invalid, perhaps due to being too old; starting over"
				_ = os.Remove(path)
				// Don't report this as a failure.  An empty build log will cause
				// us to rebuild the outputs anyway.
				return LOAD_SUCCESS
			}
		}
		const kFieldSeparator = byte('\t')
		end := strings.IndexByte(line, kFieldSeparator)
		if end == -1 {
			continue
		}

		start_time, err2 := strconv.ParseInt(line[:end], 10, 32)
		if err2 != nil {
			// TODO(maruel): Error handling.
			panic(err2)
		}
		line = line[end+1:]
		end = strings.IndexByte(line, kFieldSeparator)
		if end == -1 {
			continue
		}
		end_time, err2 := strconv.ParseInt(line[:end], 10, 32)
		if err2 != nil {
			// TODO(maruel): Error handling.
			panic(err2)
		}
		line = line[end+1:]
		end = strings.IndexByte(line, kFieldSeparator)
		if end == -1 {
			continue
		}
		restat_mtime, err2 := strconv.ParseInt(line[:end], 10, 64)
		if err2 != nil {
			// TODO(maruel): Error handling.
			panic(err2)
		}
		line = line[end+1:]
		end = strings.IndexByte(line, kFieldSeparator)
		if end == -1 {
			continue
		}
		output := line[:end]
		line = line[end+1:]
		var entry *LogEntry
		i, ok := b.entries_[output]
		if ok {
			entry = i
		} else {
			entry = &LogEntry{output: output}
			b.entries_[entry.output] = entry
			unique_entry_count++
		}
		total_entry_count++

		// TODO(maruel): Check overflows.
		entry.start_time = int32(start_time)
		entry.end_time = int32(end_time)
		entry.mtime = TimeStamp(restat_mtime)
		if log_version >= 5 {
			entry.command_hash, _ = strconv.ParseUint(line, 16, 64)
		} else {
			entry.command_hash = HashCommand(line)
		}
	}

	// Decide whether it's time to rebuild the log:
	// - if we're upgrading versions
	// - if it's getting large
	kMinCompactionEntryCount := 100
	kCompactionRatio := 3
	if log_version < BuildLogCurrentVersion {
		b.needs_recompaction_ = true
	} else if total_entry_count > kMinCompactionEntryCount && total_entry_count > unique_entry_count*kCompactionRatio {
		b.needs_recompaction_ = true
	}

	return LOAD_SUCCESS
}

// Lookup a previously-run command by its output path.
func (b *BuildLog) LookupByOutput(path string) *LogEntry {
	return b.entries_[path]
}

// Serialize an entry into a log file.
func WriteEntry(f io.Writer, entry *LogEntry) error {
	_, err := fmt.Fprintf(f, "%d\t%d\t%d\t%s\t%x\n", entry.start_time, entry.end_time, entry.mtime, entry.output, entry.command_hash)
	return err
}

// Rewrite the known log entries, throwing away old data.
func (b *BuildLog) Recompact(path string, user BuildLogUser, err *string) bool {
	defer METRIC_RECORD(".ninja_log recompact")()
	_ = b.Close()
	temp_path := path + ".recompact"
	f, err2 := os.OpenFile(temp_path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o666)
	if f == nil {
		*err = err2.Error()
		return false
	}

	if _, err2 := fmt.Fprintf(f, BuildLogFileSignature, BuildLogCurrentVersion); err2 != nil {
		*err = err2.Error()
		_ = f.Close()
		return false
	}

	var dead_outputs []string
	// TODO(maruel): Save in order?
	for name, entry := range b.entries_ {
		if user.IsPathDead(name) {
			dead_outputs = append(dead_outputs, name)
			continue
		}

		if err2 := WriteEntry(f, entry); err2 != nil {
			*err = err2.Error()
			_ = f.Close()
			return false
		}
	}

	for _, name := range dead_outputs {
		delete(b.entries_, name)
	}

	_ = f.Close()
	if err2 := os.Remove(path); err2 != nil {
		*err = err2.Error()
		return false
	}

	if err2 := os.Rename(temp_path, path); err2 != nil {
		*err = err2.Error()
		return false
	}
	return true
}

// Restat all outputs in the log
func (b *BuildLog) Restat(path string, disk_interface DiskInterface, outputs []string, err *string) bool {
	defer METRIC_RECORD(".ninja_log restat")()
	_ = b.Close()
	temp_path := path + ".restat"
	f, err2 := os.OpenFile(temp_path, os.O_CREATE|os.O_WRONLY, 0o666)
	if f == nil {
		*err = err2.Error()
		return false
	}

	if _, err2 := fmt.Fprintf(f, BuildLogFileSignature, BuildLogCurrentVersion); err2 != nil {
		*err = err2.Error()
		_ = f.Close()
		return false
	}
	for _, i := range b.entries_ {
		skip := len(outputs) > 0
		for j := 0; j < len(outputs); j++ {
			if i.output == outputs[j] {
				skip = false
				break
			}
		}
		if !skip {
			mtime := disk_interface.Stat(i.output, err)
			if mtime == -1 {
				_ = f.Close()
				return false
			}
			i.mtime = mtime
		}

		if err2 := WriteEntry(f, i); err2 != nil {
			*err = err2.Error()
			_ = f.Close()
			return false
		}
	}

	_ = f.Close()
	if err2 := os.Remove(path); err2 != nil {
		*err = err2.Error()
		return false
	}

	if err2 := os.Rename(temp_path, path); err2 != nil {
		*err = err2.Error()
		return false
	}

	return true
}
