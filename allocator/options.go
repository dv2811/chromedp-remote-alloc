package allocator

// AllocatorConfig type contains options for allocator
type AllocatorConfig struct {
	userDataDir		string
	headless		bool
	debugPort		string
	execPath		string		
}
type AllocatorOption func(*AllocatorConfig)

// Running browser instance in headless or headful mode
func HeadlessMode(headlessMode bool) func(*AllocatorConfig) {
	return func(a *AllocatorConfig) {
		a.headless = headlessMode
	}
}

// WithUserDataDir set directory for user data. default `chromium-data` directory under os.TempDir()
func WithUserDataDir(directory string) func(*AllocatorConfig) {
	return func(a *AllocatorConfig) {
		a.userDataDir = directory
	}
}

// ExecutablePath option set location for the chromium based browser executable, RemoteAllcator struct will look up this path if not set
func ExecutablePath(execPath string) func(*AllocatorConfig) {
	return func(a *AllocatorConfig) {
		a.execPath = execPath
	}
}

// ExecutablePath option set location for the chromium based browser executable, RemoteAllcator struct will look up this path if not set
func RemoteDebuggingPort(port string) func(*AllocatorConfig) {
	return func(a *AllocatorConfig) {
		a.debugPort = port
	}
}