package logger

// Thanks to https://github.com/onrik/gorm-logrus/blob/master/logger.go
// This is a slightly adapted version of it. It filters password and token columns.
// This should be improved.

import (
	"context"
	"errors"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

type logger struct {
	SlowThreshold         time.Duration
	SourceField           string
	SkipErrRecordNotFound bool
	LogQuery              bool
}

type LoggerConfig struct {
	SlowThreshold         time.Duration
	SourceField           string
	SkipErrRecordNotFound bool
	LogQuery              bool
}

func New(initLogger LoggerConfig) *logger {
	return &logger{
		SlowThreshold:         initLogger.SlowThreshold,
		SkipErrRecordNotFound: initLogger.SkipErrRecordNotFound,
		LogQuery:              initLogger.LogQuery,
	}
}

func (l *logger) LogMode(gormlogger.LogLevel) gormlogger.Interface {
	return l
}

func (l *logger) Info(ctx context.Context, s string, args ...interface{}) {
	log.WithContext(ctx).Infof(s, args...)
}

func (l *logger) Warn(ctx context.Context, s string, args ...interface{}) {
	log.WithContext(ctx).Warnf(s, args...)
}

func (l *logger) Error(ctx context.Context, s string, args ...interface{}) {
	log.WithContext(ctx).Errorf(s, args...)
}

func (l *logger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	sql, _ := fc()
	fields := log.Fields{}

	// We need to replace some confident parts of the sql
	m1 := regexp.MustCompile(`.?token\_.?.?=.?\'(.*)\'`)
	sql = m1.ReplaceAllString(sql, "t***")
	m2 := regexp.MustCompile(`.?password.?=.?\'(.*)\'`)
	sql = m2.ReplaceAllString(sql, "p***")

	if l.SourceField != "" {
		fields[l.SourceField] = utils.FileWithLineNum()
	}
	if err != nil && !(errors.Is(err, gorm.ErrRecordNotFound) && l.SkipErrRecordNotFound) {
		fields[log.ErrorKey] = err
		log.WithContext(ctx).WithFields(fields).Errorf("%s [%s]", sql, elapsed)
		return
	}

	if l.SlowThreshold != 0 && elapsed > l.SlowThreshold {
		log.WithContext(ctx).WithFields(fields).Warnf("%s [%s]", sql, elapsed)
		return
	}

	if l.LogQuery {
		log.WithContext(ctx).WithFields(fields).Infof("%s [%s]", sql, elapsed)
		return
	}

	log.WithContext(ctx).WithFields(fields).Debugf("%s [%s]", sql, elapsed)
}
