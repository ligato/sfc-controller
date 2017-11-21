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

// package ipam manages ip address ranges for subnets.  A subnet is of the
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
	"github.com/ligato/sfc-controller/controller/utils/ipam/bitmap"
)

type ipamSubnet struct {
	subnetStr     string // example form 10.5.3.0/24
	ipNetworku32  uint32
	ipMasku32     uint32
	numBitsInMask int
	bm            *bitmap.Bitmap
	ipNetwork     *net.IPNet
}

var ipamSubnetCache map[string]*ipamSubnet = make(map[string]*ipamSubnet)

func AllocateFromSubnet(ipamSubnetStr string) (string, uint32, error) {

	var ipamSubnet *ipamSubnet
	var exists bool
	var err error

	ipamSubnet, exists = ipamSubnetCache[ipamSubnetStr]
	if !exists {
		ipamSubnet, err = newIPAMSubnet(ipamSubnetStr)
		if err != nil {
			return "", 0, err
		}
		ipamSubnetCache[ipamSubnetStr] = ipamSubnet
	}
	return ipamSubnet.allocateFromSubnet()
}

func SetIpIDInSubnet(ipamSubnetStr string, ipID uint32) (string, error) {

	var ipamSubnet *ipamSubnet
	var exists bool
	var err error

	ipamSubnet, exists = ipamSubnetCache[ipamSubnetStr]
	if !exists {
		ipamSubnet, err = newIPAMSubnet(ipamSubnetStr)
		if err != nil {
			return "", err
		}
		ipamSubnetCache[ipamSubnetStr] = ipamSubnet
	}
	return ipamSubnet.setIpIDInSubnet(ipID)
}

func SetIpAddrIfInsideSubnet(ipamSubnetStr string, ipAddress string) {

	var ipamSubnet *ipamSubnet
	var exists bool
	var err error

	ipamSubnet, exists = ipamSubnetCache[ipamSubnetStr]
	if !exists {
		ipamSubnet, err = newIPAMSubnet(ipamSubnetStr)
		if err != nil {
			return
		}
		ipamSubnetCache[ipamSubnetStr] = ipamSubnet
	}

	ipamSubnet.setIpAddrIfInsideSubnet(ipAddress)
}

func DumpSubnet(ipamSubnetStr string) (string) {

	var ipamSubnet *ipamSubnet
	var exists bool
	var err error

	ipamSubnet, exists = ipamSubnetCache[ipamSubnetStr]
	if !exists {
		ipamSubnet, err = newIPAMSubnet(ipamSubnetStr)
		if err != nil {
			return err.Error()
		}
		ipamSubnetCache[ipamSubnetStr] = ipamSubnet
	}
	return fmt.Sprintf("ipam: %s", ipamSubnet)
}

func (ipamSubnet *ipamSubnet) String() string {
	str := fmt.Sprintf("network: %s, %s", ipamSubnet.ipNetwork, ipamSubnet.bm)
	return str
}

func (ipamSubnet *ipamSubnet) setIpIDInSubnet(ipID uint32) (string, error) {
	err := ipamSubnet.bm.Set(ipID)
	if err != nil {
		return "", fmt.Errorf("setIpIDInSubnet: ipID(%d) not in subnet '%s", ipID, ipamSubnet.subnetStr)
	}

	ipAddru32 := ipamSubnet.ipNetworku32 | ipID
	ipAddrStr := fmt.Sprintf("%d.", (ipAddru32>>24) & 0xFF) +
		fmt.Sprintf("%d.", (ipAddru32>>16) & 0xFF) +
		fmt.Sprintf("%d.", (ipAddru32>>8) & 0xFF) +
		fmt.Sprintf("%d", (ipAddru32) & 0xFF) +
		fmt.Sprintf("/%d", ipamSubnet.numBitsInMask)

	//fmt.Println("setIpIDInSubnet: ", ipAddrStr)

	return ipAddrStr, nil
}

func (ipamSubnet *ipamSubnet) setIpAddrIfInsideSubnet(ipAddressStr string) {

	// see if this address falls within the subnet, and if it does, set the addr in the bitmap
	ipAddress := net.ParseIP(ipAddressStr)
	if ipAddress != nil && ipamSubnet.ipNetwork.Contains(ipAddress) {

		//fmt.Println("setIpAddrIfInsideSubnet: ", ipAddress)
		ip := ipAddress.To4()
		ipAddressu32 := uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
		ipID := ipAddressu32 &^ ipamSubnet.ipMasku32
		//fmt.Println("setIpAddrIfInsideSubnet: ipID", ipID)
		ipamSubnet.setIpIDInSubnet(ipID)
	}
}

func (ipamSubnet *ipamSubnet) allocateFromSubnet() (string, uint32, error) {
	freeBit := ipamSubnet.bm.FindFirstClear()
	if freeBit == 0 {
		return "", 0, fmt.Errorf("AllocateFromSubnet: all addresses allocated in '%s", ipamSubnet.subnetStr)
	}

	ipamSubnet.bm.Set(freeBit)

	ipAddru32 := ipamSubnet.ipNetworku32 | freeBit

	ipAddrStr := fmt.Sprintf("%d.", (ipAddru32>>24) & 0xFF) +
		fmt.Sprintf("%d.", (ipAddru32>>16) & 0xFF) +
		fmt.Sprintf("%d.", (ipAddru32>>8) & 0xFF) +
		fmt.Sprintf("%d", (ipAddru32) & 0xFF) +
		fmt.Sprintf("/%d", ipamSubnet.numBitsInMask)

	//fmt.Println("AllocateFromSubnet: ", ipAddrStr, freeBit )

	return ipAddrStr, freeBit, nil
}

func newIPAMSubnet(ipSubnetStr string) (*ipamSubnet, error) {

	_, n, err := net.ParseCIDR(ipSubnetStr)
	if err != nil {
		return nil, err
	}

	numBits, _ := n.Mask.Size()

	// example: 32 - /24 is 8 bits so need 2**8 for the bitmap
	bm := bitmap.NewBitmap((1<<uint32(32 - numBits))-1)

	ipamSubnet := &ipamSubnet{
		subnetStr:     ipSubnetStr,
		ipNetworku32:  uint32(n.IP[0])<<24 | uint32(n.IP[1])<<16 | uint32(n.IP[2])<<8 | uint32(n.IP[3]),
		ipMasku32:     uint32(n.Mask[0])<<24 | uint32(n.Mask[1])<<16 | uint32(n.Mask[2])<<8 | uint32(n.Mask[3]),
		numBitsInMask: numBits,
		bm:            bm,
		ipNetwork:     n,
	}

	//fmt.Println("newIPAMSubnet: ", ipamSubnet, bm)

	return ipamSubnet, nil
}
