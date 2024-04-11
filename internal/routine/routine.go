package routine

import (
	"context"
	"log"
	"sync"
)

type Routine func(context.Context) error

// Run all of the passed in functions as go routines
// Returns once all functions are complete, an error is encountered,
// or the context is cancelled
func RunAll(ctx context.Context, fns ...Routine) error {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)

	defer func() {
		// Any failing task will cause this function to return.
		// Cancel context and wait to ensure all remaining tasks
		// are able to respond to the context Done channel and
		// perform any relevant shutdown steps
		cancel()
		wg.Wait()
	}()

	wg.Add(len(fns))

	errch := make(chan error, len(fns))

	for _, fn := range fns {
		fn := fn
		go func() {
			defer func() { wg.Done() }()
			errch <- fn(ctx)
		}()
	}

	for complete := 0; complete < len(fns); {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errch:
			if err != nil {
				return err
			}
			complete++
		}
	}

	return nil
}

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
