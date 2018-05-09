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

// MacAddrAllocatorType data struct
type MacAddrAllocatorType struct {
	MacAddrID uint32
}

func (m *MacAddrAllocatorType) String() string {
	str := fmt.Sprintf("MacAddr: %d", m.MacAddrID)
	return str
}

// formatMacAddress returns a formatted mac string
func formatMacAddress(id32 uint32) string {

	// uint32 is 4Billion interfaces ... lets not worry about it

	var macOctets [6]uint64
	macInstanceID := uint64(id32)

	macOctets[0] = 0x02
	macOctets[1] = 0xFF & (macInstanceID >> (8 * 4))
	macOctets[2] = 0xFF & (macInstanceID >> (8 * 3))
	macOctets[3] = 0xFF & (macInstanceID >> (8 * 2))
	macOctets[4] = 0xFF & (macInstanceID >> (8 * 1))
	macOctets[5] = 0xFF & (macInstanceID >> (8 * 0))

	macOctetString := ""
	for i := 0; i < 5; i++ {
		macOctetString += fmt.Sprintf("%02X:", macOctets[i])
	}
	macOctetString += fmt.Sprintf("%02X", macOctets[5])

	return macOctetString
}

// Allocate allocate the next MacAddr
func (m *MacAddrAllocatorType) Allocate() (string, uint32) {

	m.MacAddrID++

	return formatMacAddress(m.MacAddrID), m.MacAddrID
}

// NewMacAddrAllocator allocates a MacAddr id's for memory interfaces
func NewMacAddrAllocator() *MacAddrAllocatorType {

	MacAddrAllocator := &MacAddrAllocatorType{
		MacAddrID: 0,
	}

	return MacAddrAllocator
}
