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

package ginja

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
)

// As build commands run they can output extra dependency information
// (e.g. header dependencies for C source) dynamically.  DepsLog collects
// that information at build time and uses it for subsequent builds.
//
// The on-disk format is based on two primary design constraints:
// - it must be written to as a stream (during the build, which may be
//   interrupted);
// - it can be read all at once on startup.  (Alternative designs, where
//   it contains indexing information, were considered and discarded as
//   too complicated to implement; if the file is small than reading it
//   fully on startup is acceptable.)
// Here are some stats from the Windows Chrome dependency files, to
// help guide the design space.  The total text in the files sums to
// 90mb so some compression is warranted to keep load-time fast.
// There's about 10k files worth of dependencies that reference about
// 40k total paths totalling 2mb of unique strings.
//
// Based on these stats, here's the current design.
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
	needs_recompaction_ bool
	file_               *os.File
	file_path_          string

	// Maps id -> Node.
	nodes_ []*Node
	// Maps id -> deps of that id.
	deps_ []*Deps
}

func NewDepsLog() DepsLog {
	return DepsLog{}
}

// Reading (startup-time) interface.
type Deps struct {
	mtime      TimeStamp
	node_count int
	nodes      []*Node
}

func NewDeps(mtime TimeStamp, node_count int) *Deps {
	return &Deps{
		mtime:      mtime,
		node_count: node_count,
		nodes:      make([]*Node, node_count),
	}
}

// Used for tests.
func (d *DepsLog) nodes() []*Node {
	return d.nodes_
}
func (d *DepsLog) deps() []*Deps {
	return d.deps_
}

// The version is stored as 4 bytes after the signature and also serves as a
// byte order mark. Signature and version combined are 16 bytes long.
var DepsLogFileSignature = []byte("# ninjadeps\n")

const DepsLogCurrentVersion = uint32(4)

// Record size is currently limited to less than the full 32 bit, due to
// internal buffers having to have this size.
const kMaxRecordSize = (1 << 19) - 1

// Writing (build-time) interface.
func (d *DepsLog) OpenForWrite(path string, err *string) bool {
	if d.needs_recompaction_ {
		if !d.Recompact(path, err) {
			return false
		}
	}

	if d.file_ != nil {
		panic("M-A")
	}
	d.file_path_ = path // we don't actually open the file right now, but will do
	// so on the first write attempt
	return true
}

func (d *DepsLog) RecordDeps(node *Node, mtime TimeStamp, nodes []*Node) bool {
	node_count := len(nodes)
	// Track whether there's any new data to be recorded.
	made_change := false

	// Assign ids to all nodes that are missing one.
	if node.id() < 0 {
		if !d.RecordId(node) {
			return false
		}
		made_change = true
	}
	for i := 0; i < node_count; i++ {
		if nodes[i].id() < 0 {
			if !d.RecordId(nodes[i]) {
				return false
			}
			made_change = true
		}
	}

	// See if the new data is different than the existing data, if any.
	if !made_change {
		deps := d.GetDeps(node)
		if deps == nil || deps.mtime != mtime || deps.node_count != node_count {
			made_change = true
		} else {
			for i := 0; i < node_count; i++ {
				if deps.nodes[i] != nodes[i] {
					made_change = true
					break
				}
			}
		}
	}

	// Don't write anything if there's no new info.
	if !made_change {
		return true
	}

	// Update on-disk representation.
	size := uint32(4 * (1 + 2 + node_count))
	if size > kMaxRecordSize {
		//errno = ERANGE
		return false
	}

	if !d.OpenForWriteIfNeeded() {
		return false
	}
	size |= 0x80000000 // Deps record: set high bit.

	if err := binary.Write(d.file_, binary.LittleEndian, size); err != nil {
		return false
	}
	if err := binary.Write(d.file_, binary.LittleEndian, uint32(node.id())); err != nil {
		return false
	}
	if err := binary.Write(d.file_, binary.LittleEndian, mtime); err != nil {
		return false
	}
	for i := 0; i < node_count; i++ {
		if err := binary.Write(d.file_, binary.LittleEndian, uint32(nodes[i].id())); err != nil {
			return false
		}
	}
	/* TODO: Evaluate the performance impact.
	if err := d.file_.Sync(); err != nil {
		return false
	}
	*/

	// Update in-memory representation.
	deps := NewDeps(mtime, node_count)
	for i := 0; i < node_count; i++ {
		deps.nodes[i] = nodes[i]
	}
	d.UpdateDeps(node.id(), deps)

	return true
}

