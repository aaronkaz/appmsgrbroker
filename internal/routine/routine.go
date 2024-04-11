package routine

import (
	"context"
	"log"
)

type Routine func(context.Context) error

func Run(ctx context.Context, fn Routine, onDone ...func()) error {
	errChan := make(chan error)
	go func(ctx context.Context) {
		errChan <- fn(ctx)
	}(ctx)

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		log.Println("context cancelled...wait a sec...")
		for _, op := range onDone {
			op()
		}
		<-errChan
		return nil
	}
}
