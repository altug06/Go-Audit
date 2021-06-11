package Usecases

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"audit-client/Domain"

	"github.com/google/uuid"
)

var ErrRuleNotFound = errors.New("Rule not found")
var ErrUnknownAction = errors.New("Action was not found")
var ErrServerUnreachable = errors.New("Server is un reachable")
var ErrAgentNotRegistered = errors.New("Response status code was not 200 OK")

type MessageAgent struct {
	Rules    []string
	Hostname string
	ID       uuid.UUID
	HostInfo Domain.HostInfo
}

type Logger interface {
	Log(string, string)
}

type Agent struct {
	Hostname     string
	ID           uuid.UUID
	IsRegistered bool
	AuditclientS Domain.AuditDaemon
	HostInfo     Domain.HostInfo
	jobManager   JobManager
	WaitGroup    *sync.WaitGroup
	logger       Logger
	verbose      bool
}

type JobManager interface {
	PollJOB() (Job, error)
	SendMessage(interface{}, string) error
}

type Job struct {
	JobType string
	Rule    string
	Retry   int
	Status  string
	Message string
	JobID   uuid.UUID
}

func NewAgent(auditDS Domain.AuditDaemon, j JobManager, wait *sync.WaitGroup, l Logger, uid uuid.UUID) (*Agent, error) {
	a := &Agent{
		ID:           uid,
		IsRegistered: false,
		AuditclientS: auditDS,
		jobManager:   j,
		WaitGroup:    wait,
		logger:       l,
	}

	//a.UndeliveredJobs = make(chan JobResultMessage, 10)

	hostname, errH := os.Hostname()
	if errH != nil {
		return a, fmt.Errorf("there was an error getting the hostname:\r\n%s", errH)
	}

	a.Hostname = hostname
	return a, nil
}

func (a *Agent) SendStatus() {
	status, errS := a.AuditclientS.GetStatus()
	if errS != nil {
		a.logger.Log(fmt.Errorf("couldnt get the audit status: %v", errS).Error(), ERROR)
		return
	}

	errC := a.jobManager.SendMessage(status, "SendAuditStatus")
	if errC != nil {
		if errors.Is(errC, ErrAgentNotRegistered) {
			a.Register()
		}
		if a.verbose {
			a.logger.Log(fmt.Errorf("couldnt report the job result: %v", errC).Error(), ERROR)
		}

		return
	}

}

func (a *Agent) StatusCheck() {
	a.WaitGroup.Add(1)
	defer a.WaitGroup.Done()

	a.SendStatus()
	job, errC := a.jobManager.PollJOB()
	if errC != nil {
		if errors.Is(errC, ErrAgentNotRegistered) {
			a.logger.Log(fmt.Errorf("Server was restarted : %v, re-registering.. ", errC).Error(), INFO)
			a.Register()
			return
		} else if errors.Is(errC, ErrServerUnreachable) {
			//?? what do you do if server is unreachable
		}
		a.logger.Log(fmt.Errorf("Statuscheck fail : %v", errC).Error(), ERROR)
		return
	}

	if job != (Job{}) {
		j := a.JobHandler(job)
		fmt.Println("Jobresult is getting prep")
		fmt.Println(j)
		errJ := a.jobManager.SendMessage(j, "SendJobResult")
		if errJ != nil {
			a.logger.Log(fmt.Errorf("Couldnt send job result due to: %v", errJ).Error(), ERROR)
		}
	}

}

// func (a *Agent)SendProcesses(){
// 	a.WaitGroup.Add(1)
// 	defer a.WaitGroup.Done()
// 	a.HostInfo = a.HostInfo.GetHostInfo()
// 	errH := a.jobManager.SendMessage(a.HostInfo, "SendProcesses",a.ID.String())
// 	if err != nil{
// 		if errors.Is(err, errServerUnreachable){
// 			a.Logger.Log("Control server is currenty unreachable", INFO)
// 			//switch to kibana
// 		}else if errors.Is(err, errAgentNotRegistered){
// 			a.logger.Log("Control server was restarted, re-registering for further operations...", INFO)
// 			a.Register()
// 		}
// 	}
// }

