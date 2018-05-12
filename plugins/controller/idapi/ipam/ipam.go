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

package ipam

import (
	"fmt"
	"net"
)

// PoolAllocatorType data struct
type PoolAllocatorType struct {
	Name          string
	StartRange    uint32
	EndRange      uint32
	NextInRange   uint32
	TotalInRange  uint32
	Network       string // example form 10.5.3.0/24
	ipNetwork     *net.IPNet
	ipNetworku32  uint32
	ipMasku32     uint32
	numBitsInMask int
	Allocated     map[uint32]struct{}
}

func (p *PoolAllocatorType) String() string {
	str := fmt.Sprintf("IPAM Pool: %s, network: %s, addr's [%d-%d], numAllocated: %d",
		p.Name,
		p.Network,
		p.StartRange,
		p.EndRange,
		len(p.Allocated))
	return str
}

func (p *PoolAllocatorType) GetAllocatedAddressesStatus() string {

	return fmt.Sprintf("%d of %d addresses are set, %d are free",
		len(p.Allocated), p.TotalInRange, p.TotalInRange-uint32(len(p.Allocated)))
}

// IsAddressSet returns the address string if set
func (p *PoolAllocatorType) IsAddressSet(addrID uint32) (string, error) {
	if addrID < p.StartRange || addrID > p.EndRange {
		return "", fmt.Errorf("IsSetAddress: addr_id '%d' out of range '%d-%d",
			addrID, p.StartRange, p.EndRange)
	}

	if _, exists := p.Allocated[addrID]; exists {
		return p.formatIPAddress(addrID), nil
	}
	return "", fmt.Errorf("IsSetAddress: addr_id '%d' not found in range '%d-%d",
		addrID, p.StartRange, p.EndRange)
}

// SetAddress sets the address in the range as allocated
func (p *PoolAllocatorType) SetAddress(addrID uint32) (string, error) {
	if addrID < p.StartRange || addrID > p.EndRange {
		return "", fmt.Errorf("SetAddress: addr_id '%d' out of range '%d-%d",
			addrID, p.StartRange, p.EndRange)
	}

	p.Allocated[addrID] = struct{}{}

	return p.formatIPAddress(addrID), nil
}

// AllocateIPAddress allocates a free ip address from the pool
func (p *PoolAllocatorType) AllocateIPAddress() (string, uint32, error) {

	if uint32(len(p.Allocated)) == p.TotalInRange {
		return "", 0, fmt.Errorf("AllocateIPAddress: %s has no more addresses", p.Name)
	}
	found := false
	for addrID := p.NextInRange; addrID <= p.EndRange; addrID++ {
		if _, exists := p.Allocated[addrID]; !exists {
			p.Allocated[addrID] = struct{}{}
			p.NextInRange = addrID + 1
			if p.NextInRange > p.EndRange {
				p.NextInRange = p.StartRange
			}
			return p.formatIPAddress(addrID), addrID, nil
		}
	}
	if !found {
		for addrID := p.StartRange; addrID < p.NextInRange; addrID++ {
			if _, exists := p.Allocated[addrID]; !exists {
				p.Allocated[addrID] = struct{}{}
				p.NextInRange = addrID + 1
				if p.NextInRange > p.EndRange {
					p.NextInRange = p.StartRange
				}
				return p.formatIPAddress(addrID), addrID, nil
			}
		}
	}
	return "", 0, fmt.Errorf("AllocateIPAddress: %s internal error", p.Name)
}

// formatIPAddress uses the ip network and ipNUm to contruct a string
func (p *PoolAllocatorType) formatIPAddress(ipID uint32) string {

	ipAddru32 := p.ipNetworku32 | ipID
	ipAddrStr := fmt.Sprintf("%d.%d.%d.%d/%d",
		(ipAddru32>>24)&0xFF,
		(ipAddru32>>16)&0xFF,
		(ipAddru32>>8)&0xFF,
		(ipAddru32)&0xFF,
		p.numBitsInMask)

	//fmt.Println("setIpIDInSubnet: ", ipAddrStr)

	return ipAddrStr
}

// SetIPAddrIfInsidePool set this address as allocated if is contained in the pool
func (p *PoolAllocatorType) SetIPAddrIfInsidePool(ipAddressStr string) {

	// see if this address falls within the subnet, and if it does, set the addr in the bitmap
	ipAddress := net.ParseIP(ipAddressStr)
	if ipAddress != nil && p.ipNetwork.Contains(ipAddress) {

		//fmt.Println("setIpAddrIfInsideSubnet: ", ipAddress)
		ip := ipAddress.To4()
		ipAddressu32 := uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
		ipID := ipAddressu32 &^ p.ipMasku32
		//fmt.Println("setIpAddrIfInsideSubnet: ipID", ipID)
		p.Allocated[ipID] = struct{}{}
	}
}

// NewIPAMPoolAllocator allocates a ipam allocator
func NewIPAMPoolAllocator(
	name string,
	startRange uint32,
	endRange uint32,
	networkStr string) *PoolAllocatorType {

	var nextInRange uint32

	ip, n, err := net.ParseCIDR(networkStr)
	if err != nil {
		return nil
	}

	numBitsInMask, _ := n.Mask.Size()

	// example: /24 is 8 bits so need 2**8 for the bitmap
	if startRange == 0 {
		startRange = 1 // start at 1 not zero
		nextInRange = 1
	} else {
		nextInRange = startRange

	}
	if endRange == 0 {
		endRange = (1 << uint32(32-numBitsInMask)) - 2 // example: 8 bits: 256 - 1 - 1 = 254
	}

	totalInRange := endRange - startRange + 1

	if len(ip.To4()) == net.IPv4len {

		poolAllocator := &PoolAllocatorType{
			Name:          name,
			StartRange:    startRange,
			EndRange:      endRange,
			NextInRange:   nextInRange,
			TotalInRange:  totalInRange,
			Network:       networkStr,
			ipNetworku32:  uint32(n.IP[0])<<24 | uint32(n.IP[1])<<16 | uint32(n.IP[2])<<8 | uint32(n.IP[3]),
			ipMasku32:     uint32(n.Mask[0])<<24 | uint32(n.Mask[1])<<16 | uint32(n.Mask[2])<<8 | uint32(n.Mask[3]),
			numBitsInMask: numBitsInMask,
			ipNetwork:     n,
			Allocated:     make(map[uint32]struct{}),
		}
		fmt.Printf("NewIPAMPoolAllocator: %v\n", poolAllocator)

		return poolAllocator
	}

	return nil
}
