package client

import (
	"sync"

	"github.com/pkg/errors"
)

type Queue[T any] struct {
	items []T
	keys  map[string]int
	mu    sync.Mutex
}

func NewQueue[T any](capacity uint) *Queue[T] {
	return &Queue[T]{
		items: make([]T, 0, capacity),
		keys:  make(map[string]int),
	}
}

func (q *Queue[T]) Delete(index int) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if index < 0 || index >= len(q.items) {
		return errors.New("index out of range")
	}

	q.items = append(q.items[:index], q.items[index+1:]...)
	// Update keys map
	for key, idx := range q.keys {
		if idx > index {
			q.keys[key] = idx - 1
		} else if idx == index {
			delete(q.keys, key)
		}
	}

	return nil
}

func (q *Queue[T]) FindIndexByKey(key string) (int, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	index, found := q.keys[key]
	return index, found
}

func (q *Queue[T]) EnqueueWithKey(key string, item T) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.items = append(q.items, item)
	q.keys[key] = len(q.items) - 1
}

func (q *Queue[T]) Reset(capacity uint) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.items = make([]T, 0, capacity)
}

func (q *Queue[T]) Peek(index int) (T, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if index < 0 || index >= len(q.items) {
		var zeroValue T
		return zeroValue, errors.New("index out of range")
	}

	return q.items[index], nil
}

func (q *Queue[T]) Range(f func(index int, value T)) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, item := range q.items {
		f(i, item)
	}
}

func (q *Queue[T]) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	return len(q.items)
}

func (q *Queue[T]) EnqueueHead(key string, item T) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.items = append([]T{item}, q.items...)
	q.updateKeysAfterEnqueue(0)
	q.keys[key] = 0
}

func (q *Queue[T]) EnqueueTail(key string, item T) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.items = append(q.items, item)
	q.keys[key] = len(q.items) - 1
}

func (q *Queue[T]) updateKeysAfterEnqueue(index int) {
	for k, v := range q.keys {
		if v >= index {
			q.keys[k] = v + 1
		}
	}
}

func (q *Queue[T]) Dequeue() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		var zeroValue T
		return zeroValue, false
	}

	item := q.items[0]
	q.items = q.items[1:]
	return item, true
}

func (q *Queue[T]) ToSlice() []T {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]T(nil), q.items...)
}

func (q *Queue[T]) Capacity() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	return cap(q.items)
}
