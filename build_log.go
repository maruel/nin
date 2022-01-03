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

type LogEntry struct {
	output      string
	commandHash uint64
	startTime   int32
	endTime     int32
	mtime       TimeStamp
}

// Used by tests.
func (l *LogEntry) Equal(r *LogEntry) bool {
	return l.output == r.output && l.commandHash == r.commandHash &&
		l.startTime == r.startTime && l.endTime == r.endTime &&
		l.mtime == r.mtime
}

// Serialize writes an entry into a log file as a text form.
func (e *LogEntry) Serialize(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%d\t%d\t%d\t%s\t%x\n", e.startTime, e.endTime, e.mtime, e.output, e.commandHash)
	return err
}

//type Entries ExternalStringHashMap<LogEntry*>::Type
type Entries map[string]*LogEntry

func (b *BuildLog) entries() Entries {
	return b.entries_
}

// Implementation details:
// Each run's log appends to the log file.
// To load, we run through all log entries in series, throwing away
// older runs.
// Once the number of redundant entries exceeds a threshold, we write
// out a new file and replace the existing one with it.

const (
	BuildLogFileSignature          = "# ninja log v%d\n"
	BuildLogOldestSupportedVersion = 4
	BuildLogCurrentVersion         = 5
)

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

//

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
	entries_           Entries
	logFile_           *os.File
	logFilePath_       string
	needsRecompaction_ bool
}

func NewBuildLog() BuildLog {
	return BuildLog{entries_: Entries{}}
}

// Prepares writing to the log file without actually opening it - that will
// happen when/if it's needed
func (b *BuildLog) OpenForWrite(path string, user BuildLogUser, err *string) bool {
	if b.needsRecompaction_ {
		if !b.Recompact(path, user, err) {
			return false
		}
	}

	if b.logFile_ != nil {
		panic("oops")
	}
	b.logFilePath_ = path
	// we don't actually open the file right now, but will
	// do so on the first write attempt
	return true
}

func (b *BuildLog) RecordCommand(edge *Edge, startTime, endTime int32, mtime TimeStamp) bool {
	command := edge.EvaluateCommand(true)
	commandHash := HashCommand(command)
	for _, out := range edge.Outputs {
		path := out.Path
		i, ok := b.entries_[path]
		var logEntry *LogEntry
		if ok {
			logEntry = i
		} else {
			logEntry = &LogEntry{output: path}
			b.entries_[logEntry.output] = logEntry
		}
		logEntry.commandHash = commandHash
		logEntry.startTime = startTime
		logEntry.endTime = endTime
		logEntry.mtime = mtime

		if !b.OpenForWriteIfNeeded() {
			return false
		}
		if b.logFile_ != nil {
			if err2 := logEntry.Serialize(b.logFile_); err2 != nil {
				return false
			}
			/* TODO(maruel): Too expensive.
			if err := b.logFile_Sync(); err != nil {
				return false
			}
			*/
		}
	}
	return true
}

func (b *BuildLog) Close() error {
	b.OpenForWriteIfNeeded() // create the file even if nothing has been recorded
	if b.logFile_ != nil {
		_ = b.logFile_.Close()
	}
	b.logFile_ = nil
	return nil
}

// Should be called before using logFile_. When false is returned, errno
// will be set.
func (b *BuildLog) OpenForWriteIfNeeded() bool {
	if b.logFile_ != nil || b.logFilePath_ == "" {
		return true
	}
	b.logFile_, _ = os.OpenFile(b.logFilePath_, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o0666)
	if b.logFile_ == nil {
		return false
	}
	/*if setvbuf(b.logFile_, nil, _IOLBF, BUFSIZ) != 0 {
		return false
	}
	SetCloseOnExec(fileno(b.logFile_))
	*/

	// Opening a file in append mode doesn't set the file pointer to the file's
	// end on Windows. Do that explicitly.
	p, _ := b.logFile_.Seek(0, os.SEEK_END)

	if p == 0 {
		if _, err := fmt.Fprintf(b.logFile_, BuildLogFileSignature, BuildLogCurrentVersion); err != nil {
			return false
		}
	}
	return true
}

