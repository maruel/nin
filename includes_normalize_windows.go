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

func InternalGetFullPathName(file_name string, buffer *string, err *string) bool {
	*buffer = file_name
	panic("TODO")
	/*
		result_size := syscall.GetFullPathNameA(file_name, buffer_length, buffer, nil)
		if result_size == 0 {
			*err = "GetFullPathNameA(" + file_name + "): " + GetLastErrorString()
			return false
		} else if result_size > buffer_length {
			*err = "path too long"
			return false
		}
		return true
	*/
}
