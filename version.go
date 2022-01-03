// Copyright 2013 Google Inc. All Rights Reserved.
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
	"fmt"
	"log"
	"strconv"
	"strings"
)

// The version number of the current Ninja release.  This will always
// be "git" on trunk.
//
// TODO(maruel): Figure out our versioning convention.
const NinjaVersion = "1.10.2.git"

// Parse the major/minor components of a version string.
func ParseVersion(version string) (int, int) {
	end := strings.Index(version, ".")
	if end == -1 {
		end = len(version)
	}
	major, _ := strconv.Atoi(keepNumbers(version[:end]))
	minor := 0
	if end != len(version) {
		start := end + 1
		end = strings.Index(version[start:], ".")
		if end == -1 {
			end = len(version)
		} else {
			end += start
		}
		minor, _ = strconv.Atoi(keepNumbers(version[start:end]))
	}
	return major, minor
}

func keepNumbers(s string) string {
	i := strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' })
	if i != -1 {
		return s[:i]
	}
	return s
}

// checkNinjaVersion checks whether a version is compatible with the current
// Ninja version, returns an error if not.
func checkNinjaVersion(version string) error {
	binMajor, binMinor := ParseVersion(NinjaVersion)
	fileMajor, fileMinor := ParseVersion(version)
	if binMajor > fileMajor {
		log.Printf("ninja executable version (%s) greater than build file ninja_required_version (%s); versions may be incompatible.", NinjaVersion, version)
	} else if (binMajor == fileMajor && binMinor < fileMinor) || binMajor < fileMajor {
		return fmt.Errorf("ninja version (%s) incompatible with build file ninja_required_version version (%s)", NinjaVersion, version)
	}
	return nil
}