/*
type LineReader struct {

  file_ *FILE
  char buf_[256 << 10]
  bufEnd_ *char  // Points one past the last valid byte in |buf_|.

  lineStart_ *char
  // Points at the next \n in buf_ after lineStart, or NULL.
  lineEnd_ *char
}
func NewLineReader(file *FILE) LineReader {
	return LineReader{
		file_: file,
		bufEnd_: buf_,
		lineStart_: buf_,
		lineEnd_: nil,
	}
	{ memset(buf_, 0, sizeof(buf_)); }
}
// Reads a \n-terminated line from the file passed to the constructor.
// On return, *lineStart points to the beginning of the next line, and
// *lineEnd points to the \n at the end of the line. If no newline is seen
// in a fixed buffer size, *lineEnd is set to NULL. Returns false on EOF.
func (l *LineReader) ReadLine(lineStart *char*, lineEnd *char*) bool {
  if l.lineStart_ >= l.bufEnd_ || !l.lineEnd_ {
    // Buffer empty, refill.
    sizeRead := fread(l.buf_, 1, sizeof(l.buf_), l.file_)
    if !sizeRead {
      return false
    }
    l.lineStart_ = l.buf_
    l.bufEnd_ = l.buf_ + sizeRead
  } else {
    // Advance to next line in buffer.
    l.lineStart_ = l.lineEnd_ + 1
  }

  l.lineEnd_ = (char*)memchr(l.lineStart_, '\n', l.bufEnd_ - l.lineStart_)
  if !l.lineEnd_ {
    // No newline. Move rest of data to start of buffer, fill rest.
    sizeT alreadyConsumed = l.lineStart_ - l.buf_
    sizeT sizeRest = (l.bufEnd_ - l.buf_) - alreadyConsumed
    memmove(l.buf_, l.lineStart_, sizeRest)

    sizeT read = fread(l.buf_ + sizeRest, 1, sizeof(l.buf_) - sizeRest, l.file_)
    l.bufEnd_ = l.buf_ + sizeRest + read
    l.lineStart_ = l.buf_
    l.lineEnd_ = (char*)memchr(l.lineStart_, '\n', l.bufEnd_ - l.lineStart_)
  }

  *lineStart = l.lineStart_
  *lineEnd = l.lineEnd_
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

	logVersion := 0
	uniqueEntryCount := 0
	totalEntryCount := 0

	// TODO(maruel): The LineReader implementation above is significantly faster
	// because it modifies the data in-place.
	reader := bytes.NewBuffer(file)
	for {
		line, e := reader.ReadString('\n')
		if e != nil {
			break
		}
		line = line[:len(line)-1]
		if logVersion == 0 {
			_, _ = fmt.Sscanf(line, BuildLogFileSignature, &logVersion)

			if logVersion < BuildLogOldestSupportedVersion {
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

		startTime, err2 := strconv.ParseInt(line[:end], 10, 32)
		if err2 != nil {
			// TODO(maruel): Error handling.
			panic(err2)
		}
		line = line[end+1:]
		end = strings.IndexByte(line, kFieldSeparator)
		if end == -1 {
			continue
		}
		endTime, err2 := strconv.ParseInt(line[:end], 10, 32)
		if err2 != nil {
			// TODO(maruel): Error handling.
			panic(err2)
		}
		line = line[end+1:]
		end = strings.IndexByte(line, kFieldSeparator)
		if end == -1 {
			continue
		}
		restatMtime, err2 := strconv.ParseInt(line[:end], 10, 64)
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
			uniqueEntryCount++
		}
		totalEntryCount++

		// TODO(maruel): Check overflows.
		entry.startTime = int32(startTime)
		entry.endTime = int32(endTime)
		entry.mtime = TimeStamp(restatMtime)
		if logVersion >= 5 {
			entry.commandHash, _ = strconv.ParseUint(line, 16, 64)
		} else {
			entry.commandHash = HashCommand(line)
		}
	}

	// Decide whether it's time to rebuild the log:
	// - if we're upgrading versions
	// - if it's getting large
	kMinCompactionEntryCount := 100
	kCompactionRatio := 3
	if logVersion < BuildLogCurrentVersion {
		b.needsRecompaction_ = true
	} else if totalEntryCount > kMinCompactionEntryCount && totalEntryCount > uniqueEntryCount*kCompactionRatio {
		b.needsRecompaction_ = true
	}

	return LOAD_SUCCESS
}

// Lookup a previously-run command by its output path.
func (b *BuildLog) LookupByOutput(path string) *LogEntry {
	return b.entries_[path]
}

// Rewrite the known log entries, throwing away old data.
func (b *BuildLog) Recompact(path string, user BuildLogUser, err *string) bool {
	defer METRIC_RECORD(".ninja_log recompact")()
	_ = b.Close()
	tempPath := path + ".recompact"
	f, err2 := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o666)
	if f == nil {
		*err = err2.Error()
		return false
	}

	if _, err2 := fmt.Fprintf(f, BuildLogFileSignature, BuildLogCurrentVersion); err2 != nil {
		*err = err2.Error()
		_ = f.Close()
		return false
	}

	var deadOutputs []string
	// TODO(maruel): Save in order?
	for name, entry := range b.entries_ {
		if user.IsPathDead(name) {
			deadOutputs = append(deadOutputs, name)
			continue
		}

		if err2 := entry.Serialize(f); err2 != nil {
			*err = err2.Error()
			_ = f.Close()
			return false
		}
	}

	for _, name := range deadOutputs {
		delete(b.entries_, name)
	}

	_ = f.Close()
	if err2 := os.Remove(path); err2 != nil {
		*err = err2.Error()
		return false
	}

	if err2 := os.Rename(tempPath, path); err2 != nil {
		*err = err2.Error()
		return false
	}
	return true
}

// Restat all outputs in the log
func (b *BuildLog) Restat(path string, diskInterface DiskInterface, outputs []string, err *string) bool {
	defer METRIC_RECORD(".ninja_log restat")()
	_ = b.Close()
	tempPath := path + ".restat"
	f, err2 := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY, 0o666)
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
			mtime := diskInterface.Stat(i.output, err)
			if mtime == -1 {
				_ = f.Close()
				return false
			}
			i.mtime = mtime
		}

		if err2 := i.Serialize(f); err2 != nil {
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

	if err2 := os.Rename(tempPath, path); err2 != nil {
		*err = err2.Error()
		return false
	}

	return true
}
