package Usecases

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
)

var auditlog chan []byte

type GeneralInfo struct {
	Serial   uint64
	Data     map[string]*json.RawMessage
	Hostname string
}

type Event struct {
	Data []byte
	ID   string
}

type WorkerPool struct {
	Pool       chan chan Event
	JobQueue   chan Event
	MaxWorkers int
	workers    []*Worker
}

func NewPool(maxWorkers int, queue chan Event) *WorkerPool {
	pool := make(chan chan Event, maxWorkers)
	return &WorkerPool{Pool: pool, JobQueue: queue, MaxWorkers: maxWorkers}
}

func (p *WorkerPool) ShutDown() {
	for _, w := range p.workers {
		w.Stop()
	}
}

func (p *WorkerPool) InitializeWorkers(j JobManager) {
	for i := 0; i < p.MaxWorkers; i++ {
		worker := NewWorker(p.Pool, j)
		p.workers = append(p.workers, worker)
		worker.Start()
	}
	auditlog = make(chan []byte)

	go logUnixSocket()
	go p.ExecuteQueue()
}

func (p *WorkerPool) ExecuteQueue() {
	for {
		select {
		case e := <-p.JobQueue:
			go func(e Event) {
				worker := <-p.Pool
				worker <- e
			}(e)
		}
	}
}

// func Logaudit() {
// 	var auditWriter *bufio.Writer

// 	logfile, errP := os.OpenFile("/var/log/audit-data.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
// 	if errP != nil {
// 		log.Printf("Couldnt open/create the file: " + errP.Error())
// 		return
// 	}

// 	auditWriter = bufio.NewWriter(logfile)

// 	for event := range auditlog {
// 		auditWriter.Write(event)
// 		auditWriter.Write([]byte("\n"))
// 		unflushedBufferSize := auditWriter.Buffered()
// 		if unflushedBufferSize >= 3500 {
// 			errF := auditWriter.Flush()
// 			if errF != nil {
// 				log.Printf("couldnt write buffered process names to disk: " + errF.Error())
// 			}
// 		}
// 	}
// }

func logUnixSocket() {

	var auditWriter *bufio.Writer

	c, err := net.Dial("unix", "/var/run/filebeat.sock")
	if err != nil {
		log.Printf("Couldnt open the unix socket: " + err.Error())
	}
	defer c.Close()

	auditWriter = bufio.NewWriter(c)

	for event := range auditlog {
		auditWriter.Write(event)
		auditWriter.Write([]byte("\n"))
		unflushedBufferSize := auditWriter.Buffered()
		if unflushedBufferSize >= 3500 {
			errF := auditWriter.Flush()
			if errF != nil {
				log.Printf("couldnt write buffered process names to disk: " + errF.Error())
			}
		}
	}

}

type Worker struct {
	WorkerPool  chan chan Event
	JobChannel  chan Event
	QuitChannel chan bool
	jobManager  JobManager
}

func (w *Worker) Start() {
	go func() {
		defer func() {
			fmt.Println("Worker is getting killed which is not anticipated.")
		}()
		//buf := new(bytes.Buffer)
		for {
			w.WorkerPool <- w.JobChannel
			select {
			case e := <-w.JobChannel:
				auditlog <- e.Data
				fmt.Println(string(e.Data))
				//send log to kibana
				// buf.Write(e.Data)
				// fmt.Println(buf.Len())
				// fmt.Println(string(e.Data))
				// if buf.Len() > 500{
				// 	conn, errC := pool.Get(3*time.Second)
				// 	if errC != nil{
				// 		fmt.Println(fmt.Errorf("could not get socket from pool: %v\n", errC).Error())
				// 	}
				// 	//fmt.Fprintf(conn.Sock, string(e.Data))
				// 	n, errW := conn.Sock.Write(buf.Bytes())
				// 	fmt.Println(n)
				// 	if errW != nil{
				// 		fmt.Println(fmt.Errorf("could not send off audit logs: %v\n", errW).Error())
				// 	}
				// 	errCl := conn.Close()
				// 	if errCl != nil{
				// 		fmt.Println(fmt.Errorf("couldnt put socket back into pool: %v\n", errCl).Error())
				// 	}
				// 	buf.Reset()
				// }
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

func NewWorker(Pool chan chan Event, c JobManager) *Worker {
	return &Worker{WorkerPool: Pool, JobChannel: make(chan Event), QuitChannel: make(chan bool), jobManager: c}
}
