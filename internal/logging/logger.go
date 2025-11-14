package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/any-hub/any-hub/internal/config"
)

// InitLogger 根据全局配置初始化 JSON 结构化日志，确保文件/控制台输出一致。
func InitLogger(cfg config.GlobalConfig) (*logrus.Logger, error) {
	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("无法解析日志级别: %w", err)
	}

	output, outErr := buildOutput(cfg)
	if outErr != nil {
		fmt.Fprintf(os.Stderr, "logger_fallback: %v\n", outErr)
	}

	logger := logrus.New()
	logger.SetLevel(level)
	logger.SetOutput(output)
	logger.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339Nano})

	logrus.SetFormatter(logger.Formatter)
	logrus.SetOutput(logger.Out)
	logrus.SetLevel(logger.GetLevel())

	if outErr != nil {
		logger.WithFields(logrus.Fields{
			"action": "logger_fallback",
			"path":   cfg.LogFilePath,
		}).Warn(outErr.Error())
	}

	return logger, nil
}

// buildOutput 根据配置创建日志输出 Writer；失败时降级到 stdout 并返回错误。
func buildOutput(cfg config.GlobalConfig) (io.Writer, error) {
	if cfg.LogFilePath == "" {
		return os.Stdout, nil
	}

	dir := filepath.Dir(cfg.LogFilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return os.Stdout, fmt.Errorf("创建日志目录失败: %w", err)
	}

	rotator := &lumberjack.Logger{
		Filename:   cfg.LogFilePath,
		MaxSize:    cfg.LogMaxSize,
		MaxBackups: cfg.LogMaxBackups,
		Compress:   cfg.LogCompress,
		LocalTime:  true,
	}
	return rotator, nil
}
