package allocator
import (
	"context"
	"time"
	"flag"
	"fmt"
	"testing"

	"html/template"
	"net/http"
	"bytes"

	"github.com/chromedp/chromedp"
)

var (
	userDataDir  = flag.String("user-data-dir", "/tmp/chromium-data", "chrome config path")
	headlessMode = flag.Bool("headless-mode", true, "run in headless mode or not")
	execPath     = flag.String("exec-path", "", "chrome executable path")
	debugPort    = flag.String("debug-port", "9222", "debug port")
)

type Template struct {
	Message		string
}

const htmlTemplate = `
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<body>
<!-- inform user of the existence of inbound cookie -->
<p id="notification">{{ .Message}}</p>
</body>
</html>`

var (
	testHTMLTemplate,_ = template.New("test_page").Parse(htmlTemplate)
)

func interfaceHandler(w http.ResponseWriter, r *http.Request) {
	// check existence of cookie
	cookie, err := r.Cookie("session_id")
	noCookies := err != nil || cookie == nil
	data := Template{}
	if noCookies {
		// empty cookie message	
		data.Message = "" 
		http.SetCookie(w, &http.Cookie{
			Name: "session_id",
			Value: "mock_session_cookie",
			Secure: false, 
			Expires: time.Now().Add(time.Hour*720),
			HttpOnly: true,
		})
	} else {
		data.Message = "cookie exists"
	}
	
	// write to buffer first to check for execution error
	b := new(bytes.Buffer)
	err = testHTMLTemplate.Execute(b, data)

	// write raw error
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	// if OK then serve rendered content
	w.WriteHeader(http.StatusOK)
	w.Write(b.Bytes())
}

func serveTestInterface(port string) {
	mux := http.NewServeMux()
	mux.HandleFunc(`GET /`, interfaceHandler)
	http.ListenAndServe(port, mux)
}

func getMessage(portStr string) (string, error) {
	// when using headless shell docker, headless mode is preferred as no display server
	// alternatively, we can use xvfb inside our docker image but headfull not the main focus of the test
	debugger := NewRemoteAllocator(*debugPort, *userDataDir, *headlessMode, nil)

	// Start Chrome and get cleanup function
	remoteCtx, cleanup, err := debugger.Start()
	if err != nil {
		return "", err
	}
	defer cleanup()

	// open test page
	tabCtx, tabCancel := chromedp.NewContext(remoteCtx)
	defer tabCancel()

	ctx, cancel := context.WithTimeout(tabCtx, time.Second * 60)
	defer cancel()

	var msg string
	err = chromedp.Run(ctx,
		chromedp.Navigate(fmt.Sprintf("http://localhost%s", portStr)),
		chromedp.Text(`p[id="notification"]`, &msg, chromedp.ByQuery),
		chromedp.Sleep(time.Second * 10),
	)

	if err != nil {
		return "", err
	}
	return msg, nil
}

// test cookie persistence when using chromedp to run chromium with user data dir set
func TestCookiePersistence(t *testing.T) {
	time.Sleep(2 * time.Second)

	// init test page server
	portStr := ":8080"
	// run test server
	go serveTestInterface(portStr)

	// get first message
	_, err := getMessage(portStr)
	if err != nil {
		t.Errorf("first access error %v", err)
	}

	// get second message - mock back end checks for inbound cookies
	secondMsg, err := getMessage(portStr)
	if err != nil {
		t.Errorf("second access error %v", err)
	}

	// check if cookie is saved between 2 sessions
	if secondMsg == "" {
		t.Error("cookie not saved")
	}
}