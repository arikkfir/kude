package main

import (
	"fmt"
	"github.com/arikkfir/kude/internal"
	"github.com/jessevdk/go-flags"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"sigs.k8s.io/kustomize/kyaml/kio"
)

type LogConfig struct {
	CallerInfo bool   `env:"CALLER_INFO" short:"c" long:"caller-info" required:"false" description:"Show caller information"`
	Level      string `env:"LEVEL" short:"l" long:"level" description:"Log level"`
}

type CLI struct {
	Log LogConfig `group:"log" namespace:"log" env-namespace:"LOG" description:"Logging configuration"`
}

func parseCLI() CLI {
	cfg := CLI{
		Log: LogConfig{
			CallerInfo: false,
			Level:      "info",
		},
	}
	parser := flags.NewParser(&cfg, flags.Default)
	parser.NamespaceDelimiter = "-"
	if _, err := parser.Parse(); err != nil {
		if flags.WroteHelp(err) {
			os.Exit(0)
		} else {
			logrus.WithError(err).Fatal("Configuration error")
		}
	}
	return cfg
}

func ConfigureLogging(reportCallerInfo bool, level logrus.Level) {
	logger := logrus.StandardLogger()
	logger.SetLevel(level)
	logger.SetOutput(os.Stderr)
	logger.SetReportCaller(reportCallerInfo)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp:       true,
		DisableLevelTruncation: true,
		PadLevelText:           true,
	})

	// Redirect os.Stderr to logrus root logger (INFO level)
	logrusWriter := logrus.StandardLogger().WriterLevel(logrus.InfoLevel)
	pipeReader, pipeWriter, err := os.Pipe()
	if err != nil {
		logrus.WithError(err).Fatal("Failed creating pipe for stderr->logrus")
	}
	go func(w *io.PipeWriter, r *os.File) {
		_, err := io.Copy(w, r)
		if err != nil {
			logrus.WithError(err).Fatal("Failed piping stderr->logrus")
		}
	}(logrusWriter, pipeReader)
	os.Stderr = pipeWriter
}

func main() {
	cfg := parseCLI()

	// Initialize logging
	level, err := logrus.ParseLevel(cfg.Log.Level)
	if err != nil {
		logrus.WithError(err).Fatal("Invalid log level")
	}
	ConfigureLogging(cfg.Log.CallerInfo, level)

	// Configured!
	logrus.WithField("config", fmt.Sprintf("%+v", &cfg)).Debug("Configured")

	// Read pipeline
	pwd, err := os.Getwd()
	if err != nil {
		logrus.WithError(err).Fatal("Failed to read current working directory")
	}
	pipeline, err := internal.BuildPipeline(pwd, kio.ByteWriter{Writer: os.Stdout})
	if err != nil {
		logrus.WithError(err).Fatal("Failed to build pipeline")
	}

	// Execute pipeline
	if err := pipeline.Execute(); err != nil {
		logrus.WithError(err).Fatal("Pipeline failed")
	}
}
