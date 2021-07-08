/*
Copyright 2021 The SuperEdge Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package asset

import (
	"errors"
	"fmt"
	"math/big"
	"sync"

	"k8s.io/klog/v2"
)

type AsSet struct {
	sync.Mutex
	// AsStart is start of AS that can be allocated
	AsStart int
	// MaxAsCount is the maximum number of AS that can be allocated
	MaxAsCount int
	// allocatedAS counts the number of AS allocated
	allocatedAS int
	// nextCandidate points to the next AS that should be free
	nextCandidate int
	// used is a bitmap used to track the AS allocated
	used big.Int
	// TODO: label is used to identify the metrics
	label string
}

var (
	// ErrCIDRRangeNoCIDRsRemaining occurs when there is no more space
	// to allocate CIDR ranges.
	ErrNoASRemaining = errors.New(
		"bgp as number allocation failed; there are no remaining AS left to allocate in the accepted range")
)

// NewASSet creates a new AsSet.
func NewASSet(asStart int, maxAsCount int) (*AsSet, error) {
	if asStart < 0 {
		return nil, fmt.Errorf("negative asStart is not valid")
	}

	return &AsSet{
		Mutex:      sync.Mutex{},
		AsStart:    asStart,
		MaxAsCount: maxAsCount,
	}, nil
}

// AllocateNext allocates the next free As Number. This will set the AS
// as occupied and return the allocated AS.
func (s *AsSet) AllocateNext() (int, error) {
	var asNumber int
	s.Lock()
	defer s.Unlock()

	if s.allocatedAS == s.MaxAsCount {
		return asNumber, ErrNoASRemaining
	}
	candidate := s.nextCandidate
	var i int
	for i = 0; i < s.MaxAsCount; i++ {
		if s.used.Bit(candidate) == 0 {
			break
		}
		candidate = (candidate + 1) % s.MaxAsCount
	}

	s.nextCandidate = (candidate + 1) % s.MaxAsCount
	s.used.SetBit(&s.used, candidate, 1)
	s.allocatedAS++

	return s.AsStart + candidate, nil
}

// Release releases the given tarAS in range.
func (s *AsSet) Release(tarAs int) error {
	s.Lock()
	defer s.Unlock()

	i := tarAs - s.AsStart
	if i < 0 || i > s.MaxAsCount {
		return fmt.Errorf("tarAs number %d is not in range of Asset", i)
	}
	if s.used.Bit(i) != 0 {
		s.used.SetBit(&s.used, i, 0)
		s.allocatedAS--
	}

	return nil
}

// Occupy marks the given As as used. Occupy succeeds even if the AS was previously used.
func (s *AsSet) Occupy(tarAs int) (err error) {

	s.Lock()
	defer s.Unlock()
	i := tarAs - s.AsStart
	if i < 0 || i > s.MaxAsCount {
		return fmt.Errorf("tarAs number %d is not in range of Asset", i)
	}
	if s.used.Bit(i) == 0 {
		s.used.SetBit(&s.used, i, 1)
		s.allocatedAS++
	}
	klog.Infof("occupy as number %d success", tarAs)
	return nil
}
