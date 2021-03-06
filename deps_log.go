// Copyright 2012 Google Inc. All Rights Reserved.
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
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
)

// Deps is the reading (startup-time) struct.
type Deps struct {
	MTime TimeStamp
	Nodes []*Node
}

// NewDeps returns an initialized Deps.
func NewDeps(mtime TimeStamp, nodeCount int) *Deps {
	return &Deps{
		MTime: mtime,
		Nodes: make([]*Node, nodeCount),
	}
}

// DepsLog represents a .ninja_deps log file to accelerate incremental build.
//
// As build commands run they can output extra dependency information
// (e.g. header dependencies for C source) dynamically. DepsLog collects
// that information at build time and uses it for subsequent builds.
//
// The on-disk format is based on two primary design constraints:
//
// - it must be written to as a stream (during the build, which may be
// interrupted);
//
// - it can be read all at once on startup. (Alternative designs, where
// it contains indexing information, were considered and discarded as
// too complicated to implement; if the file is small than reading it
// fully on startup is acceptable.)
//
// Here are some stats from the Windows Chrome dependency files, to
// help guide the design space. The total text in the files sums to
// 90mb so some compression is warranted to keep load-time fast.
// There's about 10k files worth of dependencies that reference about
// 40k total paths totalling 2mb of unique strings.
//
// Based on these stats, here's the current design.
//
// The file is structured as version header followed by a sequence of records.
// Each record is either a path string or a dependency list.
// Numbering the path strings in file order gives them dense integer ids.
// A dependency list maps an output id to a list of input ids.
//
// Concretely, a record is:
//    four bytes record length, high bit indicates record type
//      (but max record sizes are capped at 512kB)
//    path records contain the string name of the path, followed by up to 3
//      padding bytes to align on 4 byte boundaries, followed by the
//      one's complement of the expected index of the record (to detect
//      concurrent writes of multiple ninja processes to the log).
//    dependency records are an array of 4-byte integers
//      [output path id,
//       output path mtime (lower 4 bytes), output path mtime (upper 4 bytes),
//       input path id, input path id...]
//      (The mtime is compared against the on-disk output path mtime
//      to verify the stored data is up-to-date.)
// If two records reference the same output the latter one in the file
// wins, allowing updates to just be appended to the file.  A separate
// repacking step can run occasionally to remove dead records.
type DepsLog struct {
	// Maps id -> Node.
	Nodes []*Node
	// Maps id -> Deps of that id.
	Deps []*Deps

	filePath          string
	file              *os.File
	buf               *bufio.Writer
	needsRecompaction bool
}

// The version is stored as 4 bytes after the signature and also serves as a
// byte order mark. Signature and version combined are 16 bytes long.
const (
	depsLogFileSignature  = "# ninjadeps\n"
	depsLogCurrentVersion = uint32(4)
)

// Record size is currently limited to less than the full 32 bit, due to
// internal buffers having to have this size.
const maxRecordSize = (1 << 19) - 1

// OpenForWrite prepares writing to the log file without actually opening it -
// that will happen when/if it's needed.
func (d *DepsLog) OpenForWrite(path string) error {
	if d.needsRecompaction {
		if err := d.Recompact(path); err != nil {
			return err
		}
	}

	if d.file != nil {
		panic("M-A")
	}
	// we don't actually open the file right now, but will do
	// so on the first write attempt
	d.filePath = path
	return nil
}

func (d *DepsLog) recordDeps(node *Node, mtime TimeStamp, nodes []*Node) error {
	nodeCount := len(nodes)
	// Track whether there's any new data to be recorded.
	madeChange := false

	// Assign ids to all nodes that are missing one.
	if node.ID < 0 {
		if err := d.recordID(node); err != nil {
			return err
		}
		madeChange = true
	}
	for i := 0; i < nodeCount; i++ {
		if nodes[i].ID < 0 {
			if err := d.recordID(nodes[i]); err != nil {
				return err
			}
			madeChange = true
		}
	}

	// See if the new data is different than the existing data, if any.
	if !madeChange {
		deps := d.GetDeps(node)
		if deps == nil || deps.MTime != mtime || len(deps.Nodes) != nodeCount {
			madeChange = true
		} else {
			for i := 0; i < nodeCount; i++ {
				if deps.Nodes[i] != nodes[i] {
					madeChange = true
					break
				}
			}
		}
	}

	// Don't write anything if there's no new info.
	if !madeChange {
		return nil
	}

	// Update on-disk representation.
	size := uint32(4 * (1 + 2 + nodeCount))
	if size > maxRecordSize {
		return errors.New("too many dependencies")
	}
	if err := d.openForWriteIfNeeded(); err != nil {
		return err
	}
	size |= 0x80000000 // Deps record: set high bit.

	if err := binary.Write(d.buf, binary.LittleEndian, size); err != nil {
		return err
	}
	if err := binary.Write(d.buf, binary.LittleEndian, uint32(node.ID)); err != nil {
		return err
	}
	if err := binary.Write(d.buf, binary.LittleEndian, uint64(mtime)); err != nil {
		return err
	}
	for i := 0; i < nodeCount; i++ {
		if err := binary.Write(d.buf, binary.LittleEndian, uint32(nodes[i].ID)); err != nil {
			return err
		}
	}
	if err := d.buf.Flush(); err != nil {
		return err
	}

	// Update in-memory representation.
	deps := NewDeps(mtime, nodeCount)
	for i := 0; i < nodeCount; i++ {
		deps.Nodes[i] = nodes[i]
	}
	d.updateDeps(node.ID, deps)
	return nil
}

