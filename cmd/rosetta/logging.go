package main

import (
	"fmt"
	"io"
	"os"
	"time"

	logger "github.com/ElrondNetwork/elrond-go-logger"
	"github.com/ElrondNetwork/elrond-go/core/logging"
)

const (
	defaultLogsPath      = "logs"
	logFilePrefix        = "rosetta"
	logFileLifeSpanInSec = 86400
)

func initializeLogger(logsFolder string, logLevel string) (io.Closer, error) {
	currentDirectory, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	logsFolderNotSpecified := len(logsFolder) == 0
	if logsFolderNotSpecified {
		logsFolder = currentDirectory
	}

	err = logger.SetLogLevel(logLevel)
	if err != nil {
		return nil, err
	}

	if len(logsFolder) == 0 {
	}

	fileLogging, err := logging.NewFileLogging(logsFolder, defaultLogsPath, logFilePrefix)
	if err != nil {
		return nil, fmt.Errorf("%w creating a log file", err)
	}

	err = fileLogging.ChangeFileLifeSpan(time.Second * time.Duration(logFileLifeSpanInSec))
	if err != nil {
		return nil, err
	}

	return fileLogging, nil
}
