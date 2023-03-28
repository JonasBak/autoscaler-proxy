package utils

import (
	"github.com/sirupsen/logrus"
)

var l = logrus.New()
var log = l.WithFields(logrus.Fields{})

func init() {
	l.SetLevel(logrus.DebugLevel)
}

func Logger() *logrus.Entry {
	return log
}