// Close closes the file handle.
func (d *DepsLog) Close() error {
	// create the file even if nothing has been recorded
	if err := d.openForWriteIfNeeded(); err != nil {
		return err
	}
	var err error
	if d.file != nil {
		if err2 := d.buf.Flush(); err2 != nil {
			err = err2
		}
		if err2 := d.file.Close(); err2 != nil {
			err = err2
		}
	}
	d.buf = nil
	d.file = nil
	return err
}

// Load loads a .ninja_deps to accelerate incremental build.
//
// Note: For version differences, this should migrate to the new format.
// But the v1 format could sometimes (rarely) end up with invalid data, so
// don't migrate v1 to v3 to force a rebuild. (v2 only existed for a few days,
// and there was no release with it, so pretend that it never happened.)
//
// Warning: the whole file content is kept alive.
//
// TODO(maruel): Make it an option so that when used as a library it doesn't
// become a memory bloat. This is especially important when recompacting.
func (d *DepsLog) Load(path string, state *State) (LoadStatus, error) {
	defer metricRecord(".ninja_deps load")()
	// Read the file all at once. The drawback is that it will fail hard on 32
	// bits OS on large builds. This should be rare in 2022. For small builds, it
	// will be fine (and faster).
	data, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return LoadNotFound, err
		}
		return LoadError, err
	}

	// Validate header.
	validHeader := false
	version := uint32(0)
	if len(data) >= len(depsLogFileSignature)+4 && unsafeString(data[:len(depsLogFileSignature)]) == depsLogFileSignature {
		version = binary.LittleEndian.Uint32(data[len(depsLogFileSignature):])
		validHeader = version == depsLogCurrentVersion
	}
	if !validHeader {
		// Don't report this as a failure.  An empty deps log will cause
		// us to rebuild the outputs anyway.
		_ = os.Remove(path)
		if version == 1 {
			return LoadSuccess, errors.New("deps log version change; rebuilding")
		}
		l := bytes.IndexByte(data[:], 0)
		if l <= 0 {
			return LoadSuccess, errors.New("bad deps log signature or version; starting over")
		}
		return LoadSuccess, fmt.Errorf("bad deps log signature %q or version %d; starting over", data[:l], version)
	}

	// Skip the header.
	// TODO(maruel): Calculate if it is faster to do "data = data[4:8]" or use
	// "data[offset+4:offset+8]".
	// Offset is kept to keep the last successful read, to truncate in case of
	// failure.
	offset := int64(len(depsLogFileSignature) + 4)
	data = data[offset:]
	uniqueDepRecordCount := 0
	totalDepRecordCount := 0
	for len(data) != 0 {
		// A minimal record is size (4 bytes) plus one of:
		// - content (>=4 + checksum(4)); CanonicalizePath() rejects empty paths.
		// - (id(4)+mtime(8)+nodes(4x) >12) for deps node.
		if len(data) < 12 {
			err = fmt.Errorf("premature end of file after %d bytes", int(offset)+len(data))
			break
		}
		size := binary.LittleEndian.Uint32(data[:4])
		// Skip |size|. Only bump offset after a successful read down below.
		isDeps := size&0x80000000 != 0
		size = size & ^uint32(0x80000000)
		data = data[4:]
		if len(data) < int(size) {
			err = fmt.Errorf("premature end of file after %d bytes", int(offset)+len(data)+4)
			break
		}
		if size%4 != 0 || size < 8 || size > maxRecordSize {
			// It'd be nice to do a check for "size < 12" instead. The likelihood of
			// a path with 3 characters or less is very small.
			err = fmt.Errorf("record size %d is out of bounds", size)
			break
		}
		if isDeps {
			if size < 12 {
				err = errors.New("record size is too small for deps")
				break
			}
			outID := int32(binary.LittleEndian.Uint32(data[:4]))
			if outID < 0 || outID >= 0x1000000 {
				// That's a lot of nodes.
				err = errors.New("record deps id is out of bounds")
				break
			}
			mtime := TimeStamp(binary.LittleEndian.Uint64(data[4:12]))
			depsCount := int(size-12) / 4

			// TODO(maruel): Redesign to reduce bound checks.
			deps := NewDeps(mtime, depsCount)
			x := 12
			for i := 0; i < depsCount; i++ {
				v := binary.LittleEndian.Uint32(data[x : x+4])
				if int(v) >= len(d.Nodes) || d.Nodes[v] == nil {
					err = errors.New("record deps node id is out of bounds")
					break
				}
				deps.Nodes[i] = d.Nodes[v]
				x += 4
			}

			totalDepRecordCount++
			if !d.updateDeps(outID, deps) {
				uniqueDepRecordCount++
			}
		} else {
			pathSize := size - 4
			// There can be up to 3 bytes of padding.
			if data[pathSize-1] == '\x00' {
				pathSize--
				if data[pathSize-1] == '\x00' {
					pathSize--
					if data[pathSize-1] == '\x00' {
						pathSize--
					}
				}
			}

			// TODO(maruel): We need to differentiate if we are using the GC or not.
			// When the GC is disabled, #YOLO, the buffer will never go away anyway
			// so better to leverage it!
			subpath := unsafeString(data[:pathSize])
			// Here we make a copy, because we do not want to keep a reference to the
			// read buffer.
			// subpath := string(data[:pathSize])

			// It is not necessary to pass in a correct slashBits here. It will
			// either be a Node that's in the manifest (in which case it will already
			// have a correct slashBits that GetNode will look up), or it is an
			// implicit dependency from a .d which does not affect the build command
			// (and so need not have its slashes maintained).
			node := state.GetNode(subpath, 0)

			// Check that the expected index matches the actual index. This can only
			// happen if two ninja processes write to the same deps log concurrently.
			// (This uses unary complement to make the checksum look less like a
			// dependency record entry.)
			checksum := binary.LittleEndian.Uint32(data[size-4 : size])
			expectedID := ^checksum
			id := int32(len(d.Nodes))
			if id != int32(expectedID) {
				err = errors.New("node id checksum is invalid")
				break
			}
			if node.ID >= 0 {
				err = errors.New("node is duplicate")
				break
			}
			node.ID = id
			d.Nodes = append(d.Nodes, node)
		}
		// Register the successful read.
		data = data[size:]
		offset += int64(size) + 4
	}

	if err != nil {
		// An error occurred while loading; try to recover by truncating the
		// file to the last fully-read record.
		if err2 := os.Truncate(path, offset); err2 != nil {
			return LoadError, fmt.Errorf("truncating failed while parsing error %q: %w", err, err2)
		}

		// The truncate succeeded; we'll just report the load error as a
		// warning because the build can proceed.
		err = errors.New(err.Error() + "; recovering")
		return LoadSuccess, err
	}

	// Rebuild the log if there are too many dead records.
	const minCompactionEntryCount = 1000
	kCompactionRatio := 3
	if totalDepRecordCount > minCompactionEntryCount && totalDepRecordCount > uniqueDepRecordCount*kCompactionRatio {
		d.needsRecompaction = true
	}
	return LoadSuccess, nil
}

