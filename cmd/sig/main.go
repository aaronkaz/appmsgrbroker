package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"time"
)

func main() {

	// sigs := make(chan os.Signal, 1)
	ctx := context.Background()

	ctx, _ = signal.NotifyContext(ctx)

	// signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan bool, 1)

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("context cancelled, wait a sec...")
				time.Sleep(3 * time.Second)
				done <- true
			}
		}
		// sig := <-sigs
		// fmt.Println()
		// fmt.Println(sig)
		// done <- true
	}()

	fmt.Println("awaiting signal")
	<-done
	fmt.Println("exiting")
}
