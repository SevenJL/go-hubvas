// Package idgen provides distributed unique ID generation using the Snowflake scheme.
package idgen

import (
	"sync"
	"time"
)

// Snowflake epoch (2024-01-01T00:00:00Z in milliseconds).
const snowflakeEpoch int64 = 1704067200000

// Snowflake generates roughly time-ordered unique int64 IDs.
//
// Layout: [41 bits timestamp][10 bits machine][12 bits sequence]
// This supports ~4096 IDs/ms per machine for ~69 years.
type Snowflake struct {
	mu        sync.Mutex
	machineID int64
	sequence  int64
	lastMs    int64
}

// NewSnowflake creates a generator with the given machine ID (0–1023).
func NewSnowflake(machineID int64) *Snowflake {
	if machineID < 0 || machineID > 1023 {
		machineID = 0
	}
	return &Snowflake{machineID: machineID}
}

// NextID returns the next unique ID.
func (s *Snowflake) NextID() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	ms := time.Now().UnixMilli()

	if ms == s.lastMs {
		s.sequence = (s.sequence + 1) & 0xFFF
		if s.sequence == 0 {
			// Sequence exhausted this millisecond — wait for the next one.
			for ms <= s.lastMs {
				ms = time.Now().UnixMilli()
			}
		}
	} else {
		s.sequence = 0
	}

	s.lastMs = ms

	return ((ms - snowflakeEpoch) << 22) |
		(s.machineID << 12) |
		s.sequence
}
