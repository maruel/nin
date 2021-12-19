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

package ginja

import (
	"io/ioutil"
	"os"
	"path/filepath"
)

// Interface for reading files from disk.  See DiskInterface for details.
// This base offers the minimum interface needed just to read files.
type FileReader interface {
	// Read and store in given string.  On success, return Okay.
	// On error, return another Status and fill |err|.
	ReadFile(path string, contents *string, err *string) Status
}

// Result of ReadFile.
type Status int

const (
	Okay Status = iota
	NotFound
	OtherError
)

// Interface for accessing the disk.
//
// Abstract so it can be mocked out for tests.  The real implementation
// is RealDiskInterface.
type DiskInterface interface {
	// stat() a file, returning the mtime, or 0 if missing and -1 on
	// other errors.
	Stat(path string, err *string) TimeStamp

	// Create a directory, returning false on failure.
	MakeDir(path string) bool

	// Create a file, with the specified name and contents
	// Returns true on success, false on failure
	WriteFile(path, contents string) bool

	// Remove the file named @a path. It behaves like 'rm -f path' so no errors
	// are reported if it does not exists.
	// @returns 0 if the file has been removed,
	//          1 if the file does not exist, and
	//          -1 if an error occurs.
	RemoveFile(path string) int
}

// Implementation of DiskInterface that actually hits the disk.
type RealDiskInterface struct {

	// Whether stat information can be cached.
	use_cache_ bool

	// TODO: Neither a map nor a hashmap seems ideal here.  If the statcache
	// works out, come up with a better data structure.
	cache_ Cache
}

func NewRealDiskInterface() RealDiskInterface {
	return RealDiskInterface{}
}

type DirCache map[string]TimeStamp
type Cache map[string]DirCache

func DirName(path string) string {
	return filepath.Dir(path)
	/*
		kPathSeparators := "\\/"
		kEnd := kPathSeparators + len(kPathSeparators) - 1

		slash_pos := path.find_last_of(kPathSeparators)
		if slash_pos == -1 {
			return "" // Nothing to do.
		}
		for slash_pos > 0 && find(kPathSeparators, kEnd, path[slash_pos-1]) != kEnd {
			slash_pos--
		}
		return path[0:slash_pos]
	*/
}

func MakeDir(path string) int {
	//return _mkdir(path)
	if err := os.Mkdir(path, 0o755); err != nil {
		return 1
	}
	return 0
}

/*
func TimeStampFromFileTime(filetime *FILETIME) TimeStamp {
	// FILETIME is in 100-nanosecond increments since the Windows epoch.
	// We don't much care about epoch correctness but we do want the
	// resulting value to fit in a 64-bit integer.
	mtime := (filetime.dwHighDateTime << 32) | filetime.dwLowDateTime
	// 1600 epoch -> 2000 epoch (subtract 400 years).
	return mtime - 12622770400*(1000000000/100)
}
*/

func StatSingleFile(path string, err *string) TimeStamp {
	/*
		var attrs WIN32_FILE_ATTRIBUTE_DATA
		if !GetFileAttributesExA(path, GetFileExInfoStandard, &attrs) {
			win_err := GetLastError()
			if win_err == ERROR_FILE_NOT_FOUND || win_err == ERROR_PATH_NOT_FOUND {
				return 0
			}
			*err = "GetFileAttributesEx(" + path + "): " + GetLastErrorString()
			return -1
		}
		return TimeStampFromFileTime(attrs.ftLastWriteTime)
	*/

	// This will obviously have to be optimized.
	s, err2 := os.Stat(path)
	if err2 != nil {
		*err = err2.Error()
		if os.IsNotExist(err2) {
			return 0
		}
		return -1
	}
	return TimeStamp(s.ModTime().UnixMicro())
}

/*
func IsWindows7OrLater() bool {
	version_info := OSVERSIONINFOEX{sizeof(OSVERSIONINFOEX), 6, 1, 0, 0, {0}, 0, 0, 0, 0, 0}
	comparison := 0
	VER_SET_CONDITION(comparison, VER_MAJORVERSION, VER_GREATER_EQUAL)
	VER_SET_CONDITION(comparison, VER_MINORVERSION, VER_GREATER_EQUAL)
	return VerifyVersionInfo(&version_info, VER_MAJORVERSION|VER_MINORVERSION, comparison)
}
*/

