package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func BuildDevelopmentLogger() (*zap.Logger, error) {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return config.Build()
}

func BuildProductionLogger(outputFilePath string) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{outputFilePath}
	return cfg.Build()
}
