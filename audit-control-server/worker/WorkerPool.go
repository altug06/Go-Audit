package worker

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"../Eventhandlers"
	"../Objectpool"
	"../messages"
)

var (
	lock        sync.RWMutex
	events      int
	mailClient  *Client
	processes   chan []byte
	connections chan []byte
	eventPool   *Objectpool.ReferenceCountedPool
)

type Job struct {
	Objectpool.ReferenceCounter
	JobType string
	Data    *messages.GeneralInfo
}

func AcquireEvent() *Job {
	return eventPool.Get().(*Job)
}

func NewJob(m *messages.GeneralInfo, JobType string) *Job {
	e := AcquireEvent()
	e.JobType = JobType
	e.Data = m
	return e
}

func (e *Job) Reset() {
	e.JobType = ""
	e.Data = &messages.GeneralInfo{}
}

func ResetEvent(i interface{}) error {
	obj, ok := i.(*Job)
	if !ok {
		errors.New("illegal object sent to ResetEvent")
	}
	obj.Reset()
	return nil
}

type WorkerPool struct {
	Pool       chan chan *Job
	JobQueue   chan *Job
	MaxWorkers int
}

func NewPool(maxWorkers int, queue chan *Job) *WorkerPool {
	eventPool = Objectpool.NewReferenceCountedPool(
		func(counter Objectpool.ReferenceCounter) Objectpool.ReferenceCountable {
			br := new(Job)
			br.ReferenceCounter = counter
			return br
		}, ResetEvent)

	pool := make(chan chan *Job, maxWorkers)
	return &WorkerPool{Pool: pool, JobQueue: queue, MaxWorkers: maxWorkers}
}

func LogProcesses() {
	var ProcessLog *bufio.Writer

	processlist, errP := os.OpenFile("processlist.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if errP != nil {
		log.Printf("Couldnt open/create the file: " + errP.Error())
	}

	ProcessLog = bufio.NewWriter(processlist)

	for proc := range processes {
		ProcessLog.Write(proc)

		unflushedBufferSize := ProcessLog.Buffered()
		if unflushedBufferSize >= 3500 {
			errF := ProcessLog.Flush()
			if errF != nil {
				log.Printf("couldnt write buffered process names to disk: " + errF.Error())
			}
		}
	}
}

func LogConnections() {
	var ConnectionLog *bufio.Writer

	connectionlist, errC := os.OpenFile("connectionlist.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if errC != nil {
		log.Printf("Couldnt open/create the file: " + errC.Error())
	}

	ConnectionLog = bufio.NewWriter(connectionlist)

	for con := range connections {
		ConnectionLog.Write(con)
		unflushedBufferSize := ConnectionLog.Buffered()
		if unflushedBufferSize >= 250 {
			errF := ConnectionLog.Flush()
			if errF != nil {
				log.Printf("couldnt write buffered connection names to disk: " + errF.Error())
			}
		}
	}
}

func (p *WorkerPool) InitializeWorkers() {
	mailClient = NewClient("test", "https")

	processes = make(chan []byte)
	connections = make(chan []byte)

	for i := 0; i < p.MaxWorkers; i++ {
		worker := NewWorker(p.Pool)
		worker.Start()
	}

	go LogProcesses()
	go LogConnections()
	go p.ExecuteQueue()
}

func (p *WorkerPool) ExecuteQueue() {
	for {
		select {
		case job := <-p.JobQueue:
			go func(job *Job) {
				worker := <-p.Pool
				worker <- job
			}(job)
		}
	}
}

type Worker struct {
	WorkerPool  chan chan *Job
	JobChannel  chan *Job
	QuitChannel chan bool
}

func ConvertToString(s interface{}) (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(data), nil

}

func SendLogToKibana(data *messages.GeneralInfo) {
	conn, err := pool.Get(3 * time.Second)
	if err != nil {
		log.Printf("Couldnt get a new connection")
	}

	if d, errS := ConvertToString(data); errS == nil {
		fmt.Fprintf(conn.Sock, d)
		errC := conn.Close()
		if errC != nil {
			log.Printf("Couldnt put socket back into pool: " + errC.Error())
		}
		return
	} else {
		log.Printf("couldnt convert data to string: " + errS.Error())
	}

}

func (w *Worker) Start() {
	go func() {
		for {
			w.WorkerPool <- w.JobChannel
			select {
			case job := <-w.JobChannel:
				//log.Printf("new event has arrived")
				for k, v := range job.Data.Data {
					if k == "SYSCALL" {
						var sys Eventhandlers.Syscall
						err := json.Unmarshal(*v, &sys)
						if err != nil {
							log.Printf("unmarshall error: " + err.Error())
						}
						switch sys.Syscall {
						case "execve":
							if _, ok := job.Data.Data["EXECVE"]; ok {
								job.Data.Message = sys.AnalyzeExecve(job.Data.HostName, job.Data.Data["EXECVE"], processes)
								//job.Data.Message = sys.AnalyzeExecve(job.Data.HostName ,job.Data.Data["EXECVE"], job.MessageQueue)
								// _, err := mailClient.SendMail(job.Data.Message, job.Data.HostName ,"server.php", false, "POST")
								// if err != nil{
								// 	log.Println("couldnt send email: %v", err)
								// }
								SendLogToKibana(job.Data)
							}

						case "clone":
						case "connect":
							if _, ok := job.Data.Data["SOCKADDR"]; ok {
								job.Data.Message = sys.AnalyzeConnect(job.Data.Data["SOCKADDR"], job.Data.HostName, connections)
								//job.Data.Message = sys.AnalyzeConnect(job.Data.Data["SOCKADDR"], job.Data.HostName, job.MessageQueue)
								if job.Data.Message != "" {
									SendLogToKibana(job.Data)
								}
							}
						case "open":
							job.Data.Message = sys.AnalyzeOpen(job.Data.Data["PATH"], job.Data.HostName)
							SendLogToKibana(job.Data)
						}
						break
					} else if k == "USER_LOGIN" {
						var login Eventhandlers.Sshlogin
						err := json.Unmarshal(*v, &login)
						if err != nil {
							log.Printf("Unmarshall error: " + err.Error())
						}

						job.Data.Message = login.AnalyzeLogin(job.Data.HostName)
						SendLogToKibana(job.Data)
						break
					}
				}
				job.DecrementReferenceCount()

			case <-w.QuitChannel:
				return
			}
		}
	}()
}

func (w *Worker) Stop() {
	go func() {
		w.QuitChannel <- true
	}()
}

func NewWorker(Pool chan chan *Job) *Worker {
	return &Worker{WorkerPool: Pool, JobChannel: make(chan *Job), QuitChannel: make(chan bool)}
}
