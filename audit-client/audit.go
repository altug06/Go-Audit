package main

import (
	"fmt"
	"net"
	"os"
	"sync"

	"audit-client/Infrastructure"
	"audit-client/Interfaces"
	"audit-client/Usecases"

	"github.com/google/uuid"
)

func connCreator() (net.Conn, error) {
	return net.Dial("udp", "go-logcentral2.jotservers.com:5548")
}

func main() {

	var wait sync.WaitGroup
	eventQueue := make(chan Usecases.Event, 100)
	doneCh := make(chan bool, 1)

	uid := uuid.New()

	client := Infrastructure.NewClient(os.Getenv("host"))
	auditDaemon, errA := Infrastructure.NewLibauditHandler(&wait, eventQueue)
	if errA != nil {
		fmt.Fprintf(os.Stderr, "Error auditclient: %v\n", errA)
		os.Exit(1)
	}

	// auditDaemonS, errS := Infrastructure.NewLibauditHandler(&wait, eventQueue)
	// if errS != nil {
	// 	fmt.Fprintf(os.Stderr, "Error auditclient: %v\n", errA)
	// 	os.Exit(1)
	// }

	auditManager := Interfaces.NewAuditd(&auditDaemon, doneCh)
	//auditManagerS := Interfaces.NewAuditd(&auditDaemonS, doneCh)
	jobManager := Interfaces.NewAPIClient(&client, uid.String())

	errI := auditManager.Init(os.Args[1], true)
	if errI != nil {
		fmt.Fprintf(os.Stderr, "Error audit init failed: %v\n", errI)
		os.Exit(1)
	}

	// errI = auditManagerS.Init(os.Args[1], false)
	// if errI != nil {
	// 	fmt.Fprintf(os.Stderr, "Error audit init failed: %v\n", errI)
	// 	os.Exit(1)
	//}

	workerPool := Usecases.NewPool(30, eventQueue)
	workerPool.InitializeWorkers(&jobManager)

	// errC := Usecases.NewConnectionPool(20, connCreator, 10)
	// if errC != nil{
	// 	fmt.Fprintf(os.Stderr, "Connection pool couldnt initialized: %v\r\n", errC)
	// }

	logger := Usecases.LoggerInit(os.Stdout, os.Stdout, os.Stderr)

	agent, errN := Usecases.NewAgent(&auditManager, &jobManager, &wait, &logger, uid)
	if errN != nil {
		fmt.Fprintf(os.Stderr, "Could not get hostname: %v\n", errN)
		os.Exit(1)
	}
	agent.Run()

}

// Receivers always block until there is data to receive.
// If the channel is unbuffered, the sender blocks until the receiver has received the value.
// If the channel has a buffer, the sender blocks only until the value has been copied to the buffer;
// if the buffer is full, this means waiting until some receiver has retrieved a value.
