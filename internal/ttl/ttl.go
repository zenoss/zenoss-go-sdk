package ttl

import (
	"sync"
	"time"

	"github.com/mitchellh/hashstructure/v2"

	"github.com/zenoss/zenoss-protobufs/go/cloud/data_receiver"
)

// Tracker is an expiring cache for uint64 keys and values.
type Tracker struct {
	m        map[uint64]*uint64WithTime
	mu       sync.Mutex
	maxAgeNS int64
}

type uint64WithTime struct {
	v uint64
	t int64
}

// NewTracker returns a new Tracker with specified configuration.
func NewTracker(maxAge time.Duration, checkInterval time.Duration) *Tracker {
	t := &Tracker{
		m:        make(map[uint64]*uint64WithTime),
		maxAgeNS: maxAge.Nanoseconds(),
	}

	go func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		for tick := range ticker.C {
			t.AgeAll(tick)
		}
	}()

	return t
}

// AgeAll deletes entries that would have expired as of "asof".
func (t *Tracker) AgeAll(asof time.Time) {
	t.mu.Lock()
	for k, v := range t.m {
		if asof.UnixNano()-v.t > t.maxAgeNS {
			delete(t.m, k)
		}
	}
	t.mu.Unlock()
}

// IsAlive returns true if key and value combination haven't expired.
// Otherwise it adds them to the cache and returns false.
func (t *Tracker) IsAlive(key uint64, value uint64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if old, ok := t.m[key]; ok {
		if value == old.v {
			return true
		}
	}

	t.m[key] = &uint64WithTime{v: value, t: time.Now().UnixNano()}

	return false
}

// IsModelAlive returns true if model hasn't expired from the cache.
// Otherwise it adds model to the cache and returns false.
func (t *Tracker) IsModelAlive(model *data_receiver.Model) bool {
	key, err := hashstructure.Hash(model.Dimensions, hashstructure.FormatV2, nil)
	if err != nil {
		return false
	}

	value, err := hashstructure.Hash(model.MetadataFields, hashstructure.FormatV2, nil)
	if err != nil {
		return false
	}

	return t.IsAlive(key, value)
}
