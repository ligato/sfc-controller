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

// Package ipam manages ip address ranges for subnets.  A subnet is of the
// form a.b.c.d/x ... then addresses are allocated.  It is possible to have
// subnets at the same "level" ie 10.1.1/24" and "10.1.2.0/24" and these
// would be allocatated addresses out of the separate pools.  But, having
// pools at different levels will cause issues: eg: "10.1.1.0/24", and
// "10.1.0.0/16" overlap.  The ipam can be extended to see if subnets are
// contained in other pools and the hierarchy can be "walked" to ensure
// addresses are set and cleared across levels.  Also, might have to have
// configurable address blocks per subnet so not allocating undesirable
// addresses.
package ipam

import (
	"fmt"
	"net"

	"github.com/ligato/sfc-controller/plugins/controller/idapi/bitmap"
)

// Ipv4Pool defines the pool for allocating ipv4 addresses
type Ipv4Pool struct {
	ipv4NetworkStr string // example form 10.5.3.0/24
	ipNetworku32   uint32
	ipMasku32      uint32
	numBitsInMask  int
	start          uint32
	end            uint32
	bm             *bitmap.Bitmap
	ipNetwork      *net.IPNet
}

func (ipv4Pool *Ipv4Pool) String() string {
	str := fmt.Sprintf("Network: %s, %s", ipv4Pool.ipNetwork, ipv4Pool.bm)
	return str
}

// SetIPInPool sets this address as allocated
func (ipv4Pool *Ipv4Pool) SetIPInPool(ipID uint32) (string, error) {

	if ipID < ipv4Pool.start || ipID > ipv4Pool.end {
		return "", fmt.Errorf("SetIPInPool: ipID '%d' out of range '%d-%d",
			ipID, ipv4Pool.start, ipv4Pool.end)
	}
	err := ipv4Pool.bm.Set(ipID - ipv4Pool.start + 1)
	if err != nil {
		return "", err
	}

	ipAddru32 := ipv4Pool.ipNetworku32 | ipID
	ipAddrStr := fmt.Sprintf("%d.", (ipAddru32>>24)&0xFF) +
		fmt.Sprintf("%d.", (ipAddru32>>16)&0xFF) +
		fmt.Sprintf("%d.", (ipAddru32>>8)&0xFF) +
		fmt.Sprintf("%d", (ipAddru32)&0xFF) +
		fmt.Sprintf("/%d", ipv4Pool.numBitsInMask)

	//fmt.Println("setIpIDInSubnet: ", ipAddrStr)

	return ipAddrStr, nil
}

// SetIPAddrIfInsidePool set this address as allocated if is contained in the pool
func (ipv4Pool *Ipv4Pool) SetIPAddrIfInsidePool(ipAddressStr string) {

	// see if this address falls within the subnet, and if it does, set the addr in the bitmap
	ipAddress := net.ParseIP(ipAddressStr)
	if ipAddress != nil && ipv4Pool.ipNetwork.Contains(ipAddress) {

		//fmt.Println("setIpAddrIfInsideSubnet: ", ipAddress)
		ip := ipAddress.To4()
		ipAddressu32 := uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
		ipID := ipAddressu32 &^ ipv4Pool.ipMasku32
		//fmt.Println("setIpAddrIfInsideSubnet: ipID", ipID)
		ipv4Pool.SetIPInPool(ipID)
	}
}

// AllocateFromPool find a free ipv4 addresss from the pool
func (ipv4Pool *Ipv4Pool) AllocateFromPool() (string, uint32, error) {

	freeBit := ipv4Pool.bm.FindFirstClear()
	if freeBit == 0 {
		return "", 0, fmt.Errorf("AllocateFromPool: all addresses allocated in '%s",
			ipv4Pool.ipv4NetworkStr)
	}

	ipv4Pool.bm.Set(freeBit)

	freeBit += ipv4Pool.start - 1

	ipAddru32 := ipv4Pool.ipNetworku32 | freeBit

	ipAddrStr := fmt.Sprintf("%d.", (ipAddru32>>24)&0xFF) +
		fmt.Sprintf("%d.", (ipAddru32>>16)&0xFF) +
		fmt.Sprintf("%d.", (ipAddru32>>8)&0xFF) +
		fmt.Sprintf("%d", (ipAddru32)&0xFF) // +
		//fmt.Sprintf("/%d", ipv4Pool.numBitsInMask)

	fmt.Println("AllocateFromSubnet: ", ipAddrStr, freeBit)

	return ipAddrStr, freeBit, nil
}

// NewIpv4Pool returns a pool for allocating ipv4 address from a v4 subnetwork
func NewIpv4Pool(ipv4NetworkStr string, start uint32, end uint32) *Ipv4Pool {

	_, n, err := net.ParseCIDR(ipv4NetworkStr)
	if err != nil {
		return nil
	}

	numBitsInMask, _ := n.Mask.Size()

	// example: /24 is 8 bits so need 2**8 for the bitmap
	if start == 0 {
		start = 1 // start at 1 not zero
	}
	if end == 0 {
		end = (1 << uint32(32-numBitsInMask)) - 2 // example: 8 bits: 256 - 1 - 1 = 254
	}
	bm := bitmap.NewBitmap(end - start + 1)

	ipv4Pool := &Ipv4Pool{
		ipv4NetworkStr: ipv4NetworkStr,
		ipNetworku32:   uint32(n.IP[0])<<24 | uint32(n.IP[1])<<16 | uint32(n.IP[2])<<8 | uint32(n.IP[3]),
		ipMasku32:      uint32(n.Mask[0])<<24 | uint32(n.Mask[1])<<16 | uint32(n.Mask[2])<<8 | uint32(n.Mask[3]),
		numBitsInMask:  numBitsInMask,
		bm:             bm,
		ipNetwork:      n,
		start:          start,
		end:            end,
	}

	fmt.Println("NewIPAMv4Pool: ", ipv4Pool, bm)

	return ipv4Pool
}
