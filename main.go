package main

import (
	"context"
	"log"
	"fmt"
	"os"

	"time"
	"github.com/chromedp/chromedp"

	"main/allocator"
)

func main() {
	userDataDir := os.Getenv("USER_DATA_DIR")

	RemoteAllocator := allocator.NewRemoteAllocator(nil,
		allocator.RemoteDebuggingPort("9222"),
		allocator.WithUserDataDir(userDataDir),
	)

	// Start Chrome and get cleanup function
	remoteCtx, cleanup, err := RemoteAllocator.Start()
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	// Create context for automation
	ctx, cancel := chromedp.NewContext(remoteCtx)
	defer cancel()

	// Set timeout
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, time.Second*60)
	defer timeoutCancel()

	// Example automation flow
	var body string
	err = chromedp.Run(timeoutCtx,
		chromedp.Navigate("https://example.com/"),
		chromedp.InnerHTML(`body`, &body, chromedp.ByQuery),
		chromedp.Sleep(time.Second*10),
	)

	if err != nil {
		log.Fatalf("Error during automation: %v", err)
	}

	fmt.Println("Page body:", body)
	fmt.Println("✓ Automation completed successfully!")
}