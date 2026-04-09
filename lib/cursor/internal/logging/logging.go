package logging

import (
	"io"
	"log"
	"os"
)

var syncLogger = log.New(os.Stdout, "", 0)
var debugLogger = log.New(io.Discard, "", 0)

func Configure() {
	syncLogger.SetFlags(0)
	syncLogger.SetPrefix("")
	syncLogger.SetOutput(os.Stdout)
}

func Info(message string) {
	syncLogger.Println("INFO: " + message)
}

func Debug(message string) {
	debugLogger.Println("DEBUG: " + message)
}

func Error(message string) {
	syncLogger.Println("ERROR: " + message)
}