func (d *DepsLog) Close() {
	d.OpenForWriteIfNeeded() // create the file even if nothing has been recorded
	if d.file_ != nil {
		d.file_.Close()
	}
	d.file_ = nil
}

func (d *DepsLog) Load(path string, state *State, err *string) LoadStatus {
	defer METRIC_RECORD(".ninja_deps load")()
	buf := [kMaxRecordSize + 1]byte{}
	f, err2 := os.Open(path)
	if f == nil {
		if os.IsNotExist(err2) {
			return LOAD_NOT_FOUND
		}
		*err = err2.Error()
		return LOAD_ERROR
	}

	// TODO(maruel): Read the file all at once then use a buffer.
	valid_header := true
	version := uint32(0)
	if _, err := f.Read(buf[:len(DepsLogFileSignature)]); err != nil {
		valid_header = false
	} else if err := binary.Read(f, binary.LittleEndian, &version); err != nil {
		valid_header = false
	}
	// Note: For version differences, this should migrate to the new format.
	// But the v1 format could sometimes (rarely) end up with invalid data, so
	// don't migrate v1 to v3 to force a rebuild. (v2 only existed for a few days,
	// and there was no release with it, so pretend that it never happened.)
	if !valid_header || !bytes.Equal(buf[:len(DepsLogFileSignature)], DepsLogFileSignature) || version != DepsLogCurrentVersion {
		if version == 1 {
			*err = "deps log version change; rebuilding"
		} else {
			*err = "bad deps log signature or version; starting over"
		}
		f.Close()
		os.Remove(path)
		// Don't report this as a failure.  An empty deps log will cause
		// us to rebuild the outputs anyway.
		return LOAD_SUCCESS
	}
	read_failed := false
	unique_dep_record_count := 0
	total_dep_record_count := 0
	for {
		var size uint32
		if err2 := binary.Read(f, binary.LittleEndian, &size); err2 != nil {
			if err2 != io.EOF {
				read_failed = true
				*err = err2.Error()
			}
			break
		}
		is_deps := size&0x80000000 != 0
		size = size & 0x7FFFFFFF
		//log.Printf("is_deps=%t; size=%d", is_deps, size)
		if size > kMaxRecordSize {
			read_failed = true
			panic("size")
			break
		}
		if _, err2 := f.Read(buf[:size]); err2 != nil {
			read_failed = true
			*err = err2.Error()
			break
		}

		if is_deps {
			if size%4 != 0 || size < 12 {
				panic("M-A")
			}
			// TODO(maruel): Not super efficient but looking for correctness now.
			// Optimize later.
			r := bytes.NewReader(buf[:size])
			out_id := uint32(0)
			if err2 := binary.Read(r, binary.LittleEndian, &out_id); err2 != nil {
				panic(err2)
			}
			var mtime TimeStamp
			if err2 := binary.Read(r, binary.LittleEndian, &mtime); err2 != nil {
				panic(err2)
			}
			deps_count := int(size)/4 - 3

			deps := NewDeps(mtime, deps_count)
			for i := 0; i < deps_count; i++ {
				v := uint32(0)
				if err2 := binary.Read(r, binary.LittleEndian, &v); err2 != nil {
					panic(err2)
				}
				if int(v) >= len(d.nodes_) {
					panic("M-A")
				}
				if d.nodes_[v] == nil {
					panic("M-A")
				}
				deps.nodes[i] = d.nodes_[v]
			}

			total_dep_record_count++
			if !d.UpdateDeps(int(out_id), deps) {
				unique_dep_record_count++
			}
		} else {
			path_size := size - 4
			if path_size <= 0 {
				panic("M-A")
			} // CanonicalizePath() rejects empty paths.
			// There can be up to 3 bytes of padding.
			if buf[path_size-1] == '\x00' {
				path_size--
			}
			if buf[path_size-1] == '\x00' {
				path_size--
			}
			if buf[path_size-1] == '\x00' {
				path_size--
			}
			subpath := string(buf[:path_size])
			// It is not necessary to pass in a correct slash_bits here. It will
			// either be a Node that's in the manifest (in which case it will already
			// have a correct slash_bits that GetNode will look up), or it is an
			// implicit dependency from a .d which does not affect the build command
			// (and so need not have its slashes maintained).
			node := state.GetNode(subpath, 0)

			// Check that the expected index matches the actual index. This can only
			// happen if two ninja processes write to the same deps log concurrently.
			// (This uses unary complement to make the checksum look less like a
			// dependency record entry.)
			checksum := uint32(0) // *reinterpret_cast<unsigned*>(buf + size - 4)
			if err2 := binary.Read(bytes.NewReader(buf[size-4:]), binary.LittleEndian, &checksum); err2 != nil {
				panic(err2)
			}
			expected_id := ^checksum
			id := len(d.nodes_)
			if id != int(expected_id) {
				read_failed = true
				panic(expected_id)
				break
			}

			if node.id() >= 0 {
				panic("M-A")
			}
			node.set_id(id)
			d.nodes_ = append(d.nodes_, node)
		}
	}

	if read_failed {
		// An error occurred while loading; try to recover by truncating the
		// file to the last fully-read record.
		if *err == "" {
			*err = "premature end of file"
		}

		if err := f.Truncate(0); err != nil {
			f.Close()
			return LOAD_ERROR
		}
		f.Close()

		// The truncate succeeded; we'll just report the load error as a
		// warning because the build can proceed.
		*err += "; recovering"
		return LOAD_SUCCESS
	}
	f.Close()

	// Rebuild the log if there are too many dead records.
	kMinCompactionEntryCount := 1000
	kCompactionRatio := 3
	if total_dep_record_count > kMinCompactionEntryCount && total_dep_record_count > unique_dep_record_count*kCompactionRatio {
		d.needs_recompaction_ = true
	}
	return LOAD_SUCCESS
}