func StatAllFilesInDir(dir string, stamps map[string]TimeStamp, err *string) bool {
	/*
		// FindExInfoBasic is 30% faster than FindExInfoStandard.
		//can_use_basic_info := IsWindows7OrLater()
		// This is not in earlier SDKs.
		//FINDEX_INFO_LEVELS
		kFindExInfoBasic := 1
		//FINDEX_INFO_LEVELS
		level := kFindExInfoBasic
		// FindExInfoStandard
		var ffd WIN32_FIND_DATAA
		find_handle := FindFirstFileExA((dir + "\\*"), level, &ffd, FindExSearchNameMatch, nil, 0)

		if find_handle == INVALID_HANDLE_VALUE {
			win_err := GetLastError()
			if win_err == ERROR_FILE_NOT_FOUND || win_err == ERROR_PATH_NOT_FOUND {
				return true
			}
			*err = "FindFirstFileExA(" + dir + "): " + GetLastErrorString()
			return false
		}
		for {
			lowername := ffd.cFileName
			if lowername == ".." {
				// Seems to just copy the timestamp for ".." from ".", which is wrong.
				// This is the case at least on NTFS under Windows 7.
				continue
			}
			lowername = strings.ToLower(lowername)
			stamps[lowername] = TimeStampFromFileTime(ffd.ftLastWriteTime)
			if !FindNextFileA(find_handle, &ffd) {
				break
			}
		}
		FindClose(find_handle)
		return true
	*/
	f, err2 := os.Open(dir)
	if err2 != nil {
		*err = err2.Error()
		return false
	}
	defer f.Close()
	d, err2 := f.Readdir(0)
	if err2 != nil {
		*err = err2.Error()
		return false
	}
	for _, i := range d {
		if !i.IsDir() {
			stamps[i.Name()] = TimeStamp(i.ModTime().UnixMicro())
		}
	}
	return true
}

/*
// Create all the parent directories for path; like mkdir -p
// `basename path`.
func MakeDirs(d DiskInterface, path string) bool {
	dir := DirName(path)
	if len(dir) == 0 {
		return true // Reached root; assume it's there.
	}
	err := ""
	mtime := d.Stat(dir, &err)
	if mtime < 0 {
		Error("%s", err)
		return false
	}
	if mtime > 0 {
		return true // Exists already; we're done.
	}

	// Directory doesn't exist.  Try creating its parent first.
	success := d.MakeDirs(dir)
	if success == nil {
		return false
	}
	return MakeDir(dir)
}

func (r *RealDiskInterface) Stat(path string, err *string) TimeStamp {
	METRIC_RECORD("node stat")
	// MSDN: "Naming Files, Paths, and Namespaces"
	// http://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx
	if !path.empty() && path[0] != '\\' && path.size() > MAX_PATH {
		var err_stream ostringstream
		//err_stream << "Stat(" << path << "): Filename longer than " << MAX_PATH << " characters"
		*err = err_stream.str()
		return -1
	}
	if !r.use_cache_ {
		return StatSingleFile(path, err)
	}

	dir := DirName(path)
	o := 0
	if dir.size() != 0 {
		o = dir.size() + 1
	}
	base := path[o:]
	if base == ".." {
		// StatAllFilesInDir does not report any information for base = "..".
		base = "."
		dir = path
	}

	dir = strings.ToLower(dir)
	base = strings.ToLower(base)

	ci := r.cache_.find(dir)
	if ci == r.cache_.end() {
		ci = r.cache_.insert(make_pair(dir, DirCache())).first
		s := "."
		if !dir.empty() {
			s = dir
		}
		if !StatAllFilesInDir(s, &ci.second, err) {
			r.cache_.erase(ci)
			return -1
		}
	}
	di := ci.second.find(base)
	if di != ci.second.end() {
		return di.second
	}
	return 0
}
*/

func (r *RealDiskInterface) WriteFile(path string, contents string) bool {
	return ioutil.WriteFile(path, []byte(contents), 0o755) == nil
}

func (r *RealDiskInterface) MakeDir(path string) bool {
	/*
		if MakeDir(path) < 0 {
			if errno == EEXIST {
				return true
			}
			Error("mkdir(%s): %s", path, strerror(errno))
			return false
		}
		return true
	*/
	err := os.Mkdir(path, 0o755)
	return err == nil || os.IsExist(err)
}

func (r *RealDiskInterface) ReadFile(path string, contents *string, err *string) Status {
	c, err2 := ioutil.ReadFile(path)
	if err2 == nil {
		*contents = string(c)
		return Okay
	}
	*err = err2.Error()
	if os.IsNotExist(err2) {
		return NotFound
	}
	return OtherError
}

/*
func (r *RealDiskInterface) RemoveFile(path string) int {
	attributes := GetFileAttributes(path)
	if attributes == INVALID_FILE_ATTRIBUTES {
		win_err := GetLastError()
		if win_err == ERROR_FILE_NOT_FOUND || win_err == ERROR_PATH_NOT_FOUND {
			return 1
		}
	} else if (attributes & FILE_ATTRIBUTE_READONLY) != 0 {
		// On non-Windows systems, remove() will happily delete read-only files.
		// On Windows Ninja should behave the same:
		//   https://github.com/ninja-build/ninja/issues/1886
		// Skip error checking.  If this fails, accept whatever happens below.
		SetFileAttributes(path, attributes & ^FILE_ATTRIBUTE_READONLY)
	}
	if (attributes & FILE_ATTRIBUTE_DIRECTORY) != 0 {
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
	return 0
}
*/

// Whether stat information can be cached.  Only has an effect on Windows.
func (r *RealDiskInterface) AllowStatCache(allow bool) {
	r.use_cache_ = allow
	if !r.use_cache_ {
		r.cache_ = nil
	}
}
