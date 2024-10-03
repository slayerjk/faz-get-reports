package logging

import (
	"fmt"
	"log"
	"os"
	"time"
)

// Start logging via log package; creates log file as <fileName>_<mm.dd.YYYY>.log
func StartLogging(fileName, logDirPath string, logsToKeep int) (*os.File, error) {
	// create log dir
	if err := os.MkdirAll(logDirPath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create log dir %s:\n\t%v", logDirPath, err)
	}

	// set current time
	timeNow := time.Now().Format("02.01.2006")

	// create log file
	logFilePath := fmt.Sprintf("%s/%s_%s.log", logDirPath, fileName, timeNow)

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open created log file %s:\n\t%v", logFilePath, err)
	}
	log.SetOutput(logFile)

	return logFile, nil
}
