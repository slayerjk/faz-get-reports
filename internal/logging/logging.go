package logging

import (
	"fmt"
	"log"
	"os"
	"time"
)

func StartLogging(appName, logDirPath string) error {
	// TODO: logname with current date
	timeNow := time.Now().Format("02.01.2006")

	// create log dir
	if err := os.MkdirAll(logDirPath, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create log dir %s:\n\t%v", logDirPath, err)
	}

	// create log file
	logFileName := fmt.Sprintf("%s_%s.log", appName, timeNow)
	logFilePath := fmt.Sprintf("%s/%s", logDirPath, logFileName)

	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open created log file %s:\n\t%v", logFilePath, err)
	}
	log.SetOutput(file)
	defer file.Close()

	return nil
}