// GetDeps returns the Deps for this node ID.
//
// Silently ignore invalid node ID.
func (d *DepsLog) GetDeps(node *Node) *Deps {
	// Abort if the node has no id (never referenced in the deps) or if
	// there's no deps recorded for the node.
	if node.ID < 0 || int(node.ID) >= len(d.Deps) {
		return nil
	}
	return d.Deps[node.ID]
}

// GetFirstReverseDepsNode returns something?
//
// TODO(maruel): Understand better.
func (d *DepsLog) GetFirstReverseDepsNode(node *Node) *Node {
	for id := 0; id < len(d.Deps); id++ {
		deps := d.Deps[id]
		if deps == nil {
			continue
		}
		for _, n := range deps.Nodes {
			if n == node {
				return d.Nodes[id]
			}
		}
	}
	return nil
}

// Recompact rewrites the known log entries, throwing away old data.
func (d *DepsLog) Recompact(path string) error {
	defer metricRecord(".ninja_deps recompact")()

	if err := d.Close(); err != nil {
		return err
	}
	tempPath := path + ".recompact"

	// OpenForWrite() opens for append.  Make sure it's not appending to a
	// left-over file from a previous recompaction attempt that crashed somehow.
	if err := os.Remove(tempPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Create a new temporary log to regenerate everything.
	newLog := DepsLog{}
	if err := newLog.OpenForWrite(tempPath); err != nil {
		return err
	}

	// Clear all known ids so that new ones can be reassigned.  The new indices
	// will refer to the ordering in newLog, not in the current log.
	for _, i := range d.Nodes {
		i.ID = -1
	}

	// Write out all deps again.
	for oldID := 0; oldID < len(d.Deps); oldID++ {
		deps := d.Deps[oldID]
		if deps == nil { // If nodes[oldID] is a leaf, it has no deps.
			continue
		}

		if !d.IsDepsEntryLiveFor(d.Nodes[oldID]) {
			continue
		}

		if err := newLog.recordDeps(d.Nodes[oldID], deps.MTime, deps.Nodes); err != nil {
			_ = newLog.Close()
			return err
		}
	}

	if err := newLog.Close(); err != nil {
		return err
	}

	// All nodes now have ids that refer to newLog, so steal its data.
	d.Deps = newLog.Deps
	d.Nodes = newLog.Nodes

	if err := os.Remove(path); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

// IsDepsEntryLiveFor returns if the deps entry for a node is still reachable
// from the manifest.
//
// The deps log can contain deps entries for files that were built in the
// past but are no longer part of the manifest.  This function returns if
// this is the case for a given node.  This function is slow, don't call
// it from code that runs on every build.
func (d *DepsLog) IsDepsEntryLiveFor(node *Node) bool {
	// Skip entries that don't have in-edges or whose edges don't have a
	// "deps" attribute. They were in the deps log from previous builds, but
	// the the files they were for were removed from the build and their deps
	// entries are no longer needed.
	// (Without the check for "deps", a chain of two or more nodes that each
	// had deps wouldn't be collected in a single recompaction.)
	return node.InEdge != nil && node.InEdge.GetBinding("deps") != ""
}

// Updates the in-memory representation.  Takes ownership of |deps|.
// Returns true if a prior deps record was deleted.
func (d *DepsLog) updateDeps(outID int32, deps *Deps) bool {
	if n := int(outID) + 1 - len(d.Deps); n > 0 {
		d.Deps = append(d.Deps, make([]*Deps, n)...)
	}
	existed := d.Deps[outID] != nil
	d.Deps[outID] = deps
	return existed
}

var zeroBytes [4]byte

// Write a node name record, assigning it an id.
func (d *DepsLog) recordID(node *Node) error {
	if node.Path == "" {
		return errors.New("node.Path is empty")
	}
	pathSize := len(node.Path)
	padding := (4 - pathSize%4) % 4 // Pad path to 4 byte boundary.

	size := uint32(pathSize + padding + 4)
	if size > maxRecordSize {
		return errors.New("node.Path is too long")
	}
	if err := d.openForWriteIfNeeded(); err != nil {
		return nil
	}
	if err := binary.Write(d.buf, binary.LittleEndian, size); err != nil {
		return nil
	}
	if _, err := d.buf.WriteString(node.Path); err != nil {
		return nil
	}
	if padding != 0 {
		if _, err := d.buf.Write(zeroBytes[:padding]); err != nil {
			return nil
		}
	}
	id := int32(len(d.Nodes))
	checksum := ^uint32(id)
	if err := binary.Write(d.buf, binary.LittleEndian, checksum); err != nil {
		return nil
	}
	if err := d.buf.Flush(); err != nil {
		return nil
	}
	node.ID = id
	d.Nodes = append(d.Nodes, node)
	return nil
}

// openForWriteIfNeeded should be called before using file.
func (d *DepsLog) openForWriteIfNeeded() error {
	if d.filePath == "" {
		return nil
	}
	if d.file != nil {
		panic("surprising state")
	}
	var err error
	d.file, err = os.OpenFile(d.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return err
	}
	// Set the buffer size large and flush the file buffer after every record to
	// make sure records aren't written partially.
	d.buf = bufio.NewWriterSize(d.file, maxRecordSize+1)

	// Opening a file in append mode doesn't set the file pointer to the file's
	// end on Windows. Do that explicitly.
	offset, err := d.file.Seek(0, os.SEEK_END)
	if err != nil {
		return err
	}

	if offset == 0 {
		if _, err = d.buf.WriteString(depsLogFileSignature); err != nil {
			return err
		}
		if err = binary.Write(d.buf, binary.LittleEndian, depsLogCurrentVersion); err != nil {
			return err
		}
	}
	if err = d.buf.Flush(); err != nil {
		return err
	}
	d.filePath = ""
	return nil
}
