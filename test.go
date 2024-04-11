package main

import (
	"context"
	"log"
	"os"
	"os/exec"
)

func main() {
	log.Println("test!")
	cmd := exec.Command("go", "run", "cmd/main.go")
	cmd.Stdout = os.Stdout

	ctx := context.Background()

	// if err := routine.RunAll(ctx, func ()  {

	// })
	// if err := cmd.Run(); err != nil {
	// 	log.Fatal(err)
	// }
}