func (d *DepsLog) GetDeps(node *Node) *Deps {
	// Abort if the node has no id (never referenced in the deps) or if
	// there's no deps recorded for the node.
	if node.id() < 0 || node.id() >= len(d.deps_) {
		return nil
	}
	return d.deps_[node.id()]
}

func (d *DepsLog) GetFirstReverseDepsNode(node *Node) *Node {
	for id := 0; id < len(d.deps_); id++ {
		deps := d.deps_[id]
		if deps == nil {
			continue
		}
		for i := 0; i < deps.node_count; i++ {
			if deps.nodes[i] == node {
				return d.nodes_[id]
			}
		}
	}
	return nil
}

// Rewrite the known log entries, throwing away old data.
func (d *DepsLog) Recompact(path string, err *string) bool {
	defer METRIC_RECORD(".ninja_deps recompact")()

	d.Close()
	temp_path := path + ".recompact"

	// OpenForWrite() opens for append.  Make sure it's not appending to a
	// left-over file from a previous recompaction attempt that crashed somehow.
	if err2 := os.Remove(temp_path); err2 != nil && !os.IsNotExist(err2) {
		*err = err2.Error()
		return false
	}

	new_log := NewDepsLog()
	if !new_log.OpenForWrite(temp_path, err) {
		return false
	}

	// Clear all known ids so that new ones can be reassigned.  The new indices
	// will refer to the ordering in new_log, not in the current log.
	for _, i := range d.nodes_ {
		i.set_id(-1)
	}

	// Write out all deps again.
	for old_id := 0; old_id < len(d.deps_); old_id++ {
		deps := d.deps_[old_id]
		if deps == nil { // If nodes_[old_id] is a leaf, it has no deps.
			continue
		}

		if !d.IsDepsEntryLiveFor(d.nodes_[old_id]) {
			continue
		}

		if !new_log.RecordDeps(d.nodes_[old_id], deps.mtime, deps.nodes) {
			new_log.Close()
			return false
		}
	}

	new_log.Close()

	// All nodes now have ids that refer to new_log, so steal its data.
	d.deps_ = new_log.deps_
	d.nodes_ = new_log.nodes_

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

// Returns if the deps entry for a node is still reachable from the manifest.
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
	return node.in_edge() != nil && node.in_edge().GetBinding("deps") != ""
}

