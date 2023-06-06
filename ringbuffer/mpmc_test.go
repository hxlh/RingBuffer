package ringbuffer

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// 使用-race参数会发生读写冲突，原因是持有旧readIndex的reader访问了旧值，
// 而此时writer正在写该位置。但实际并不会造成错误，因为持有旧readIndex的
// reader将在之后的cas操作中失败，并不会返回未定义的旧值。

func TestNormal(t *testing.T) {
	t.Skip()
	sendNum := 128
	rb := NewRingBuffer[int](uint64(sendNum))

	for i := 0; i < sendNum; i++ {
		rb.Push(i)
	}

	recvNum := 0
	for i := 0; i < 512; i++ {
		e, success := rb.Pop()
		if success {
			if e != recvNum {
				t.Fatalf("e!=%v", i)
			}
			recvNum++
			if recvNum == sendNum {
				break
			}
		}
	}
}

// 测试index发生uint64回环的情况
func NewRingBuffer2[T any](capacity uint64) *RingBuffer[T] {
	return &RingBuffer[T]{
		size:         0,
		capacity:     capacity,
		writeIndex:   math.MaxUint64 - 5,
		readIndex:    math.MaxUint64 - 5,
		maxReadIndex: math.MaxUint64 - 5,
		element:      make([]T, capacity),
	}
}
func TestValidity(t *testing.T) {
	t.Skip()
	// config
	rb := NewRingBuffer2[int](16)
	reader := 6
	sender := 3
	dataSize := 2000 * 10000

	// init
	wg := sync.WaitGroup{}
	dataRange := make([][]int, 0, sender)
	fragSize := dataSize / sender
	totalSize := fragSize * sender
	resArr := make([]int, 0, totalSize)
	resArrLocker := sync.Mutex{}

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
			fmt.Println("sender end")
			wg.Done()
		}(dataRange[i][0], dataRange[i][1])
	}

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
			fmt.Println("read end")
		}()
	}
	wg.Wait()

	sort.Ints(resArr)
	for i := 0; i < totalSize; i++ {
		if resArr[i] != i {
			t.Fatalf("resArr[i]!=i=>%v!=%v", resArr[i], i)
		}
	}
}

func LockFree(bufSize,dataSize,sender,reader int)  {
	rb := NewRingBuffer2[int](uint64(bufSize))

	// init
	wg := sync.WaitGroup{}
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

	recved := uint64(0)

	for i := 0; i < reader; i++ {
		wg.Add(1)
		go func() {
			exit := false
			for !exit {
				_, success := rb.Pop()

				if success {
					atomic.AddUint64(&recved, 1)
				}
				if int(atomic.LoadUint64(&recved)) == totalSize {
					exit = true
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func Chan(bufSize,dataSize,sender,reader int)  {

	// init
	wg := sync.WaitGroup{}
	dataRange := make([][]int, 0, sender)
	fragSize := dataSize / sender
	totalSize := fragSize * sender

	ch:=make(chan int,bufSize)

	start := 0
	for i := 0; i < sender; i++ {
		dataRange = append(dataRange, []int{start, start + fragSize})
		start += fragSize
	}


	for i := 0; i < sender; i++ {
		wg.Add(1)
		go func(start, end int) {
			for j := start; j < end; j++ {
				ch<-j
			}
			wg.Done()
		}(dataRange[i][0], dataRange[i][1])
	}

	recved := uint64(0)

	for i := 0; i < reader; i++ {
		wg.Add(1)
		go func() {
			exit := false
			for !exit {
				n,ok:=<-ch
				_=n
				if !ok{
					wg.Done()
					return
				}
				new:=atomic.AddUint64(&recved,1)
				if new==uint64(totalSize){
					break
				}
			}
			close(ch)
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestCompare(t *testing.T) {
	bufSize:=32
	dataSize:=10000*10000

	start:=time.Now()
	LockFree(bufSize,dataSize,2,5)
	dur:=time.Since(start)
	fmt.Printf("LockFree use time %v ms\n",dur.Milliseconds())

	start=time.Now()
	Chan(bufSize,dataSize,2,5)
	dur=time.Since(start)
	fmt.Printf("chan use time %v ms\n",dur.Milliseconds())
}


