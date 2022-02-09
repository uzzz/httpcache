package memory

import (
	"reflect"
	"testing"
	"time"

	"github.com/uzzz/httpcache"
)

func TestStore(t *testing.T) {
	store, err := NewStore()
	if err != nil {
		t.Fatal("unexpected error", err)
	}
	data := []byte("data")
	err = store.Set(uint64(1), data, time.Minute)
	if err != nil {
		t.Error("unexpected error", err)
	}

	fetchedData, err := store.Get(uint64(1))
	if err != nil {
		t.Error("unexpected error", err)
	}
	if !reflect.DeepEqual(data, fetchedData) {
		t.Errorf("expected to return '%s', got '%s'", string(data), string(fetchedData))
	}

	_, err = store.Get(uint64(2))
	if err != httpcache.ErrNoEntry {
		t.Errorf("expected httpcache.ErrNoEntry, got %s", err)
	}
}

func TestStoreDataCopy(t *testing.T) {
	store, err := NewStore()
	if err != nil {
		t.Fatal("unexpected error", err)
	}
	data := []byte("data")

	if err = store.Set(uint64(1), data, time.Millisecond); err != nil {
		t.Error("unexpected error", err)
	}

	data[0] = 'x' // change original value

	fetchedData, err := store.Get(uint64(1))
	if err != nil {
		t.Error("unexpected error", err)
	}
	if !reflect.DeepEqual([]byte("data"), fetchedData) {
		t.Errorf("expected to return '%s', got '%s'", string(data), string(fetchedData))
	}
}

func TestStoreTTL(t *testing.T) {
	store, err := NewStore()
	if err != nil {
		t.Fatal("unexpected error", err)
	}

	data := []byte("data")

	if err = store.Set(uint64(1), data, time.Millisecond); err != nil {
		t.Error("unexpected error", err)
	}

	fetchedData, err := store.Get(uint64(1))
	if err != nil {
		t.Error("unexpected error", err)
	}
	if !reflect.DeepEqual(data, fetchedData) {
		t.Errorf("expected to return '%s', got '%s'", string(data), string(fetchedData))
	}

	time.Sleep(2 * time.Millisecond)

	_, err = store.Get(uint64(1))
	if err != httpcache.ErrNoEntry {
		t.Errorf("expected httpcache.ErrNoEntry, got %s", err)
	}
}

func TestStoreCapacity(t *testing.T) {
	store, err := NewStore(WithCapacity(8))
	if err != nil {
		t.Fatal("unexpected error", err)
	}

	if err := store.Set(uint64(1), []byte("1234567890"), time.Minute); err != httpcache.ErrEntryIsTooBig {
		t.Errorf("unexpected error httpcache.ErrEntryIsTooBig, got '%s'", err)
	}

	if err := store.Set(uint64(1), []byte("12345678"), time.Minute); err != nil {
		t.Error("unexpected error", err)
	}

	fetchedData, err := store.Get(uint64(1))
	if err != nil {
		t.Error("unexpected error", err)
	}
	if !reflect.DeepEqual([]byte("12345678"), fetchedData) {
		t.Errorf("expected to return '%s', got '%s'", "12345678", string(fetchedData))
	}
}

func TestStoreEviction(t *testing.T) {
	t.Run("one item", func(t *testing.T) {
		store, err := NewStore(WithCapacity(10))
		if err != nil {
			t.Fatal("unexpected error", err)
		}
		data := []byte("data")

		if err := store.Set(uint64(1), data, time.Minute); err != nil {
			t.Error("unexpected error", err)
		}
		if err := store.Set(uint64(2), data, time.Minute); err != nil {
			t.Error("unexpected error", err)
		}
		if err := store.Set(uint64(3), data, time.Minute); err != nil { // exceeds capacity
			t.Error("unexpected error", err)
		}

		if _, err := store.Get(uint64(1)); err != httpcache.ErrNoEntry { // evicts least recently used
			t.Errorf("expected error httpcache.ErrNoEntry, got %s", err)
		}
	})

	t.Run("touched", func(t *testing.T) {
		store, err := NewStore(WithCapacity(10))
		if err != nil {
			t.Fatal("unexpected error", err)
		}
		data := []byte("data")

		if err := store.Set(uint64(1), data, time.Minute); err != nil {
			t.Error("unexpected error", err)
		}
		if err := store.Set(uint64(2), data, time.Minute); err != nil {
			t.Error("unexpected error", err)
		}
		// touch key 1
		fetchedData, err := store.Get(uint64(1))
		if err != nil {
			t.Error("unexpected error", err)
		}
		if !reflect.DeepEqual(data, fetchedData) {
			t.Errorf("expected to return '%s', got '%s'", string(data), string(fetchedData))
		}
		// pu another item that exceeds capacity
		if err := store.Set(uint64(3), data, time.Minute); err != nil {
			t.Error("unexpected error", err)
		}

		if _, err := store.Get(uint64(2)); err != httpcache.ErrNoEntry { // evicts least recently used
			t.Errorf("expected error httpcache.ErrNoEntry, got %v", err)
		}
	})

	t.Run("few items", func(t *testing.T) {
		store, err := NewStore(WithCapacity(10))
		if err != nil {
			t.Fatal("unexpected error", err)
		}
		data := []byte("data")

		if err := store.Set(uint64(1), data, time.Minute); err != nil {
			t.Error("unexpected error", err)
		}
		if err := store.Set(uint64(2), data, time.Minute); err != nil {
			t.Error("unexpected error", err)
		}
		if err := store.Set(uint64(3), []byte("1234567890"), time.Minute); err != nil { // exceeds capacity
			t.Error("unexpected error", err)
		}

		if _, err := store.Get(uint64(1)); err != httpcache.ErrNoEntry { // evicts least recently used
			t.Errorf("expected error httpcache.ErrNoEntry, got %s", err)
		}
		if _, err := store.Get(uint64(2)); err != httpcache.ErrNoEntry { // evicts least recently used
			t.Errorf("expected error httpcache.ErrNoEntry, got %s", err)
		}
	})
}