// Updates the in-memory representation.  Takes ownership of |deps|.
// Returns true if a prior deps record was deleted.
func (d *DepsLog) UpdateDeps(out_id int, deps *Deps) bool {
	if out_id >= len(d.deps_) {
		d.deps_ = append(d.deps_, make([]*Deps, out_id+1)...)
	}
	existed := d.deps_[out_id] != nil
	d.deps_[out_id] = deps
	return existed
}

// Write a node name record, assigning it an id.
func (d *DepsLog) RecordId(node *Node) bool {
	path_size := len(node.path())
	padding := (4 - path_size%4) % 4 // Pad path to 4 byte boundary.

	size := uint32(path_size + padding + 4)
	if size > kMaxRecordSize {
		panic(1)
		//errno = ERANGE
		return false
	}

	if !d.OpenForWriteIfNeeded() {
		panic(1)
		return false
	}
	if err := binary.Write(d.file_, binary.LittleEndian, size); err != nil {
		panic(1)
		return false
	}
	if _, err := d.file_.Write([]byte(node.path())); err != nil {
		if node.path() == "" {
			panic("M-A")
		}
		panic(1)
		return false
	}
	if padding != 0 {
		if _, err := d.file_.Write(make([]byte, padding)); err != nil {
			panic(1)
			return false
		}
	}
	id := len(d.nodes_)
	checksum := ^uint32(id)
	if err := binary.Write(d.file_, binary.LittleEndian, checksum); err != nil {
		panic(1)
		return false
	}
	/* TODO: Evaluate the performance impact.
	if err := d.file_.Sync(); err != nil {
		return false
	}
	*/
	node.set_id(id)
	d.nodes_ = append(d.nodes_, node)

	return true
}

// Should be called before using file_. When false is returned, errno will
// be set.
func (d *DepsLog) OpenForWriteIfNeeded() bool {
	if d.file_path_ == "" {
		return true
	}
	d.file_, _ = os.OpenFile(d.file_path_, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if d.file_ == nil {
		panic(1)
		return false
	}
	/* TODO
	// Set the buffer size to this and flush the file buffer after every record
	// to make sure records aren't written partially.
	if setvbuf(d.file_, nil, _IOFBF, kMaxRecordSize+1) != 0 {
		return false
	}
	SetCloseOnExec(fileno(d.file_))
	*/

	// Opening a file in append mode doesn't set the file pointer to the file's
	// end on Windows. Do that explicitly.
	offset, err := d.file_.Seek(0, os.SEEK_END)
	if err != nil {
		panic(1)
		return false
	}

	if offset == 0 {
		if _, err := d.file_.Write(DepsLogFileSignature); err != nil {
			panic(err)
			return false
		}
		if err := binary.Write(d.file_, binary.LittleEndian, DepsLogCurrentVersion); err != nil {
			panic(err)
			return false
		}
	}
	if err := d.file_.Sync(); err != nil {
		return false
	}
	d.file_path_ = ""
	return true
}
