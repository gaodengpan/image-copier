package use_cases

type Logger interface {
	Infof(format string, args ...interface{})
	Debugf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Warn(args ...interface{})
	Info(args ...interface{})
}
