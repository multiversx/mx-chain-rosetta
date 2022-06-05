package main

import (
	"fmt"
	"io"
	"os"
	"time"

	logger "github.com/ElrondNetwork/elrond-go-logger"
	"github.com/ElrondNetwork/elrond-go/core/logging"
	"github.com/urfave/cli"
)

func initializeLogger(ctx *cli.Context) (io.Closer, error) {
	logLevelFlagValue := ctx.GlobalString(logLevel.Name)
	err := logger.SetLogLevel(logLevelFlagValue)
	if err != nil {
		return nil, err
	}

	folder, err := getLogsFolder(ctx)
	if err != nil {
		return nil, err
	}

	fileLogging, err := logging.NewFileLogging(folder, defaultLogsPath, logFilePrefix)
	if err != nil {
		return nil, fmt.Errorf("%w creating a log file", err)
	}

	err = fileLogging.ChangeFileLifeSpan(time.Second * time.Duration(logFileLifeSpanInSec))
	if err != nil {
		return nil, err
	}

	return fileLogging, nil
}

func getLogsFolder(ctx *cli.Context) (string, error) {
	if ctx.IsSet(logsFolder.Name) {
		return ctx.GlobalString(logsFolder.Name), nil
	}

	return os.Getwd()
}
