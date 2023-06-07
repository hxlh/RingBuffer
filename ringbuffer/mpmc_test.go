package mpmc

import (
	"fmt"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hedzr/go-ringbuf/v2"
)

func TestNormal(t *testing.T) {
	// t.Skip()
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

func TestValidity(t *testing.T) {
	// config
	rb := NewRingBuffer2[int](1024)
	reader := 6
	sender := 3
	dataSize := 5000 * 10000

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

	timeStart := time.Now()

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

	fmt.Printf("耗时：%v ms\n", time.Since(timeStart).Milliseconds())

	sort.Ints(resArr)
	for i := 0; i < totalSize; i++ {
		if resArr[i] != i {
			t.Fatalf("resArr[i]!=i=>%v!=%v", resArr[i], i)
		}
	}
}

func LockFree(bufSize, dataSize, sender, reader int) {
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
					runtime.Gosched()
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
				} else {
					runtime.Gosched()
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

func Chan(bufSize, dataSize, sender, reader int) {

	// init
	wg := sync.WaitGroup{}
	dataRange := make([][]int, 0, sender)
	fragSize := dataSize / sender
	totalSize := fragSize * sender

	ch := make(chan int, bufSize)

	start := 0
	for i := 0; i < sender; i++ {
		dataRange = append(dataRange, []int{start, start + fragSize})
		start += fragSize
	}

	for i := 0; i < sender; i++ {
		wg.Add(1)
		go func(start, end int) {
			for j := start; j < end; j++ {
				ch <- j
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
				_, ok := <-ch
				if !ok {
					wg.Done()
					return
				}
				new := atomic.AddUint64(&recved, 1)
				if new == uint64(totalSize) {
					exit = true
				}
			}
			close(ch)
			wg.Done()
		}()
	}
	wg.Wait()
}

func ThirtyPart(bufSize, dataSize, sender, reader int) {
	rb := ringbuf.New[int](uint32(bufSize))
	rb.Size()
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
				for {
					if err := rb.Enqueue(j); err == nil {
						break
					}
					runtime.Gosched()
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
				_, err := rb.Dequeue()
				if err == nil {
					atomic.AddUint64(&recved, 1)
				} else {
					runtime.Gosched()
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

func TestThirtyPartRingBufferValidity(t *testing.T) {
	// config
	rb := ringbuf.New[int](1024)
	reader := 6
	sender := 3
	dataSize := 5000 * 10000

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

	timeStart := time.Now()

	for i := 0; i < sender; i++ {
		wg.Add(1)
		go func(start, end int) {
			for j := start; j < end; j++ {
				for {
					if err := rb.Enqueue(j); err == nil {
						break
					}
					runtime.Gosched()
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
				res, success := rb.Dequeue()

				resArrLocker.Lock()
				if success == nil {
					resArr = append(resArr, res)
				} else {
					runtime.Gosched()
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

	fmt.Printf("耗时：%v ms\n", time.Since(timeStart).Milliseconds())

	sort.Ints(resArr)
	for i := 0; i < totalSize; i++ {
		if resArr[i] != i {
			t.Fatalf("resArr[i]!=i=>%v!=%v", resArr[i], i)
		}
	}
}

func TestCompareMyRingBufferAndChan(t *testing.T) {
	bufSize := 4096
	dataSize := 1000 * 10000
	sender := 7
	reader := 7
	start := time.Now()
	LockFree(bufSize, dataSize, sender, reader)
	dur := time.Since(start)
	fmt.Printf("LockFree use time %v ms\n", dur.Milliseconds())

	start = time.Now()
	Chan(bufSize, dataSize, sender, reader)
	dur = time.Since(start)
	fmt.Printf("chan use time %v ms\n", dur.Milliseconds())
}

func TestCompareThirtyPartRingBufferAndChan(t *testing.T) {
	bufSize := 4096
	dataSize := 1000 * 10000
	sender := 7
	reader := 7

	start := time.Now()
	ThirtyPart(bufSize, dataSize, sender, reader)
	dur := time.Since(start)
	fmt.Printf("ThirtyPart use time %v ms\n", dur.Milliseconds())

	start = time.Now()
	Chan(bufSize, dataSize, sender, reader)
	dur = time.Since(start)
	fmt.Printf("chan use time %v ms\n", dur.Milliseconds())
}

func TestCompareMyRingBufferAndThirtyPartRingBuffer(t *testing.T) {
	bufSize := 4096
	dataSize := 1000 * 10000
	sender := 7
	reader := 7

	start := time.Now()
	ThirtyPart(bufSize, dataSize, sender, reader)
	dur := time.Since(start)
	fmt.Printf("ThirtyPartRingBuffer use time %v ms\n", dur.Milliseconds())

	start = time.Now()
	LockFree(bufSize, dataSize, sender, reader)
	dur = time.Since(start)
	fmt.Printf("MyRingBuffer use time %v ms\n", dur.Milliseconds())
}
