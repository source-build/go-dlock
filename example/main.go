package main

import (
	"fmt"
	go_dlock "github.com/source-build/go-dlock"
	"log"
	"sync"
	"time"
)

var wg sync.WaitGroup

func main() {
	wg.Add(2)
	go func() {
		defer wg.Done()
		l := go_dlock.NewDLock("192.168.1.5:7668", "xxx")
		fmt.Println("g1 await lock")
		if err := l.Lock("2312"); err != nil {
			log.Println("failed!", err)
			return
		}
		defer l.UnLock()
		fmt.Println("g1 lock")
		time.Sleep(time.Second * 5)
		fmt.Println("g1 unlock")
	}()

	go func() {
		defer wg.Done()
		l := go_dlock.NewDLock("192.168.1.5:7668", "xxx")
		fmt.Println("g2 await lock")
		if err := l.Lock("2312"); err != nil {
			log.Println("failed!", err)
			return
		}
		defer l.UnLock()
		fmt.Println("g2 lock")
		time.Sleep(time.Second * 5)
		fmt.Println("g2 unlock")
	}()
	wg.Wait()
}
