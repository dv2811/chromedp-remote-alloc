package allocator

import (
	"log/slog"
	"net/http"
	"context"
	"errors"
	"fmt"
	
	"os/signal"
	"os/exec"
	"path/filepath"
	"os"

	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/chromedp/chromedp"
)

var (
	ErrChromiumPathNotFound = errors.New("allocator: chromium based browser executable path not detected")
	ErrMaxConnectAttempts 	= errors.New("max debug connection attempts exceeded")
	ErrMaxStartAttempts     = errors.New("max start attempts exceeded")
)

type RemoteAllocator struct {
	// debugPort   string
	// userDataDir string
	// headless	bool
	logger      *slog.Logger
	command     *exec.Cmd
	config		*AllocatorConfig
}

// func NewRemoteAllocator(debugPort string, userDataDir string, headless bool, logger *slog.Logger) *RemoteAllocator {
func NewRemoteAllocator(logger *slog.Logger, options ...AllocatorOption) *RemoteAllocator {
	if logger == nil {
		logger = slog.Default()
	}
	
	config := &AllocatorConfig{}
	for _, opt := range options {
		opt(config)
	}

	if config.userDataDir == "" {
		config.userDataDir = filepath.Join(os.TempDir(), "chromium-data")
	}

	if config.debugPort == "" {
		config.debugPort = "9222"
	}

	return &RemoteAllocator{
		config:      config,
		logger:      logger,
		// debugPort:   debugPort,
		// userDataDir: userDataDir,
		// headless:    headless,
	}
}

// StartRemoteContext starts Chrome and returns context + cleanup function
// The cleanup function can be safely called multiple times (idempotent)
// Usage: remoteCtx, cleanup, err := RemoteAllocator.StartRemoteContext()
//        defer cleanup()
func (d *RemoteAllocator) Start() (context.Context, func(), error) {
	// os.MkdirAll(d.userDataDir, 0755)
	// os.Remove(filepath.Join(d.userDataDir, "SingletonLock"))
	chromeExecPath := getChromePath()
	if chromeExecPath == "" {
		chromeExecPath = getChromePath()
	}
	
	args := []string{
		fmt.Sprintf("--user-data-dir=%s", d.config.userDataDir),
		fmt.Sprintf("--remote-debugging-port=%s", d.config.debugPort),
		"--no-first-run",
		"--no-default-browser-check",
		"--enable-unsafe-swiftshader",
		"--disable-blink-features=AutomationControlled",
		"--disable-features=IsolateOrigins,site-per-process",
		"--no-sandbox", // applies to Docker environemnt where chromium is run as root
		"--disable-dev-shm-usage",
	}
	if d.config.headless {
		args = append(args, "--headless")
	}

	cmd := exec.Command(chromeExecPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		d.logger.Error("failed to start chrome", "error", err)
		return nil, nil, fmt.Errorf("failed to start chrome: %w", err)
	}

	d.logger.Info(fmt.Sprintf("Chrome started with PID: %d", cmd.Process.Pid))
	d.command = cmd
	debugURL := fmt.Sprintf("http://localhost:%s", d.config.debugPort)

	// Wait for Chrome to be ready
	debugURLRetry := 0
	for ; debugURLRetry < 30; debugURLRetry++ {
		resp, err := http.Get(fmt.Sprintf("%s/json/version", debugURL))
		if err == nil {
			resp.Body.Close()
			d.logger.Info("Chrome is ready")
			break
		}
		time.Sleep(time.Millisecond*500)
	}

	if debugURLRetry == 30 {
		d.logger.Error("failed to connect to Chrome debug URL")
		cmd.Process.Kill()
		return nil, nil, ErrMaxConnectAttempts
	}

	// Connect with retries
	var remoteCtx context.Context
	var remoteCancel context.CancelFunc
	allocateCount := 0
	for ; allocateCount < 3; allocateCount++ {
		remoteCtx, remoteCancel = chromedp.NewRemoteAllocator(context.Background(), debugURL)

		testCtx, testCancel := chromedp.NewContext(remoteCtx)
		err := chromedp.Run(testCtx, chromedp.Navigate("about:blank"))
		testCancel()

		if err == nil {
			d.logger.Info("Successfully connected to Chrome")
			break
		}

		d.logger.Warn(fmt.Sprintf("Connection attempt %d failed: %v", allocateCount+1, err))
		remoteCancel()

		if allocateCount < 2 {
			time.Sleep(2 * time.Second)
		}
	}

	if allocateCount == 3 {
		d.logger.Error("failed to start remote allocator after all attempts")
		cmd.Process.Kill()
		return nil, nil, ErrMaxStartAttempts
	}

	// Use sync.Once to ensure cleanup runs exactly once
	var cleanupOnce sync.Once

	// Cleanup function that handles graceful shutdown
	cleanup := func() {
		cleanupOnce.Do(func() {
			d.logger.Info("Starting cleanup...")

			// Cancel the remote context first - let chromedp save state
			// Using chromedp.Cancel could be more robust. See https://pkg.go.dev/github.com/chromedp/chromedp#Cancel
			remoteCancel()

			// CRITICAL: Wait for Chrome to save profile data (issue #1411)
			d.logger.Info("Waiting for Chrome to save profile data...")
			time.Sleep(3 * time.Second)

			// Shutdown Chrome process gracefully
			d.logger.Info("Shutting down Chrome process...")
			if d.command != nil && d.command.Process != nil {
				if err := d.command.Process.Signal(syscall.SIGTERM); err != nil {
					d.logger.Warn("Failed to send SIGTERM", "error", err)
				}

				done := make(chan error, 1)
				go func() {
					done <- d.command.Wait()
				}()

				select {
				case err := <-done:
					if err != nil {
						d.logger.Info("Chrome exited", "error", err)
					} else {
						d.logger.Info("Chrome stopped gracefully")
					}
				case <-time.After(5 * time.Second):
					d.logger.Warn("Chrome didn't stop in time, forcing kill...")
					d.command.Process.Kill()
					<-done
					d.logger.Info("Chrome killed")
				}
			}

			d.logger.Info("Cleanup complete")
		})
	}

	// Handle signals in background
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		sig := <-quit
		d.logger.Info(fmt.Sprintf("Received signal: %v", sig))
		cleanup()
		os.Exit(0)
	}()

	return remoteCtx, cleanup, nil
}

func getChromePath() string {
	paths := []string{
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
		"/headless-shell/headless-shell",
	}

	if runtime.GOOS == "darwin" {
		paths = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// iterate through possible chromium browsers
	names := []string{"chromium", "google-chrome", "microsoft-edge"}
	for _, name := range names {
		path, err := exec.LookPath(name)
		if err == nil {
			return path
		}
	}

	return "chromium"
}