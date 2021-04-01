package routego

// Logger is a simple interface used by mesh to do logging.
type Logger interface {
	Printf(format string, args ...interface{})
	Println(v ...interface{})
	PrintlnError(v ...interface{})
	PrintfError(format string, v ...interface{})
}

type NullLogger struct {
}

func (n NullLogger) Printf(format string, args ...interface{}) {
}

func (n NullLogger) Println(v ...interface{}) {
}

func (n NullLogger) PrintlnError(v ...interface{}) {
}

func (n NullLogger) PrintfError(format string, v ...interface{}) {
}
