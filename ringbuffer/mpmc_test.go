package ringbuffer

import (
	"fmt"
	"sort"
	"sync"
	"testing"
)

// 使用-race参数会发生读写冲突，原因是持有旧readIndex的reader访问了旧值，
// 而此时writer正在写该位置。但实际并不会造成错误，因为持有旧readIndex的
// reader将在之后的cas操作中失败，并不会返回未定义的旧值。

func TestNormal(t *testing.T) {
	t.Skip()
	rb := NewRingBuffer[int](1024)
	for i := 0; i < 10; i++ {
		rb.Push(i)
	}
	for i := 0; i < 15; i++ {
		e, _ := rb.Pop()
		if e != i {
			t.Fatalf("e!=%v", i)
		}
	}
}

func TestValidity(t *testing.T) {
	wg := sync.WaitGroup{}
	rb := NewRingBuffer[int](1024)
	reader := 1
	sender := 3
	dataSize := 800 * 10000
	dataRange := make([][]int, 0, sender)
	fragSize := dataSize / sender
	totalSize := fragSize * sender
	
	start := 0
	for i := 0; i < sender; i++ {
		dataRange = append(dataRange, []int{start, start + fragSize})
		start += fragSize
	}

	for i := 0; i < sender; i++ {
		wg.Add(1)
		go func(start, end int) {
			for j := start; j < end; j++ {
				for !rb.Push(j) {
					continue
				}
			}
			wg.Done()
		}(dataRange[i][0], dataRange[i][1])
	}

	resArr := make([]int, 0, totalSize)
	resArrLocker := sync.Mutex{}

	for i := 0; i < reader; i++ {
		wg.Add(1)
		go func() {
			exit := false
			for !exit {
				res, success := rb.Pop()

				resArrLocker.Lock()
				if success {
					resArr = append(resArr, res)
				}
				if len(resArr) == totalSize {
					exit = true
				}
				resArrLocker.Unlock()
			}
			wg.Done()
		}()
	}
	wg.Wait()

	sort.Ints(resArr)
	for i := 0; i < totalSize; i++ {
		if resArr[i] != i {
			t.Fatalf("resArr[i]!=i")
		}
	}
}

func BenchmarkSpeed(b *testing.B) {
	wg := sync.WaitGroup{}
	rb := NewRingBuffer[string](1024)
	reader := 5
	sender := 2

	for i := 0; i < sender; i++ {
		wg.Add(1)
		go func() {
			for i := 0; i < b.N; i++ {
				rb.Push(fmt.Sprintf("Test String index %v", i))
			}
			wg.Done()
		}()
	}
	for i := 0; i < reader; i++ {
		wg.Add(1)
		go func() {
			for i := 0; i < b.N; i++ {
				rb.Pop()
			}
			wg.Done()
		}()
	}
	wg.Wait()
}
