package mpmc

import (
	"math"
	"runtime"
	"sync/atomic"

	"golang.org/x/sys/cpu"
)

// 最大自旋次数
const _MaxSpinTimes = 64

type _DataStatus uint64

const (
	_ReadUnReady _DataStatus = iota
	// 可读
	_ReadReady
)

// 加CacheLinePad后耗时性能提升很多
type RingBuffer[T any] struct {
	size       int64
	_          cpu.CacheLinePad
	capacity   uint64
	_          cpu.CacheLinePad
	writeIndex uint64
	_          cpu.CacheLinePad
	readIndex  uint64
	_          cpu.CacheLinePad
	element    []_RingBufferData[T]
}

type _RingBufferData[T any] struct {
	status _DataStatus
	val    T
}

func NewRingBuffer[T any](capacity uint64) *RingBuffer[T] {
	return &RingBuffer[T]{
		size:       0,
		capacity:   capacity,
		writeIndex: 0,
		readIndex:  0,
		element:    make([]_RingBufferData[T], capacity),
	}
}

// 用来测试index发生uint64回环的情况
func NewRingBuffer2[T any](capacity uint64) *RingBuffer[T] {
	return &RingBuffer[T]{
		size:       0,
		capacity:   capacity,
		writeIndex: math.MaxUint64 - 5,
		readIndex:  math.MaxUint64 - 5,
		element:    make([]_RingBufferData[T], capacity),
	}
}

func (this *RingBuffer[T]) toIndex(index uint64) uint64 {
	return index % this.capacity
}

func (this *RingBuffer[T]) Push(element T) bool {
	spinTimes := 0

	for spinTimes < _MaxSpinTimes {

		// 获取可写位置
		currWriteIndex := atomic.LoadUint64(&this.writeIndex)
		slot := &this.element[this.toIndex(currWriteIndex)]
		slot_next := &this.element[this.toIndex(currWriteIndex+1)]
		if _DataStatus(atomic.LoadUint64((*uint64)(&slot_next.status))) == _ReadReady {
			return false
		}

		if atomic.CompareAndSwapUint64(&this.writeIndex, currWriteIndex, currWriteIndex+1) {
			slot.val = element
			atomic.StoreUint64((*uint64)(&slot.status), uint64(_ReadReady))
			atomic.AddInt64(&this.size, 1)
			return true
		}
		// 在这里加runtime.Gosched()，性能提升，猜测是降低自旋的强度
		runtime.Gosched()
		spinTimes++
	}
	return false
}

func (this *RingBuffer[T]) Pop() (T, bool) {
	spinTimes := 0
	var res T
	for spinTimes < _MaxSpinTimes {

		currReadIndex := atomic.LoadUint64(&this.readIndex)

		slot := &this.element[this.toIndex(currReadIndex)]
		if _DataStatus(atomic.LoadUint64((*uint64)(&slot.status))) == _ReadUnReady {
			return res, false
		}

		if atomic.CompareAndSwapUint64(&this.readIndex, currReadIndex, currReadIndex+1) {
			res = slot.val
			atomic.StoreUint64((*uint64)(&slot.status), uint64(_ReadUnReady))
			atomic.AddInt64(&this.size, -1)
			return res, true
		}
		// 在这里加runtime.Gosched()，性能提升，猜测是降低自旋的强度
		runtime.Gosched()
		spinTimes++
	}

	return res, false
}

func (this *RingBuffer[T]) Size() int64 {
	return atomic.LoadInt64(&this.size)
}

func (this *RingBuffer[T]) Cap() uint64 {
	return this.capacity
}
