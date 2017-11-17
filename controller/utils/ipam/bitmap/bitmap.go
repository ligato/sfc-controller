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

// package Bitmap is a utility package for ipam
package bitmap

import (
	"fmt"
)

const ALL_BITS_SET = 0xFFFFFFFFFFFFFFFF

type Bitmap struct {
	u64Array []uint64
	numBits uint32
}

func (bm *Bitmap) IsSet(i uint32) bool {
	if i < 0 || i > bm.numBits {
		return false
	}
	i -= 1;
	return bm.u64Array[i/64]&(1<< (63-i%64)) != 0
}

func (bm *Bitmap) Set(i uint32) error {
	if i < 0 || i > bm.numBits {
		err := fmt.Errorf("Bitmap: bit out of range: '%d'", i)
		return err
	}
	i -= 1;
	bm.u64Array[i/64] |= 1 << (63-i%64)
	return nil
}

func (bm *Bitmap) Clear(i uint32) {
	if i < 0 || i > bm.numBits {
		return
	}
	i -= 1;
	bm.u64Array[i/64] &^= 1 << (63-i%64)
}

func (bm *Bitmap) FindFirstClear() uint32 {
	for i, v := range bm.u64Array {
		fmt.Println("FindFirstClear:", i, v)
		if v != ALL_BITS_SET {
			for j := 0; j < 64; j++ {
				bit := uint32(i*64 + j + 1)
				//fmt.Println("FindFirstClear:", j, bit)
				if !bm.IsSet(bit) {
					return bit
				}
			}
		}
	}
	return 0
}

func (bm *Bitmap) String() string {
	str := fmt.Sprintf("numBits: %d, bits:[", bm.numBits)
	for _, v := range bm.u64Array {
		str += fmt.Sprintf("%016X", v)
	}
	str += fmt.Sprintf("]")

	return str
}

func NewBitmap(numBits uint32) *Bitmap {
	bm := &Bitmap{
		u64Array: make([]uint64, (numBits-1)/64+1),
		numBits: numBits,
	}
	//fmt.Println("NewBitmap: ", bm)
	return bm
}
