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
	// Stat stat()'s a file, returning the mtime, or 0 if missing and -1 on
	// other errors.
	Stat(path string) (TimeStamp, error)

	// MakeDir creates a directory, returning false on failure.
	MakeDir(path string) error

	// WriteFile creates a file, with the specified name and contents
	WriteFile(path, contents string) error

	// RemoveFile removes the file named path.
	//
	// It should return an error that matches os.IsNotExist() if the file was not
	// present.
	RemoveFile(path string) error
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

func statAllFilesInDir(dir string, stamps map[string]TimeStamp) error {
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
func MakeDirs(d DiskInterface, path string) error {
	dir := dirName(path)
	if dir == path || dir == "." || dir == "" {
		return nil // Reached root; assume it's there.
	}
	mtime, err := d.Stat(dir)
	if mtime < 0 {
		return err
	}
	if mtime > 0 {
		return nil // Exists already; we're done.
	}

	// Directory doesn't exist.  Try creating its parent first.
	if err := MakeDirs(d, dir); err != nil {
		return err
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

func (r *RealDiskInterface) WriteFile(path string, contents string) error {
	return ioutil.WriteFile(path, unsafeByteSlice(contents), 0o666)
}

func (r *RealDiskInterface) MakeDir(path string) error {
	return os.Mkdir(path, 0o777)
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

func (r *RealDiskInterface) RemoveFile(path string) error {
	return os.Remove(path)
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
