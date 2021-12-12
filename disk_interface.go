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


// Interface for reading files from disk.  See DiskInterface for details.
// This base offers the minimum interface needed just to read files.
type FileReader struct {
  virtual ~FileReader() {}

  // Result of ReadFile.
  enum Status {
    Okay,
    NotFound,
    OtherError
  }

}

// Interface for accessing the disk.
//
// Abstract so it can be mocked out for tests.  The real implementation
// is RealDiskInterface.
type DiskInterface struct {

}

// Implementation of DiskInterface that actually hits the disk.
type RealDiskInterface struct {
  RealDiskInterface()
                      : use_cache_(false)
                      {}
  virtual ~RealDiskInterface() {}

  // Whether stat information can be cached.
  bool use_cache_

  typedef map<string, TimeStamp> DirCache
  // TODO: Neither a map nor a hashmap seems ideal here.  If the statcache
  // works out, come up with a better data structure.
  typedef map<string, DirCache> Cache
  mutable Cache cache_
}


func DirName(path string) string {
  static const char kPathSeparators[] = "\\/"
  static const char kPathSeparators[] = "/"
  static string const kEnd = kPathSeparators + sizeof(kPathSeparators) - 1

  string::size_type slash_pos = path.find_last_of(kPathSeparators)
  if slash_pos == string::npos {
  while (slash_pos > 0 && find(kPathSeparators, kEnd, path[slash_pos - 1]) != kEnd)
  }
    --slash_pos
  return path.substr(0, slash_pos)
}

func MakeDir(path string) int {
  return mkdir(path, 0777)
}

func TimeStampFromFileTime(filetime *FILETIME) TimeStamp {
  // FILETIME is in 100-nanosecond increments since the Windows epoch.
  // We don't much care about epoch correctness but we do want the
  // resulting value to fit in a 64-bit integer.
  uint64_t mtime = ((uint64_t)filetime.dwHighDateTime << 32) |
    ((uint64_t)filetime.dwLowDateTime)
  // 1600 epoch -> 2000 epoch (subtract 400 years).
  return (TimeStamp)mtime - 12622770400LL * (1000000000LL / 100)
}

func StatSingleFile(path string, err *string) TimeStamp {
  WIN32_FILE_ATTRIBUTE_DATA attrs
  if !GetFileAttributesExA(path, GetFileExInfoStandard, &attrs) {
    win_err := GetLastError()
    if win_err == ERROR_FILE_NOT_FOUND || win_err == ERROR_PATH_NOT_FOUND {
      return 0
    }
    *err = "GetFileAttributesEx(" + path + "): " + GetLastErrorString()
    return -1
  }
  return TimeStampFromFileTime(attrs.ftLastWriteTime)
}

func IsWindows7OrLater() bool {
  OSVERSIONINFOEX version_info =
      { sizeof(OSVERSIONINFOEX), 6, 1, 0, 0, {0}, 0, 0, 0, 0, 0}
  comparison := 0
  VER_SET_CONDITION(comparison, VER_MAJORVERSION, VER_GREATER_EQUAL)
  VER_SET_CONDITION(comparison, VER_MINORVERSION, VER_GREATER_EQUAL)
  return VerifyVersionInfo( &version_info, VER_MAJORVERSION | VER_MINORVERSION, comparison)
}

func StatAllFilesInDir(dir string, stamps *map<string, TimeStamp>, err *string) bool {
  // FindExInfoBasic is 30% faster than FindExInfoStandard.
  static bool can_use_basic_info = IsWindows7OrLater()
  // This is not in earlier SDKs.
  const FINDEX_INFO_LEVELS kFindExInfoBasic =
      static_cast<FINDEX_INFO_LEVELS>(1)
  FINDEX_INFO_LEVELS level =
      can_use_basic_info ? kFindExInfoBasic : FindExInfoStandard
  WIN32_FIND_DATAA ffd
  find_handle := FindFirstFileExA((dir + "\\*"), level, &ffd, FindExSearchNameMatch, nil, 0)

  if find_handle == INVALID_HANDLE_VALUE {
    win_err := GetLastError()
    if win_err == ERROR_FILE_NOT_FOUND || win_err == ERROR_PATH_NOT_FOUND {
      return true
    }
    *err = "FindFirstFileExA(" + dir + "): " + GetLastErrorString()
    return false
  }
  do {
    lowername := ffd.cFileName
    if lowername == ".." {
      // Seems to just copy the timestamp for ".." from ".", which is wrong.
      // This is the case at least on NTFS under Windows 7.
      continue
    }
    transform(lowername.begin(), lowername.end(), lowername.begin(), ::tolower)
    stamps.insert(make_pair(lowername, TimeStampFromFileTime(ffd.ftLastWriteTime)))
  } while (FindNextFileA(find_handle, &ffd))
  FindClose(find_handle)
  return true
}

// DiskInterface ---------------------------------------------------------------

// Create all the parent directories for path; like mkdir -p
// `basename path`.
func (d *DiskInterface) MakeDirs(path string) bool {
  dir := DirName(path)
  if len(dir) == 0 {
    return true  // Reached root; assume it's there.
  }
  string err
  mtime := Stat(dir, &err)
  if mtime < 0 {
    Error("%s", err)
    return false
  }
  if mtime > 0 {
    return true  // Exists already; we're done.
  }

  // Directory doesn't exist.  Try creating its parent first.
  success := MakeDirs(dir)
  if success == nil {
    return false
  }
}

