package memory

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/uzzz/httpcache"
)

type item struct {
	data    []byte
	expires time.Time
	alNode  *accessListNode
}

type accessList struct {
	head, tail *accessListNode
}

func (al *accessList) addToHead(key uint64) {
	node := &accessListNode{key: key}

	if al.head == nil {
		al.tail = node
	} else {
		al.head.prev = node
	}

	node.next = al.head
	al.head = node
}

func (al *accessList) remove(item *accessListNode) {
	if item == al.head {
		al.head = al.head.next
	} else {
		item.prev.next = item.next
	}

	if item == al.tail {
		al.tail = item.prev
	} else {
		item.next.prev = item.prev
	}
}

func (al *accessList) removeFromTail() (uint64, bool) {
	if al.tail == nil {
		return 0, false
	}

	tmp := al.tail
	if al.head.next == nil {
		al.head = nil
	} else {
		al.tail.prev.next = nil
	}
	al.tail = al.tail.prev

	return tmp.key, true
}

type accessListNode struct {
	next, prev *accessListNode
	key        uint64
}

type Store struct {
	mutex         sync.RWMutex
	sizeBytes     int
	capacityBytes int
	data          map[uint64]item
	al            *accessList
}

// NewStore initializes memory store.
func NewStore(opts ...Option) (*Store, error) {
	options := defaultOptions

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, err
		}
	}

	return &Store{
		data:          make(map[uint64]item),
		capacityBytes: options.capacityBytes,
		al:            &accessList{},
	}, nil
}

// Option is used to set Store settings.
type Option func(o *Options) error

type Options struct {
	capacityBytes int
}

var defaultOptions = Options{
	capacityBytes: math.MaxInt,
}

// Get data from store
func (s *Store) Get(key uint64) ([]byte, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	i, ok := s.data[key]
	if !ok {
		return nil, httpcache.ErrNoEntry
	}
	if !i.expires.IsZero() && i.expires.Before(time.Now()) {
		return nil, httpcache.ErrNoEntry
	}

	s.al.remove(i.alNode)
	s.al.addToHead(key)
	i.alNode = s.al.head
	s.data[key] = i

	return i.data, nil
}

// Set sets data
func (s *Store) Set(key uint64, data []byte, ttl time.Duration) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var (
		i           item
		ok          bool
		bytesNeeded int
	)

	i, ok = s.data[key]
	if ok { // override
		bytesNeeded = len(data) - len(i.data)
	} else { // new item
		bytesNeeded = len(data)
		i = item{}
	}

	if bytesNeeded > s.capacityBytes {
		return httpcache.ErrEntryIsTooBig
	}

	leftBytes := s.capacityLeftBytes()
	if bytesNeeded > leftBytes {
		s.evict(bytesNeeded - leftBytes)
	}

	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	i.data = dataCopy

	s.sizeBytes += bytesNeeded
	if i.alNode != nil { // value update
		s.al.remove(i.alNode)
	}
	s.al.addToHead(key)

	i.expires = time.Now().Add(ttl)
	i.alNode = s.al.head
	s.data[key] = i

	return nil
}

func (s *Store) capacityLeftBytes() int {
	return s.capacityBytes - s.sizeBytes
}

func (s *Store) evict(bytes int) {
	evictedBytes := 0
	for evictedBytes < bytes {
		key, ok := s.al.removeFromTail()
		if !ok {
			panic("no more items in the access list") // should never happen
		}
		i, ok := s.data[key]
		if !ok {
			panic("key from the access list not found") // should never happen
		}
		evictedBytes += len(i.data)
		delete(s.data, key)
	}
}

// WithCapacity sets the maximum size of cached data in bytes.
func WithCapacity(bytes int) Option {
	return func(o *Options) error {
		if bytes <= 1 {
			return fmt.Errorf("capacity must be greater than %v", bytes)
		}

		o.capacityBytes = bytes

		return nil
	}
}

var _ httpcache.Store = (*Store)(nil)
