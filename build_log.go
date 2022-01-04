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
	"reflect"
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
func (l *LogEntry) Serialize(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%d\t%d\t%d\t%s\t%x\n", l.startTime, l.endTime, l.mtime, l.output, l.commandHash)
	return err
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

// HashCommand hashes a command using the MurmurHash2 algorithm by Austin
// Appleby.
func HashCommand(command string) uint64 {
	seed := uint64(0xDECAFBADDECAFBAD)
	const m = 0xc6a4a7935bd1e995
	r := 47
	l := len(command)
	// I tried a few combinations (data as []byte) and this one seemed to be the
	// best. Feel free to micro-optimize.
	data := (*[0x7fff0000]uint64)(unsafe.Pointer((*reflect.StringHeader)(unsafe.Pointer(&command)).Data))[:l/8]
	h := seed ^ (uint64(l) * m)

	i := 0
	for ; i < len(data); i++ {
		k := data[i]
		k *= m
		k ^= k >> r
		k *= m
		h ^= k
		h *= m
	}

	data2 := (*[0x7fff0000]byte)(unsafe.Pointer((*reflect.StringHeader)(unsafe.Pointer(&command)).Data))[8*i : 8*(i+1)]
	switch (l - 8*i) & 7 {
	case 7:
		h ^= uint64(data2[6]) << 48
		fallthrough
	case 6:
		h ^= uint64(data2[5]) << 40
		fallthrough
	case 5:
		h ^= uint64(data2[4]) << 32
		fallthrough
	case 4:
		h ^= uint64(data2[3]) << 24
		fallthrough
	case 3:
		h ^= uint64(data2[2]) << 16
		fallthrough
	case 2:
		h ^= uint64(data2[1]) << 8
		fallthrough
	case 1:
		h ^= uint64(data2[0])
		h *= m
	}
	h ^= h >> r
	h *= m
	h ^= h >> r
	return h
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
	Entries           map[string]*LogEntry
	logFile           *os.File
	logFilePath       string
	needsRecompaction bool
}

// Note: the C++ version uses ExternalStringHashMap<LogEntry*> for
// BuildLog.entries.

func NewBuildLog() BuildLog {
	return BuildLog{Entries: map[string]*LogEntry{}}
}

// Prepares writing to the log file without actually opening it - that will
// happen when/if it's needed
func (b *BuildLog) OpenForWrite(path string, user BuildLogUser, err *string) bool {
	if b.needsRecompaction {
		if !b.Recompact(path, user, err) {
			return false
		}
	}

	if b.logFile != nil {
		panic("oops")
	}
	b.logFilePath = path
	// we don't actually open the file right now, but will
	// do so on the first write attempt
	return true
}

func (b *BuildLog) RecordCommand(edge *Edge, startTime, endTime int32, mtime TimeStamp) bool {
	command := edge.EvaluateCommand(true)
	commandHash := HashCommand(command)
	for _, out := range edge.Outputs {
		path := out.Path
		i, ok := b.Entries[path]
		var logEntry *LogEntry
		if ok {
			logEntry = i
		} else {
			logEntry = &LogEntry{output: path}
			b.Entries[logEntry.output] = logEntry
		}
		logEntry.commandHash = commandHash
		logEntry.startTime = startTime
		logEntry.endTime = endTime
		logEntry.mtime = mtime

		if !b.OpenForWriteIfNeeded() {
			return false
		}
		if b.logFile != nil {
			if err2 := logEntry.Serialize(b.logFile); err2 != nil {
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
	if b.logFile != nil {
		_ = b.logFile.Close()
	}
	b.logFile = nil
	return nil
}

// Should be called before using logFile. When false is returned, errno
// will be set.
func (b *BuildLog) OpenForWriteIfNeeded() bool {
	if b.logFile != nil || b.logFilePath == "" {
		return true
	}
	b.logFile, _ = os.OpenFile(b.logFilePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o0666)
	if b.logFile == nil {
		return false
	}
	/*if setvbuf(b.logFile, nil, _IOLBF, BUFSIZ) != 0 {
		return false
	}
	SetCloseOnExec(fileno(b.logFile))
	*/

	// Opening a file in append mode doesn't set the file pointer to the file's
	// end on Windows. Do that explicitly.
	p, _ := b.logFile.Seek(0, os.SEEK_END)

	if p == 0 {
		if _, err := fmt.Fprintf(b.logFile, BuildLogFileSignature, BuildLogCurrentVersion); err != nil {
			return false
		}
	}
	return true
}

/*
type LineReader struct {

  file *FILE
  char buf[256 << 10]
  bufEnd *char  // Points one past the last valid byte in |buf|.

  lineStart *char
  // Points at the next \n in buf after lineStart, or NULL.
  lineEnd *char
}
func NewLineReader(file *FILE) LineReader {
	return LineReader{
		file: file,
		bufEnd: buf,
		lineStart: buf,
		lineEnd: nil,
	}
	{ memset(buf, 0, sizeof(buf)); }
}
// Reads a \n-terminated line from the file passed to the constructor.
// On return, *lineStart points to the beginning of the next line, and
// *lineEnd points to the \n at the end of the line. If no newline is seen
// in a fixed buffer size, *lineEnd is set to NULL. Returns false on EOF.
func (l *LineReader) ReadLine(lineStart *char*, lineEnd *char*) bool {
  if l.lineStart >= l.bufEnd || !l.lineEnd {
    // Buffer empty, refill.
    sizeRead := fread(l.buf, 1, sizeof(l.buf), l.file)
    if !sizeRead {
      return false
    }
    l.lineStart = l.buf
    l.bufEnd = l.buf + sizeRead
  } else {
    // Advance to next line in buffer.
    l.lineStart = l.lineEnd + 1
  }

  l.lineEnd = (char*)memchr(l.lineStart, '\n', l.bufEnd - l.lineStart)
  if !l.lineEnd {
    // No newline. Move rest of data to start of buffer, fill rest.
    sizeT alreadyConsumed = l.lineStart - l.buf
    sizeT sizeRest = (l.bufEnd - l.buf) - alreadyConsumed
    memmove(l.buf, l.lineStart, sizeRest)

    sizeT read = fread(l.buf + sizeRest, 1, sizeof(l.buf) - sizeRest, l.file)
    l.bufEnd = l.buf + sizeRest + read
    l.lineStart = l.buf
    l.lineEnd = (char*)memchr(l.lineStart, '\n', l.bufEnd - l.lineStart)
  }

  *lineStart = l.lineStart
  *lineEnd = l.lineEnd
  return true
}
*/

// Load the on-disk log.
func (b *BuildLog) Load(path string, err *string) LoadStatus {
	defer metricRecord(".ninja_log load")()
	file, err2 := ioutil.ReadFile(path)
	if file == nil {
		if os.IsNotExist(err2) {
			return LoadNotFound
		}
		*err = err2.Error()
		return LoadError
	}

	if len(file) == 0 {
		return LoadSuccess // file was empty
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
				return LoadSuccess
			}
		}
		const fieldSeparator = byte('\t')
		end := strings.IndexByte(line, fieldSeparator)
		if end == -1 {
			continue
		}

		startTime, err2 := strconv.ParseInt(line[:end], 10, 32)
		if err2 != nil {
			// TODO(maruel): Error handling.
			panic(err2)
		}
		line = line[end+1:]
		end = strings.IndexByte(line, fieldSeparator)
		if end == -1 {
			continue
		}
		endTime, err2 := strconv.ParseInt(line[:end], 10, 32)
		if err2 != nil {
			// TODO(maruel): Error handling.
			panic(err2)
		}
		line = line[end+1:]
		end = strings.IndexByte(line, fieldSeparator)
		if end == -1 {
			continue
		}
		restatMtime, err2 := strconv.ParseInt(line[:end], 10, 64)
		if err2 != nil {
			// TODO(maruel): Error handling.
			panic(err2)
		}
		line = line[end+1:]
		end = strings.IndexByte(line, fieldSeparator)
		if end == -1 {
			continue
		}
		output := line[:end]
		line = line[end+1:]
		var entry *LogEntry
		i, ok := b.Entries[output]
		if ok {
			entry = i
		} else {
			entry = &LogEntry{output: output}
			b.Entries[entry.output] = entry
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
	const minCompactionEntryCount = 100
	const compactionRatio = 3
	if logVersion < BuildLogCurrentVersion {
		b.needsRecompaction = true
	} else if totalEntryCount > minCompactionEntryCount && totalEntryCount > uniqueEntryCount*compactionRatio {
		b.needsRecompaction = true
	}

	return LoadSuccess
}

// Rewrite the known log entries, throwing away old data.
func (b *BuildLog) Recompact(path string, user BuildLogUser, err *string) bool {
	defer metricRecord(".ninja_log recompact")()
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
	for name, entry := range b.Entries {
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
		delete(b.Entries, name)
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
func (b *BuildLog) Restat(path string, di DiskInterface, outputs []string, err *string) bool {
	defer metricRecord(".ninja_log restat")()
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
	for _, i := range b.Entries {
		skip := len(outputs) > 0
		for j := 0; j < len(outputs); j++ {
			if i.output == outputs[j] {
				skip = false
				break
			}
		}
		if !skip {
			mtime, err2 := di.Stat(i.output)
			if mtime == -1 {
				*err = err2.Error()
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