// RealDiskInterface -----------------------------------------------------------

func (r *RealDiskInterface) Stat(path string, err *string) TimeStamp {
  METRIC_RECORD("node stat")
  // MSDN: "Naming Files, Paths, and Namespaces"
  // http://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx
  if !path.empty() && path[0] != '\\' && path.size() > MAX_PATH {
    ostringstream err_stream
    err_stream << "Stat(" << path << "): Filename longer than " << MAX_PATH
               << " characters"
    *err = err_stream.str()
    return -1
  }
  if !use_cache_ {

  }
  dir := DirName(path)
  string base(path.substr(dir.size() ? dir.size() + 1 : 0))
  if base == ".." {
    // StatAllFilesInDir does not report any information for base = "..".
    base = "."
    dir = path
  }

  transform(dir.begin(), dir.end(), dir.begin(), ::tolower)
  transform(base.begin(), base.end(), base.begin(), ::tolower)

  ci := cache_.find(dir)
  if ci == cache_.end() {
    ci = cache_.insert(make_pair(dir, DirCache())).first
    if !StatAllFilesInDir(dir.empty() ? "." : dir, &ci.second, err) {
      cache_.erase(ci)
      return -1
    }
  }
  di := ci.second.find(base)
  return di != ci.second.end() ? di.second : 0
  struct stat st
  if stat(path, &st) < 0 {
    if errno == ENOENT || errno == ENOTDIR {
      return 0
    }
    *err = "stat(" + path + "): " + strerror(errno)
    return -1
  }
  // Some users (Flatpak) set mtime to 0, this should be harmless
  // and avoids conflicting with our return value of 0 meaning
  // that it doesn't exist.
  if st.st_mtime == 0 {
    return 1
  }
  return (int64_t)st.st_mtime * 1000000000LL + st.st_mtime_n
  return ((int64_t)st.st_mtimespec.tv_sec * 1000000000LL + st.st_mtimespec.tv_nsec)
  return (int64_t)st.st_mtim.tv_sec * 1000000000LL + st.st_mtim.tv_nsec
  return (int64_t)st.st_mtime * 1000000000LL + st.st_mtimensec
}

func (r *RealDiskInterface) WriteFile(path string, contents string) bool {
  fp := fopen(path, "w")
  if fp == nil {
    Error("WriteFile(%s): Unable to create file. %s", path, strerror(errno))
    return false
  }

  if fwrite(contents.data(), 1, contents.length(), fp) < contents.length() {
    Error("WriteFile(%s): Unable to write to the file. %s", path, strerror(errno))
    fclose(fp)
    return false
  }

  if fclose(fp) == EOF {
    Error("WriteFile(%s): Unable to close the file. %s", path, strerror(errno))
    return false
  }

  return true
}

func (r *RealDiskInterface) MakeDir(path string) bool {
  if ::MakeDir(path) < 0 {
    if errno == EEXIST {
      return true
    }
    Error("mkdir(%s): %s", path, strerror(errno))
    return false
  }
  return true
}

FileReader::Status RealDiskInterface::ReadFile(string path, string* contents, string* err) {
  switch (::ReadFile(path, contents, err)) {
  case 0:       return Okay
  case -ENOENT: return NotFound
  default:      return OtherError
  }
}

func (r *RealDiskInterface) RemoveFile(path string) int {
  attributes := GetFileAttributes(path)
  if attributes == INVALID_FILE_ATTRIBUTES {
    win_err := GetLastError()
    if win_err == ERROR_FILE_NOT_FOUND || win_err == ERROR_PATH_NOT_FOUND {
      return 1
    }
  } else if attributes & FILE_ATTRIBUTE_READONLY {
    // On non-Windows systems, remove() will happily delete read-only files.
    // On Windows Ninja should behave the same:
    //   https://github.com/ninja-build/ninja/issues/1886
    // Skip error checking.  If this fails, accept whatever happens below.
    SetFileAttributes(path, attributes & ~FILE_ATTRIBUTE_READONLY)
  }
  if attributes & FILE_ATTRIBUTE_DIRECTORY {
    // remove() deletes both files and directories. On Windows we have to 
    // select the correct function (DeleteFile will yield Permission Denied when
    // used on a directory)
    // This fixes the behavior of ninja -t clean in some cases
    // https://github.com/ninja-build/ninja/issues/828
    if !RemoveDirectory(path) {
      win_err := GetLastError()
      if win_err == ERROR_FILE_NOT_FOUND || win_err == ERROR_PATH_NOT_FOUND {
        return 1
      }
      // Report remove(), not RemoveDirectory(), for cross-platform consistency.
      Error("remove(%s): %s", path, GetLastErrorString())
      return -1
    }
  } else {
    if !DeleteFile(path) {
      win_err := GetLastError()
      if win_err == ERROR_FILE_NOT_FOUND || win_err == ERROR_PATH_NOT_FOUND {
        return 1
      }
      // Report as remove(), not DeleteFile(), for cross-platform consistency.
      Error("remove(%s): %s", path, GetLastErrorString())
      return -1
    }
  }
  if remove(path) < 0 {
    switch (errno) {
      case ENOENT:
        return 1
      default:
        Error("remove(%s): %s", path, strerror(errno))
        return -1
    }
  }
  return 0
}

// Whether stat information can be cached.  Only has an effect on Windows.
func (r *RealDiskInterface) AllowStatCache(allow bool) {
  use_cache_ = allow
  if !use_cache_ {
    cache_ = nil
  }
}
