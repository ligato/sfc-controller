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
	"github.com/ligato/sfc-controller/plugins/controller/idapi/bitmap"
)

type VxlanVniAllocatorType struct {
	StartVni uint32 `json:"start_vni,omitempty"`
	EndVni   uint32 `json:"end_vni,omitempty"`
	numBits  uint32
	BM       *bitmap.Bitmap `json:"bitmap,omitempty"`
}

func (v *VxlanVniAllocatorType) String() string {
	str := fmt.Sprintf("vxlan vni's: [%d-%d], %s",
		v.StartVni,
		v.EndVni,
		v.BM)
	return str
}

// SetVni marks the vni in teh range as allocated
func (v *VxlanVniAllocatorType) SetVni(vni uint32) error {
	if vni < v.StartVni || vni > v.EndVni {
		return fmt.Errorf("SetVni: vni '%d' out of range '%d-%d",
			vni, v.StartVni, v.EndVni)
	}
	err := v.BM.Set(vni - v.StartVni + 1)
	if err != nil {
		return err
	}
	return nil
}

// AllocateVni allocates a free vni
func (v *VxlanVniAllocatorType) AllocateVni() (uint32, error) {
	freeBit := v.BM.FindFirstClear()
	if freeBit == 0 {
		return 0, fmt.Errorf("AllocateVni: all vni's allocated")
	}

	v.BM.Set(freeBit)

	return freeBit + v.StartVni - 1, nil
}

// NewVxlanVniAllocator allocates a range of id's lfor vni's
func NewVxlanVniAllocator(startVni uint32, endVni uint32) *VxlanVniAllocatorType {

	numBits := endVni - startVni + 1

	bm := bitmap.NewBitmap(numBits)

	vxlanVniAllocator := &VxlanVniAllocatorType{
		numBits:  numBits,
		BM:       bm,
		StartVni: startVni,
		EndVni:   endVni,
	}

	return vxlanVniAllocator
}