func (a *Agent) JobHandler(j Job) Job {
	switch j.JobType {
	case "Delete":
		errD := a.AuditclientS.DeleteRule(j.Rule)
		if errD != nil {
			j.Status = "JobFailed"
			j.Message = fmt.Errorf("couldnt delete the rule: %v", errD).Error()
			return j
		}
		j.Status = "JobSuccess"
		return j
	case "AddRule":
		errD := a.AuditclientS.SetRule(j.Rule)
		if errD != nil {
			j.Status = "JobFailed"
			j.Message = fmt.Errorf("couldnt add the rule: %v", errD).Error()
			return j
		}

		j.Status = "JobSuccess"
		return j
	case "SaveConfig":
		errS := a.SaveConfig()
		if errS != nil {
			j.Status = "JobFailed"
			j.Message = fmt.Errorf("couldnt save the rules: %v", errS).Error()
			return j
		}
		j.Status = "JobSuccess"
		return j
	// case "StopAudit":
	// 	a.Auditclient.StopAudit()
	// 	j.Status = "JobSuccess"
	// 	return j
	// case "StartAudit":
	// 	a.Auditclient.StartAudit()
	// 	j.Status = "JobSuccess"
	// 	return j
	case "ShutDown":
		go a.DeRegister()
		j.Status = "JobSuccess"
		return j
	case "Purge":
		rules := a.AuditclientS.ListRules()
		var err error
		counter := 0
		for _, rule := range rules {
			err = a.AuditclientS.DeleteRule(rule)
			if err != nil {
				fmt.Println(rule)
				a.logger.Log(err.Error(), WARN)
				err = nil
				continue
			}
			counter++
		}

		if counter == len(rules) {
			j.Status = "JobSuccess"
			return j
		}

		j.Status = "JobFailed"
		j.Message = "Some rules were not deleted"
		return j
	default:
		a.logger.Log("Unsupported job type : "+j.JobType, WARN)
		return Job{}

	}
}

func (a *Agent) SaveConfig() error {
	if _, err := os.Stat("./auditrules.json"); !os.IsNotExist(err) {
		errR := os.Remove("./auditrules.json")
		if errR != nil {
			return fmt.Errorf("Error deleting the old config file: %v", errR)
		}
	}
	data, errM := json.Marshal(a.AuditclientS.ListRules())
	if errM != nil {
		return fmt.Errorf("Error marshaling rules to save in config file: %v", errM)
	}
	err := ioutil.WriteFile("auditrules.json", data, 0755)
	if err != nil {
		return fmt.Errorf("Error writing rules to the specified file: %v", errM)
	}
	return nil
}

func (a *Agent) Register() {
	a.logger.Log("Registering agent: "+a.Hostname, INFO)
	message := MessageAgent{
		Rules:    a.AuditclientS.ListRules(),
		Hostname: a.Hostname,
		ID:       a.ID,
		HostInfo: a.HostInfo,
	}
	// check if server is responding ? and increase the counter by one
	err := a.jobManager.SendMessage(message, "Register")
	if err != nil {
		if errors.Is(err, ErrServerUnreachable) {
			//send to kibana
			a.logger.Log("Control server is currenty unreachable, retry in 10 seconds", INFO)
		}
		return
	}

	a.IsRegistered = true
}

func (a *Agent) DeRegister() {
	fmt.Println("Shutting down audit daemon")
	a.AuditclientS.StopAudit()
	fmt.Println("Waiting for all workers to finish their job")
	a.WaitGroup.Wait()
	fmt.Println("Closing audit sockets")
	//a.Auditclient.Close()
	a.AuditclientS.Close()
	// fmt.Println("Deregistering from the control server")
	// err := a.jobManager.SendMessage(nil, "DeRegister")
	// if err != nil {
	// 	if errors.Is(err, ErrServerUnreachable) {
	// 		a.logger.Log("Control server is currenty unreachable", INFO)
	// 	} else if errors.Is(err, ErrAgentNotRegistered) {
	// 		a.logger.Log("Control server was restarted", INFO)
	// 	}

	// }

	os.Exit(1)

}

func (a *Agent) Run() {

	a.AuditclientS.StartAudit()

	fmt.Println(a.AuditclientS.ListRules())
	status, err := a.AuditclientS.GetStatus()
	if err != nil {
		a.logger.Log(fmt.Errorf("couldnt get status : %v", err).Error(), WARN)
	} else {
		fmt.Println(status)
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	for {
		select {
		case <-sigc:
			a.DeRegister()
		}
		// default:
		// 	if !a.IsRegistered {
		// 		a.Register()
		// 	} else {
		// 		go a.StatusCheck()
		// 		//go a.SendProcesses()
		// 	}
		// 	time.Sleep(10 * time.Second)
		// }

	}

}

//TO-DO
//improve logging and error handling - in progress
//Decide what to do in case of a server failure - in-progress
//Sleep for how long ??
