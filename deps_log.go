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

//go:build nobuild

package ginja


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
  DepsLog() : needs_recompaction_(false), file_(nil) {}
  ~DepsLog()

  // Reading (startup-time) interface.

  // Used for tests.
  const vector<Node*>& nodes() const { return nodes_; }
  const vector<Deps*>& deps() const { return deps_; }

  needs_recompaction_ bool
  file_ *FILE
  file_path_ string

  // Maps id -> Node.
  nodes_ []*Node
  // Maps id -> deps of that id.
  deps_ []*Deps

  DepsLogTest friend struct
}
type Deps struct {
type Deps struct {
  Deps(int64_t mtime, int node_count)
      : mtime(mtime), node_count(node_count), nodes(new Node*[node_count]) {}
  ~Deps() { delete [] nodes; }
  mtime TimeStamp
  node_count int
  nodes *Node*
  }


typedef __int32 int32_t
typedef unsigned __int32 uint32_t

// The version is stored as 4 bytes after the signature and also serves as a
// byte order mark. Signature and version combined are 16 bytes long.
const char kFileSignature[] = "# ninjadeps\n"
const int kCurrentVersion = 4

// Record size is currently limited to less than the full 32 bit, due to
// internal buffers having to have this size.
const unsigned kMaxRecordSize = (1 << 19) - 1

DepsLog::~DepsLog() {
  Close()
}

// Writing (build-time) interface.
func (d *DepsLog) OpenForWrite(path string, err *string) bool {
  if needs_recompaction_ {
    if !Recompact(path, err) {
      return false
    }
  }

  if !!file_ { panic("oops") }
  file_path_ = path  // we don't actually open the file right now, but will do
                      // so on the first write attempt
  return true
}

func (d *DepsLog) RecordDeps(node *Node, mtime TimeStamp, nodes *vector<Node*>) bool {
  return RecordDeps(node, mtime, nodes.size(), nodes.empty() ? nil : (Node**)&nodes.front())
}

func (d *DepsLog) RecordDeps(node *Node, mtime TimeStamp, node_count int, nodes *Node*) bool {
  // Track whether there's any new data to be recorded.
  made_change := false

  // Assign ids to all nodes that are missing one.
  if node.id() < 0 {
    if !RecordId(node) {
      return false
    }
    made_change = true
  }
  for i := 0; i < node_count; i++ {
    if nodes[i].id() < 0 {
      if !RecordId(nodes[i]) {
        return false
      }
      made_change = true
    }
  }

  // See if the new data is different than the existing data, if any.
  if !made_change {
    deps := GetDeps(node)
    if !deps || deps.mtime != mtime || deps.node_count != node_count {
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
  unsigned size = 4 * (1 + 2 + node_count)
  if size > kMaxRecordSize {
    errno = ERANGE
    return false
  }

  if !OpenForWriteIfNeeded() {
    return false
  }
  size |= 0x80000000  // Deps record: set high bit.
  if fwrite(&size, 4, 1, file_) < 1 {
    return false
  }
  id := node.id()
  if fwrite(&id, 4, 1, file_) < 1 {
    return false
  }
  mtime_part := static_cast<uint32_t>(mtime & 0xffffffff)
  if fwrite(&mtime_part, 4, 1, file_) < 1 {
    return false
  }
  mtime_part = static_cast<uint32_t>((mtime >> 32) & 0xffffffff)
  if fwrite(&mtime_part, 4, 1, file_) < 1 {
    return false
  }
  for i := 0; i < node_count; i++ {
    id = nodes[i].id()
    if fwrite(&id, 4, 1, file_) < 1 {
      return false
    }
  }
  if fflush(file_) != 0 {
    return false
  }

  // Update in-memory representation.
  deps := new Deps(mtime, node_count)
  for i := 0; i < node_count; i++ {
    deps.nodes[i] = nodes[i]
  }
  UpdateDeps(node.id(), deps)

  return true
}

func (d *DepsLog) Close() {
  OpenForWriteIfNeeded()  // create the file even if nothing has been recorded
  if file_ {
    fclose(file_)
  }
  file_ = nil
}

func (d *DepsLog) Load(path string, state *State, err *string) LoadStatus {
  METRIC_RECORD(".ninja_deps load")
  char buf[kMaxRecordSize + 1]
  FILE* f = fopen(path, "rb")
  if f == nil {
    if errno == ENOENT {
      return LOAD_NOT_FOUND
    }
    *err = strerror(errno)
    return LOAD_ERROR
  }

  valid_header := true
  version := 0
  if !fgets(buf, sizeof(buf), f) || fread(&version, 4, 1, f) < 1 {
    valid_header = false
  }
  // Note: For version differences, this should migrate to the new format.
  // But the v1 format could sometimes (rarely) end up with invalid data, so
  // don't migrate v1 to v3 to force a rebuild. (v2 only existed for a few days,
  // and there was no release with it, so pretend that it never happened.)
  if !valid_header || strcmp(buf, kFileSignature) != 0 || version != kCurrentVersion {
    if version == 1 {
      *err = "deps log version change; rebuilding"
    } else {
      *err = "bad deps log signature or version; starting over"
    }
    fclose(f)
    unlink(path)
    // Don't report this as a failure.  An empty deps log will cause
    // us to rebuild the outputs anyway.
    return LOAD_SUCCESS
  }

  var offset int32
  read_failed := false
  unique_dep_record_count := 0
  total_dep_record_count := 0
  for ; ;  {
    offset = ftell(f)

    var size uint32
    if fread(&size, 4, 1, f) < 1 {
      if !feof(f) {
        read_failed = true
      }
      break
    }
    bool is_deps = (size >> 31) != 0
    size = size & 0x7FFFFFFF

    if size > kMaxRecordSize || fread(buf, size, 1, f) < 1 {
      read_failed = true
      break
    }

    if is_deps {
      if !size % 4 == 0 { panic("oops") }
      deps_data := reinterpret_cast<int*>(buf)
      out_id := deps_data[0]
      var mtime TimeStamp
      mtime = (TimeStamp)(((uint64_t)(unsigned int)deps_data[2] << 32) | (uint64_t)(unsigned int)deps_data[1])
      deps_data += 3
      int deps_count = (size / 4) - 3

      deps := new Deps(mtime, deps_count)
      for i := 0; i < deps_count; i++ {
        if !deps_data[i] < (int)nodes_.size() { panic("oops") }
        if !nodes_[deps_data[i]] { panic("oops") }
        deps.nodes[i] = nodes_[deps_data[i]]
      }

      total_dep_record_count++
      if !UpdateDeps(out_id, deps) {
        unique_dep_record_count++
      }
    } else {
      int path_size = size - 4
      if !path_size > 0 { panic("oops") }  // CanonicalizePath() rejects empty paths.
      // There can be up to 3 bytes of padding.
      if buf[path_size - 1] == '\0' {
      	path_size--
      }
      if buf[path_size - 1] == '\0' {
      	path_size--
      }
      if buf[path_size - 1] == '\0' {
      	path_size--
      }
      string subpath(buf, path_size)
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
      unsigned checksum = *reinterpret_cast<unsigned*>(buf + size - 4)
      int expected_id = ~checksum
      id := nodes_.size()
      if id != expected_id {
        read_failed = true
        break
      }

      if !node.id() < 0 { panic("oops") }
      node.set_id(id)
      nodes_.push_back(node)
    }
  }

  if read_failed {
    // An error occurred while loading; try to recover by truncating the
    // file to the last fully-read record.
    if ferror(f) {
      *err = strerror(ferror(f))
    } else {
      *err = "premature end of file"
    }
    fclose(f)

    if !Truncate(path, offset, err) {
      return LOAD_ERROR
    }

    // The truncate succeeded; we'll just report the load error as a
    // warning because the build can proceed.
    *err += "; recovering"
    return LOAD_SUCCESS
  }

  fclose(f)

  // Rebuild the log if there are too many dead records.
  kMinCompactionEntryCount := 1000
  kCompactionRatio := 3
  if total_dep_record_count > kMinCompactionEntryCount && total_dep_record_count > unique_dep_record_count * kCompactionRatio {
    needs_recompaction_ = true
  }

  return LOAD_SUCCESS
}

func (d *DepsLog) GetDeps(node *Node) *DepsLog::Deps {
  // Abort if the node has no id (never referenced in the deps) or if
  // there's no deps recorded for the node.
  if node.id() < 0 || node.id() >= (int)deps_.size() {
    return nil
  }
  return deps_[node.id()]
}

func (d *DepsLog) GetFirstReverseDepsNode(node *Node) *Node {
  for id := 0; id < deps_.size(); id++ {
    deps := deps_[id]
    if deps == nil {
      continue
    }
    for i := 0; i < deps.node_count; i++ {
      if deps.nodes[i] == node {
        return nodes_[id]
      }
    }
  }
  return nil
}

// Rewrite the known log entries, throwing away old data.
func (d *DepsLog) Recompact(path string, err *string) bool {
  METRIC_RECORD(".ninja_deps recompact")

  Close()
  string temp_path = path + ".recompact"

  // OpenForWrite() opens for append.  Make sure it's not appending to a
  // left-over file from a previous recompaction attempt that crashed somehow.
  unlink(temp_path)

  var new_log DepsLog
  if !new_log.OpenForWrite(temp_path, err) {
    return false
  }

  // Clear all known ids so that new ones can be reassigned.  The new indices
  // will refer to the ordering in new_log, not in the current log.
  for i := nodes_.begin(); i != nodes_.end(); i++ {
    (*i).set_id(-1)
  }

  // Write out all deps again.
  for old_id := 0; old_id < (int)deps_.size(); old_id++ {
    deps := deps_[old_id]
    if deps == nil {  // If nodes_[old_id] is a leaf, it has no deps.
    	continue
    }

    if !IsDepsEntryLiveFor(nodes_[old_id]) {
      continue
    }

    if !new_log.RecordDeps(nodes_[old_id], deps.mtime, deps.node_count, deps.nodes) {
      new_log.Close()
      return false
    }
  }

  new_log.Close()

  // All nodes now have ids that refer to new_log, so steal its data.
  deps_.swap(new_log.deps_)
  nodes_.swap(new_log.nodes_)

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
  return node.in_edge() && !node.in_edge().GetBinding("deps").empty()
}

// Updates the in-memory representation.  Takes ownership of |deps|.
// Returns true if a prior deps record was deleted.
func (d *DepsLog) UpdateDeps(out_id int, deps *Deps) bool {
  if out_id >= (int)deps_.size() {
    deps_.resize(out_id + 1)
  }

  bool delete_old = deps_[out_id] != nil
  if delete_old {
    delete deps_[out_id]
  }
  deps_[out_id] = deps
  return delete_old
}

// Write a node name record, assigning it an id.
func (d *DepsLog) RecordId(node *Node) bool {
  path_size := node.path().size()
  int padding = (4 - path_size % 4) % 4  // Pad path to 4 byte boundary.

  unsigned size = path_size + padding + 4
  if size > kMaxRecordSize {
    errno = ERANGE
    return false
  }

  if !OpenForWriteIfNeeded() {
    return false
  }
  if fwrite(&size, 4, 1, file_) < 1 {
    return false
  }
  if fwrite(node.path().data(), path_size, 1, file_) < 1 {
    if !!node.path().empty() { panic("oops") }
    return false
  }
  if padding && fwrite("\0\0", padding, 1, file_) < 1 {
    return false
  }
  id := nodes_.size()
  unsigned checksum = ~(unsigned)id
  if fwrite(&checksum, 4, 1, file_) < 1 {
    return false
  }
  if fflush(file_) != 0 {
    return false
  }

  node.set_id(id)
  nodes_.push_back(node)

  return true
}

// Should be called before using file_. When false is returned, errno will
// be set.
func (d *DepsLog) OpenForWriteIfNeeded() bool {
  if file_path_.empty() {
    return true
  }
  file_ = fopen(file_path_, "ab")
  if !file_ {
    return false
  }
  // Set the buffer size to this and flush the file buffer after every record
  // to make sure records aren't written partially.
  if setvbuf(file_, nil, _IOFBF, kMaxRecordSize + 1) != 0 {
    return false
  }
  SetCloseOnExec(fileno(file_))

  // Opening a file in append mode doesn't set the file pointer to the file's
  // end on Windows. Do that explicitly.
  fseek(file_, 0, SEEK_END)

  if ftell(file_) == 0 {
    if fwrite(kFileSignature, sizeof(kFileSignature) - 1, 1, file_) < 1 {
      return false
    }
    if fwrite(&kCurrentVersion, 4, 1, file_) < 1 {
      return false
    }
  }
  if fflush(file_) != 0 {
    return false
  }
  file_path_ = nil
  return true
}

