package ringbuffer

import (
	"runtime"
	"sync/atomic"
)

type RingBuffer[T any] struct {
	size         int64
	capacity     int64
	writeIndex   int64
	readIndex    int64
	maxReadIndex int64
	element      []T
}

func NewRingBuffer[T any](capacity int64) *RingBuffer[T] {
	return &RingBuffer[T]{
		size:         0,
		capacity:     capacity,
		writeIndex:   0,
		readIndex:    0,
		maxReadIndex: 0,
		element:      make([]T, capacity),
	}
}

func (this *RingBuffer[T]) toIndex(index int64) int64 {
	return index % this.capacity
}

/*
	在Push和Pop的判断是否空和是否满的条件中的两个变量的原子读之间可能会发生调度，导致readIndex>maxReadIndex,
	或者writeIndex+1>readIndex，因此不能用==判断。
*/

func (this *RingBuffer[T]) Push(element T) bool {
	// 获取可写位置
	var currWriteIndex int64
	var currReadIndex int64
	for {
		currWriteIndex = atomic.LoadInt64(&this.writeIndex)
		currReadIndex = atomic.LoadInt64(&this.readIndex)
		// 是否满队
		if !(currWriteIndex >= currReadIndex && currWriteIndex-currReadIndex <= this.capacity-1) {
			return false
		}
		if atomic.CompareAndSwapInt64(&this.writeIndex, currWriteIndex, (currWriteIndex + 1)) {
			break
		}
	}
	// 写入该位置
	this.element[this.toIndex(currWriteIndex)]=element

	// 更新最大可读index
	for !atomic.CompareAndSwapInt64(&this.maxReadIndex, currWriteIndex, (currWriteIndex + 1)) {
		// 因为需要每个writer按顺序提交maxReadIndex，所以需要让出cpu，防止死锁
		runtime.Gosched()
	}

	atomic.AddInt64(&this.size, 1)

	return true
}

func (this *RingBuffer[T]) Pop() (T, bool) {
	// 检测是否为空
	var currReadIndex int64
	var currMaxReadIndex int64
	var res T
	for {
		currReadIndex = atomic.LoadInt64(&this.readIndex)
		currMaxReadIndex = atomic.LoadInt64(&this.maxReadIndex)

		if currReadIndex >= currMaxReadIndex {
			return res, false
		}

		res = this.element[this.toIndex(currReadIndex)]

		if atomic.CompareAndSwapInt64(&this.readIndex, currReadIndex, currReadIndex+1) {
			break
		}
	}

	atomic.AddInt64(&this.size, -1)

	return res, true
}
