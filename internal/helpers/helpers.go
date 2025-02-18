package helpers

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
)

// check if app process is running already
func IsAppAlreadyRunning(psName string) (bool, error) {
	var (
		// normal command output
		cmdOutput []byte
		// command error output
		errOutput error
		// output error compiled
		errOutCompiled error = fmt.Errorf("failed to get output of process list: OS=%s:\n\t%v", runtime.GOOS, errOutput)
	)

	// different methods for different OS
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("powershell", "Get-Process", psName)
		cmdOutput, errOutput = cmd.Output()
		if errOutput != nil {
			return false, errOutCompiled
		}
	case "linux":
		cmd := exec.Command("ps", "-C", psName)
		cmdOutput, errOutput = cmd.Output()
		if errOutput != nil {
			return false, errOutCompiled
		}
	default:
		return false, fmt.Errorf("platform doesn't supported: %s", runtime.GOOS)
	}

	// searching if there are more than one process of running app(based on psName)
	searchProcessRegexp := regexp.MustCompile(psName)
	searchProcessResult := searchProcessRegexp.FindAll(cmdOutput, -1)
	if len(searchProcessResult) > 1 {
		return true, nil
	}

	return false, nil
}
