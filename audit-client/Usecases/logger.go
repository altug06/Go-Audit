package Usecases


import(
	"log"
	"io"
)

var(
	WARN = "WARNING"
	INFO = "INFO"
	ERROR = "ERROR"
)

type Log struct{
	Info    *log.Logger
    Warning *log.Logger
	Error   *log.Logger
}


func LoggerInit(infoHandle io.Writer, warningHandle io.Writer, errorHandle io.Writer) Log{
	return Log{
				Info: log.New(infoHandle, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
				Warning: log.New(warningHandle, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile),
				Error: log.New(warningHandle, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
	}
}

func (l *Log)Log(message string, Type string){
	switch Type{
	case WARN:
			l.Warning.Println(message)
	case ERROR:
			l.Error.Println(message)
	case INFO:
			l.Info.Println(message)
	}
}
