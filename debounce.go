package playback

import (
	"sync"
	"time"
)

var debounce = debounceMap{
	store: make(map[string]time.Time, 10),
}

func Debounce(key string, f func(), delay time.Duration) {
	go func() {
		debounce.Put(key, time.Now())
		time.Sleep(delay)
		debounce.DoIfLast(key, f, delay)
	}()
}

type debounceMap struct {
	store map[string]time.Time
	mutex sync.RWMutex
}

func (m *debounceMap) DoIfLast(key string, f func(), delay time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if time.Since(m.get(key)) < delay {
		return
	}

	f()
	m.delete(key)
}

func (m *debounceMap) Put(key string, value time.Time) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.store[key] = value
}

func (m *debounceMap) Delete(key string) time.Time {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.delete(key)
}

func (m *debounceMap) delete(key string) time.Time {
	value := m.store[key]
	delete(m.store, key)

	return value
}

func (m *debounceMap) Get(key string) time.Time {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.get(key)
}

func (m *debounceMap) get(key string) time.Time {
	return m.store[key]
}
