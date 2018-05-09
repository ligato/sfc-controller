// Copyright (c) 2018 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package idapi

import (
	"fmt"
)

// MemifAllocatorType data struct
type MemifAllocatorType struct {
	MemifID uint32
}

func (m *MemifAllocatorType) String() string {
	str := fmt.Sprintf("memif: %d", m.MemifID)
	return str
}

// Allocate allocate the next memif id
func (m *MemifAllocatorType) Allocate() uint32 {

	m.MemifID++

	return m.MemifID
}

// NewMemifAllocator allocates a memif id's for memory interfaces
func NewMemifAllocator() *MemifAllocatorType {

	MemifAllocator := &MemifAllocatorType{
		MemifID: 0,
	}

	return MemifAllocator
}
