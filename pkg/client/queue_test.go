package client

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestQueueInitializationWithCapacity(t *testing.T) {
	capacity := uint(10)
	queue := NewQueue[int](capacity)

	// The initial length of the queue should be 0
	assert.Equal(t, 0, len(queue.ToSlice()), "Initial length of queue should be 0")
}

func TestQueueFindIndexByKey(t *testing.T) {
	queue := NewQueue[int](10)
	queue.EnqueueWithKey("first", 1)
	queue.EnqueueWithKey("second", 2)

	// Find index by key
	index, found := queue.FindIndexByKey("second")
	assert.True(t, found, "Item with key 'second' should be found")
	assert.Equal(t, 1, index, "Index of item with key 'second' should be 1")

	// Try to find index by a non-existing key
	_, found = queue.FindIndexByKey("third")
	assert.False(t, found, "Item with key 'third' should not be found")
}

func TestDequeueEmptyQueue(t *testing.T) {
	queue := NewQueue[int](0)

	// Dequeue from an empty queue
	item, ok := queue.Dequeue()
	assert.False(t, ok, "Expected false from Dequeue on empty queue but got true")
	assert.Equal(t, 0, item, "Expected zero value from Dequeue on empty queue")
}

func TestQueueReset(t *testing.T) {
	queue := NewQueue[int](10)

	// Add items to the queue
	queue.EnqueueTail("key1", 1)
	queue.EnqueueTail("key2", 2)

	// Reset the queue with a different capacity
	newCapacity := uint(5)
	queue.Reset(newCapacity)

	// After reset, the size should be 0
	assert.Equal(t, 0, queue.Size(), "Size of queue after reset should be 0")

	// The capacity of the queue should match the new capacity
	assert.Equal(t, int(newCapacity), queue.Capacity(), "Capacity of queue after reset should match new capacity")
}

func TestQueueSize(t *testing.T) {
	queue := NewQueue[int](10)

	// Initially, the size should be 0
	assert.Equal(t, 0, queue.Size(), "Initial size of queue should be 0")

	// Add items to the queue
	queue.EnqueueTail("key1", 1)
	queue.EnqueueTail("key2", 2)

	// Now, the size should be 2
	assert.Equal(t, 2, queue.Size(), "Size of queue after enqueueing items should be 2")
}

func TestEnqueueAndDequeue(t *testing.T) {
	queue := NewQueue[int](0)

	// Test EnqueueTail
	queue.EnqueueTail("key1", 1)
	queue.EnqueueTail("key2", 2)

	// Test EnqueueHead
	queue.EnqueueHead("key3", 3)

	// Check the state of the queue
	expectedSliceAfterEnqueue := []int{3, 1, 2}
	assert.Equal(t, expectedSliceAfterEnqueue, queue.ToSlice(), "Queue contents after enqueue operations are not as expected")

	// Test Dequeue
	item, ok := queue.Dequeue()
	assert.True(t, ok, "Expected true from Dequeue but got false")
	assert.Equal(t, 3, item, "Dequeued item is not as expected")

	// Check the state of the queue after dequeue
	expectedSliceAfterDequeue := []int{1, 2}
	assert.Equal(t, expectedSliceAfterDequeue, queue.ToSlice(), "Queue contents after dequeue operation are not as expected")
}

func TestQueueDelete(t *testing.T) {
	queue := NewQueue[int](10)
	queue.EnqueueTail("key1", 1)
	queue.EnqueueTail("key2", 2)
	queue.EnqueueTail("key3", 3)

	// Delete the item at index 1 (value 2)
	err := queue.Delete(1)
	assert.NoError(t, err, "Error should not occur when deleting valid index")

	expectedItems := []int{1, 3}
	assert.Equal(t, expectedItems, queue.ToSlice(), "Items after deletion do not match expected items")
}

func TestQueueToSlice(t *testing.T) {
	queue := NewQueue[string](0)

	queue.EnqueueTail("key1", "a")
	queue.EnqueueTail("key2", "b")
	queue.EnqueueHead("key3", "c")

	expectedSlice := []string{"c", "a", "b"}
	assert.Equal(t, expectedSlice, queue.ToSlice(), "Queue to slice conversion did not match expected slice")
}

func TestQueueRange(t *testing.T) {
	queue := NewQueue[int](10)
	queue.EnqueueTail("key1", 10)
	queue.EnqueueTail("key2", 20)
	queue.EnqueueTail("key3", 30)

	var items []int
	queue.Range(func(index int, value int) {
		items = append(items, value)
	})

	expectedItems := []int{10, 20, 30}
	assert.Equal(t, expectedItems, items, "Items iterated by Range do not match expected items")
}

func TestConcurrentAccess(t *testing.T) {
	queue := NewQueue[int](1)

	// Perform concurrent Enqueue and Dequeue operations
	go func() {
		for i := 0; i < 1000; i++ {
			queue.EnqueueTail(fmt.Sprintf("key%d", i), i)
		}
	}()

	go func() {
		for i := 0; i < 1000; i++ {
			queue.EnqueueHead(fmt.Sprintf("headKey%d", i), i)
		}
	}()

	time.Sleep(1 * time.Second) // Wait for goroutines to finish

	// The exact contents of the queue are unpredictable due to concurrent access,
	// but we can check the size of the queue
	assert.Equal(t, 2000, len(queue.ToSlice()), "Queue size after concurrent access is not as expected")
}

func TestPeek(t *testing.T) {
	queue := NewQueue[int](50000)

	// Add items to the queue
	queue.EnqueueTail("key1", 10)
	queue.EnqueueTail("key2", 20)
	queue.EnqueueTail("key3", 30)

	// Test valid Peek
	item, err := queue.Peek(1)
	assert.NoError(t, err, "Expected no error from Peek on valid index")
	assert.Equal(t, 20, item, "Peeked item at index 1 is not as expected")

	// Test Peek with invalid index
	_, err = queue.Peek(5)
	assert.Error(t, err, "Expected error from Peek on invalid index")
}

func TestQueueEnqueueHead(t *testing.T) {
	queue := NewQueue[int](10)
	queue.EnqueueHead("first", 1)
	queue.EnqueueHead("second", 2)

	// Check if the items are enqueued correctly
	expectedItems := []int{2, 1}
	assert.Equal(t, expectedItems, queue.ToSlice(), "Items enqueued at head do not match expected items")

	// Check if the keys are stored correctly
	index, found := queue.FindIndexByKey("second")
	assert.True(t, found, "Item with key 'second' should be found")
	assert.Equal(t, 0, index, "Index of item with key 'second' should be 0")
}

func TestQueueEnqueueTail(t *testing.T) {
	queue := NewQueue[int](10)
	queue.EnqueueTail("first", 1)
	queue.EnqueueTail("second", 2)

	// Check if the items are enqueued correctly
	expectedItems := []int{1, 2}
	assert.Equal(t, expectedItems, queue.ToSlice(), "Items enqueued at tail do not match expected items")

	// Check if the keys are stored correctly
	index, found := queue.FindIndexByKey("second")
	assert.True(t, found, "Item with key 'second' should be found")
	assert.Equal(t, 1, index, "Index of item with key 'second' should be 1")
}
