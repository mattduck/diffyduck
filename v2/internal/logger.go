package internal

import (
	"log"
	"os"
	"sync"
)

var (
	logger *log.Logger
	once   sync.Once
)

// InitLogger initializes the logger to write to /tmp/matt.log
func InitLogger() {
	once.Do(func() {
		file, err := os.OpenFile("/tmp/matt.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			// Fallback to stdout if we can't write to file
			logger = log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
			return
		}
		logger = log.New(file, "", log.LstdFlags|log.Lmicroseconds)
	})
}

// Logf logs a formatted message with timestamp
func Logf(format string, args ...interface{}) {
	if logger == nil {
		InitLogger()
	}
	logger.Printf(format, args...)
}

// Log logs a simple message with timestamp
func Log(msg string) {
	if logger == nil {
		InitLogger()
	}
	logger.Print(msg)
}
