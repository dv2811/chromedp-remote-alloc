# Chrome Cookie Persistence Test

This project demonstrates and tests the solution to [chromedp issue #1411](https://github.com/chromedp/chromedp/issues/1411) - ensuring cookies are properly saved and loaded when using Chrome with the `--user-data-dir` flag.

## Problem Statement

When using chromedp with a custom user data directory (`--user-data-dir`), cookies and other browser state may not persist between sessions if Chrome is not shut down gracefully. This can cause:
- Loss of authentication cookies
- Resetting of user preferences
- Failed automation workflows that depend on session state

## Solution

This implementation provides a `RemoteAllocator` wrapper that:
1. Starts Chrome with remote debugging enabled
2. Connects chromedp via the Chrome DevTools Protocol
3. **Gracefully shuts down Chrome** to ensure profile data is written to disk
4. Uses `sync.Once` to prevent multiple cleanup calls
5. Waits 3 seconds after canceling the context before terminating Chrome (critical for data sync)

## Project Structure

```
.
|__ main.go          # Debugger implementation and example usage
|__ Dockerfile       # Container setup using headless-shell
|__ README.md        # This file
|__ allocator/       # allocator package
   |__ debug.go      # RemoteAllocator and launch command wrapper
   |__ debug_test.go # Test suite validating cookie persistence
```

## Key Components

### `RemoteAllocator` (allocator/debug.go)

The `RemoteAllocator` struct manages the Chrome lifecycle:

```go
type RemoteAllocator struct {
   logger      *slog.Logger
   command     *exec.Cmd
   config      *AllocatorConfig
}
```

**Key method:** `Start()` returns:
- A chromedp remote context
- A cleanup function (idempotent, safe to call multiple times)
- An error if Chrome fails to start

### Cookie Persistence Test (debug_test.go)

Mock HTTP server that:
1. Checks for the presence of `session_id` cookie
2. If no cookie exists: sets the cookie and returns empty message
3. If cookie exists: returns "cookie exists" message

This allows validation of cookie persistence across browser sessions.

The test validates the solution by:

1. **First Session:**
   - Starts Chrome with user data directory
   - Navigates to test server
   - Server sets a cookie
   - Chrome shuts down gracefully (profile saved)

2. **Second Session:**
   - Starts Chrome again with same user data directory
   - Navigates to test server
   - Server checks for cookie
   - **Expected result:** Cookie exists (loaded from saved profile)

If the cookie is found in the second session, it proves cookies persist between Chrome instances using the same profile directory.

## Usage

### Running the Example

```bash
go run main.go
```

This starts Chrome, navigates to example.com, and demonstrates the cleanup mechanism.

### Running the Test Locally

```bash
go test -v
```

Flags:
- `--headless-mode=true/false` - Run Chrome in headless mode (default: true)
- `--user-data-dir=<path>` - Chrome profile directory (default: `/home/user/.config/chromium`)

### Running in Docker

```bash
docker build -t chrome-cookie-test .
docker run --rm chrome-cookie-test
```

The Docker setup uses `chromedp/headless-shell` as the base image for minimal size.

## Critical Implementation Details

### Graceful Shutdown Sequence

```go
// 1. Cancel chromedp context (allows Chrome to save state)
remoteCancel()

// 2. Wait for profile data to be written to disk
time.Sleep(3 * time.Second)

// 3. Send SIGTERM to Chrome process
cmd.Process.Signal(syscall.SIGTERM)

// 4. Wait up to 5 seconds for graceful exit
// 5. Force kill if necessary
```

### Required Chrome Flags for Docker

```
--no-sandbox              # Required in containerized environments
--disable-dev-shm-usage   # Prevents shared memory issues in Docker
--headless                # Run without display server
--remote-debugging-port   # Enable DevTools Protocol connection
--user-data-dir           # Specify profile directory for persistence
```

### Connection Retry Logic

The implementation retries both:
- Chrome debug endpoint availability (30 attempts over 15 seconds)
- chromedp allocator connection (3 attempts with 2-second delays)

This handles timing issues in containerized environments where Chrome may take longer to initialize.

## Testing Methodology

The test validates cookie persistence by:
- Running two **completely separate Chrome sessions**
- Each session starts Chrome, performs actions, then fully shuts down
- The second session reads from the profile saved by the first session
- Success = cookie from first session is available in second session

This proves the graceful shutdown and profile sync mechanism works correctly.

## References

- [chromedp issue #1411](https://github.com/chromedp/chromedp/issues/1411) - Original issue about cookie persistence
- [chromedp documentation](https://github.com/chromedp/chromedp)
- [Chrome DevTools Protocol](https://chromedevtools.github.io/devtools-protocol/)
