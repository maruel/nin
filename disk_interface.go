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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// Interface for reading files from disk.  See DiskInterface for details.
// This base offers the minimum interface needed just to read files.
type FileReader interface {
	// ReadFile reads a file and returns its content.
	//
	// If the content is not empty, it appends a zero byte at the end of the
	// slice.
	ReadFile(path string) ([]byte, error)
}

// Interface for accessing the disk.
//
// Abstract so it can be mocked out for tests.  The real implementation
// is RealDiskInterface.
type DiskInterface interface {
	FileReader
	// stat() a file, returning the mtime, or 0 if missing and -1 on
	// other errors.
	Stat(path string) (TimeStamp, error)

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

type dirCache map[string]TimeStamp
type cache map[string]dirCache

func dirName(path string) string {
	return filepath.Dir(path)
	/*
		pathSeparators := "\\/"
		end := pathSeparators + len(pathSeparators) - 1

		slashPos := path.findLastOf(pathSeparators)
		if slashPos == -1 {
			return "" // Nothing to do.
		}
		for slashPos > 0 && find(pathSeparators, end, path[slashPos-1]) != end {
			slashPos--
		}
		return path[0:slashPos]
	*/
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

func statSingleFile(path string) (TimeStamp, error) {
	s, err := os.Stat(path)
	if err != nil {
		// See TestDiskInterfaceTest_StatMissingFile for rationale for ENOTDIR
		// check.
		if os.IsNotExist(err) || errors.Unwrap(err) == syscall.ENOTDIR {
			return 0, nil
		}
		return -1, err
	}
	return TimeStamp(s.ModTime().UnixMicro()), nil
}

/*
func IsWindows7OrLater() bool {
	versionInfo := OSVERSIONINFOEX{sizeof(OSVERSIONINFOEX), 6, 1, 0, 0, {0}, 0, 0, 0, 0, 0}
	comparison := 0
	VER_SET_CONDITION(comparison, VER_MAJORVERSION, VER_GREATER_EQUAL)
	VER_SET_CONDITION(comparison, VER_MINORVERSION, VER_GREATER_EQUAL)
	return VerifyVersionInfo(&versionInfo, VER_MAJORVERSION|VER_MINORVERSION, comparison)
}
*/

func statAllFilesInDir(dir string, stamps map[string]TimeStamp) error {
	/*
		// FindExInfoBasic is 30% faster than FindExInfoStandard.
		//canUseBasicInfo := IsWindows7OrLater()
		// This is not in earlier SDKs.
		//FINDEX_INFO_LEVELS
		findExInfoBasic := 1
		//FINDEX_INFO_LEVELS
		level := findExInfoBasic
		// FindExInfoStandard
		var ffd WIN32_FIND_DATAA
		findHandle := FindFirstFileExA((dir + "\\*"), level, &ffd, FindExSearchNameMatch, nil, 0)

		if findHandle == INVALID_HANDLE_VALUE {
			winErr := GetLastError()
			if winErr == ERROR_FILE_NOT_FOUND || winErr == ERROR_PATH_NOT_FOUND {
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
			if !FindNextFileA(findHandle, &ffd) {
				break
			}
		}
		FindClose(findHandle)
		return true
	*/
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	d, err := f.Readdir(0)
	if err != nil {
		_ = f.Close()
		return err
	}
	for _, i := range d {
		if !i.IsDir() {
			stamps[i.Name()] = TimeStamp(i.ModTime().UnixMicro())
		}
	}
	return f.Close()
}

// Create all the parent directories for path; like mkdir -p
// `basename path`.
func MakeDirs(d DiskInterface, path string) bool {
	dir := dirName(path)
	if dir == path || dir == "." || dir == "" {
		return true // Reached root; assume it's there.
	}
	mtime, err := d.Stat(dir)
	if mtime < 0 {
		errorf("%s", err)
		return false
	}
	if mtime > 0 {
		return true // Exists already; we're done.
	}

	// Directory doesn't exist.  Try creating its parent first.
	if !MakeDirs(d, dir) {
		return false
	}
	return d.MakeDir(dir)
}

//

// Implementation of DiskInterface that actually hits the disk.
type RealDiskInterface struct {
	// Whether stat information can be cached.
	useCache bool

	// TODO: Neither a map nor a hashmap seems ideal here.  If the statcache
	// works out, come up with a better data structure.
	cache cache
}

func NewRealDiskInterface() RealDiskInterface {
	return RealDiskInterface{}
}

// MSDN: "Naming Files, Paths, and Namespaces"
// http://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx
const maxPath = 260

func (r *RealDiskInterface) Stat(path string) (TimeStamp, error) {
	defer metricRecord("node stat")()
	if runtime.GOOS == "windows" {
		if path != "" && path[0] != '\\' && len(path) >= maxPath {
			return -1, fmt.Errorf("Stat(%s): Filename longer than %d characters", path, maxPath)
		}
		if !r.useCache {
			return statSingleFile(path)
		}

		dir := dirName(path)
		o := 0
		if dir != "" {
			o = len(dir) + 1
		}
		base := path[o:]
		if base == ".." {
			// statAllFilesInDir does not report any information for base = "..".
			base = "."
			dir = path
		}

		dir = strings.ToLower(dir)
		base = strings.ToLower(base)

		ci, ok := r.cache[dir]
		if !ok {
			ci = dirCache{}
			r.cache[dir] = ci
			s := "."
			if dir != "" {
				s = dir
			}
			if err := statAllFilesInDir(s, ci); err != nil {
				delete(r.cache, dir)
				return -1, err
			}
		}
		return ci[base], nil
	}
	return statSingleFile(path)
}

func (r *RealDiskInterface) WriteFile(path string, contents string) bool {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return false
	}
	_, err = f.WriteString(contents)
	if err1 := f.Close(); err == nil {
		err = err1
	}
	return err == nil
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
	err := os.Mkdir(path, 0o777)
	return err == nil || os.IsExist(err)
}

func (r *RealDiskInterface) ReadFile(path string) ([]byte, error) {
	c, err := ioutil.ReadFile(path)
	if err == nil {
		if len(c) != 0 {
			// ioutil.ReadFile() is guaranteed to have an extra byte in the slice,
			// (ab)use it.
			c = c[:len(c)+1]
		}
		return c, nil
	}
	return nil, err
}

func (r *RealDiskInterface) RemoveFile(path string) int {
	if err := os.Remove(path); err != nil {
		// TODO: return -1?
		return 1
	}
	return 0
	/*
		attributes := GetFileAttributes(path)
		if attributes == INVALID_FILE_ATTRIBUTES {
			winErr := GetLastError()
			if winErr == ERROR_FILE_NOT_FOUND || winErr == ERROR_PATH_NOT_FOUND {
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
				winErr := GetLastError()
				if winErr == ERROR_FILE_NOT_FOUND || winErr == ERROR_PATH_NOT_FOUND {
					return 1
				}
				// Report remove(), not RemoveDirectory(), for cross-platform consistency.
				Error("remove(%s): %s", path, GetLastErrorString())
				return -1
			}
		} else {
			if !DeleteFile(path) {
				winErr := GetLastError()
				if winErr == ERROR_FILE_NOT_FOUND || winErr == ERROR_PATH_NOT_FOUND {
					return 1
				}
				// Report as remove(), not DeleteFile(), for cross-platform consistency.
				Error("remove(%s): %s", path, GetLastErrorString())
				return -1
			}
		}
		return 0
	*/
}

// Whether stat information can be cached.  Only has an effect on Windows.
func (r *RealDiskInterface) AllowStatCache(allow bool) {
	if runtime.GOOS == "windows" {
		r.useCache = allow
		if !r.useCache {
			r.cache = nil
		} else if r.cache == nil {
			r.cache = cache{}
		}
	}
}
