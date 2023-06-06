package ringbuffer

import (
	"math"
	"runtime"
	"sync/atomic"
)

type RingBuffer[T any] struct {
	size         int64
	capacity     uint64
	writeIndex   uint64
	readIndex    uint64
	maxReadIndex uint64
	element      []T
}

func NewRingBuffer[T any](capacity uint64) *RingBuffer[T] {
	return &RingBuffer[T]{
		size:         0,
		capacity:     capacity,
		writeIndex:   0,
		readIndex:    0,
		maxReadIndex: 0,
		element:      make([]T, capacity),
	}
}

func (this *RingBuffer[T]) toIndex(index uint64) uint64 {
	return index % this.capacity
}

/*
	在Push和Pop的判断是否空和是否满的条件中的两个变量的原子读之间可能会发生调度，导致readIndex>maxReadIndex,
	或者writeIndex+1>readIndex，因此不能用==判断。
*/

func (this *RingBuffer[T]) Push(element T) bool {
	// 获取可写位置
	var currWriteIndex uint64
	var currReadIndex uint64
	for {

		// 顺序一致，不可调换顺序
		currReadIndex = atomic.LoadUint64(&this.readIndex)
		currWriteIndex = atomic.LoadUint64(&this.writeIndex)

		// 检查是否满队
		if currWriteIndex < currReadIndex {
			// 处理回环的情况
			if (math.MaxInt64-currReadIndex+1)+currWriteIndex >= this.capacity-1 {
				return false
			}
		} else {
			if currWriteIndex-currReadIndex >= this.capacity-1 {
				return false
			}
		}

		if atomic.CompareAndSwapUint64(&this.writeIndex, currWriteIndex, (currWriteIndex + 1)) {
			break
		}
	}

	// 写入该位置
	this.element[this.toIndex(currWriteIndex)] = element

	// 更新最大可读index
	for !atomic.CompareAndSwapUint64(&this.maxReadIndex, currWriteIndex, (currWriteIndex + 1)) {
		// 因为需要每个writer按顺序提交maxReadIndex，所以需要让出cpu，防止死锁
		runtime.Gosched()
	}

	atomic.AddInt64(&this.size, 1)

	return true
}

func (this *RingBuffer[T]) Pop() (T, bool) {
	// 检测是否为空
	var currReadIndex uint64
	var currMaxReadIndex uint64
	var res T
	for {
		// 顺序一致，不可调换顺序，必须currReadIndex<=currMaxReadIndex
		currReadIndex = atomic.LoadUint64(&this.readIndex)
		currMaxReadIndex = atomic.LoadUint64(&this.maxReadIndex)

		// // 检查是否为空
		// if currReadIndex == currMaxReadIndex {
		// 	return res, false
		// }
		// // 检查是否发生回环
		// if currReadIndex>currMaxReadIndex{
		// 	// 发生回环
		// }
		// 上面代码等效于
		if currReadIndex == currMaxReadIndex {
			return res, false
		}

		res = this.element[this.toIndex(currReadIndex)]

		if atomic.CompareAndSwapUint64(&this.readIndex, currReadIndex, currReadIndex+1) {
			break
		}
	}

	atomic.AddInt64(&this.size, -1)

	return res, true
}
