// Copyright (c) 2017 Cisco and/or its affiliates.
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
	name       string
	StartRange uint32
	EndRange   uint32
	Network    string
	ipv4Pool   *Ipv4Pool
	//ipv6Pool *Ipv6Pool
}

func (p *PoolAllocatorType) String() string {
	str := fmt.Sprintf("IPAM Pool %s: addrs's: [%d-%d], %s",
		p.name,
		p.StartRange,
		p.EndRange,
		p.ipv4Pool.String())
	return str
}

// SetAddress sets the address in the range as allocated
func (p *PoolAllocatorType) SetAddress(addrID uint32) error {
	if addrID < p.StartRange || addrID > p.EndRange {
		return fmt.Errorf("SetAddress: addr_id '%d' out of range '%d-%d",
			addrID, p.StartRange, p.EndRange)
	}
	_, err := p.ipv4Pool.SetIPInPool(addrID - p.StartRange + 1)
	if err != nil {
		return err
	}
	return nil
}

// AllocateIPAddress allocates a free ip address from the pool
func (p *PoolAllocatorType) AllocateIPAddress() (string, uint32, error) {
	if p.ipv4Pool != nil {
		return p.ipv4Pool.AllocateFromPool()
	}
	return "", 0, fmt.Errorf("AllocateIPAddress: %s is not ipv4 ... v6 not supported",
		p.name)
}

// NewIPAMPoolAllocator allocates a ipam allocator
func NewIPAMPoolAllocator(
	name string,
	startRange uint32,
	endRange uint32,
	networkStr string) *PoolAllocatorType {

	ip, network, err := net.ParseCIDR(networkStr)
	if err != nil {
		return nil
	}
	fmt.Printf("NewIPAMPoolAllocator: ip: %s, len(ip):%d, net: %s \n", ip, len(ip), network)
	fmt.Printf("NewIPAMPoolAllocator: len(ipTo4): %d\n", len(ip.To4()))
	if len(ip.To4()) == net.IPv4len {
		poolAllocator := &PoolAllocatorType{
			name:       name,
			StartRange: startRange,
			EndRange:   endRange,
			Network:    networkStr,
			ipv4Pool:   NewIpv4Pool(networkStr, startRange, endRange),
		}
		fmt.Printf("NewIPAMPoolAllocator: %v\n", poolAllocator)

		return poolAllocator
	}

	return nil
}
