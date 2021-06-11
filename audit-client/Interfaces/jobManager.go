package Interfaces

import(

	"audit-client/Usecases"
	"audit-client/Domain"

	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"errors"
	"strconv"
)


type ExternalServiceHandler interface{
	SendMessage([]byte, string, bool, string, string) (io.ReadCloser, error)
	Ping() error
}

type APIClient struct{
	ExtServ	 		ExternalServiceHandler
	failedCheckin 	int
	uid				string
}

type APIStatusError struct{
	Action			string
	StatusCode 		int
}

func (a *APIStatusError)Error() string{
	return a.Action + " : HTTP Response status code = " + strconv.Itoa(a.StatusCode)
}

type APIClientError struct{
	Action		string
	Err			error
	Op			string
}

func (a *APIClientError)Error() string{
	return a.Action + " : " + a.Err.Error() + " : " + a.Op
}

type BaseMessage struct{
	MessageType string
	Data		Usecases.Job
}


func NewAPIClient(client ExternalServiceHandler, id string) APIClient{
	return APIClient{ExtServ: client, failedCheckin: 0, uid: id}
}


func (e *APIClient)PollJOB() (Usecases.Job, error){
	if e.failedCheckin>10{
		if err := e.ExtServ.Ping(); err != nil{
			return Usecases.Job{}, err
		}else{
			e.failedCheckin = 0
		}
	}

	body, errJ := e.ExtServ.SendMessage(nil, "StatusCheck", false, e.uid ,"GET")
	if errJ != nil{
		var c *APIClientError
		var a *APIStatusError
		if errors.As(errJ, &c){
			if c.Op == "ServerIssues"{
				e.failedCheckin++
			}
		}else if errors.As(errJ, &a){
			if a.StatusCode == 404{
				return Usecases.Job{}, Usecases.ErrAgentNotRegistered
			}
		}
		return Usecases.Job{}, errJ
	}

	data, errB := ioutil.ReadAll(body)
	if errB != nil{
		fmt.Println(fmt.Errorf("body read fail : %v", errB).Error())
		return Usecases.Job{}, errB
	}

	var j BaseMessage

	errM := json.Unmarshal(data, &j)
	if errB != nil{
		fmt.Println(fmt.Errorf("Unmarshal fail : %v", errM).Error())
		return Usecases.Job{}, errB
	}

	if j.MessageType == "Job"{
		return j.Data, nil
	}else{
		return Usecases.Job{}, nil
	}
	
}

func (e *APIClient)SendMessage(data interface{}, action string) error{
	
	if e.failedCheckin>10{
		if err := e.ExtServ.Ping(); err != nil{
			return Usecases.ErrServerUnreachable
		}
	}

	var err error
	var s *APIStatusError

	switch action {
	case "SendProcess":
		err = e.SendProcesses(data, e.uid)
	case "SendJobResult":
		err = e.SendJobResult(data.(Usecases.Job), e.uid)
	case "SendAuditStatus":
		err = e.SendAuditStatus(data.(Domain.AuditStatus), e.uid)
	case "SendAuditEvent":
		err = e.SendAuditEvent(data.([]byte), e.uid)
	case "DeRegister":
		err = e.DeRegister(e.uid)
	case "Register":
		err = e.Register(data.(Usecases.MessageAgent), e.uid)
	default:
		return Usecases.ErrUnknownAction
		
	}

	if err != nil{
		if errors.As(err, &s){
			if s.StatusCode == 404{
				return Usecases.ErrAgentNotRegistered
			}	
		}else{
			var c *APIClientError
			if errors.As(err, &c){
				if c.Op == "ServerIssues"{
					e.failedCheckin++
				}
			}
		return err
		}
	}

	e.failedCheckin = 0

	return nil
}

func (e *APIClient)SendProcesses(hostInfo interface{}, uid string) error{
	data, errJ := json.Marshal(hostInfo)
	if errJ != nil{
		return errJ
	}
	_, errJ = e.ExtServ.SendMessage(data, "UpdateProcessInfo", true, uid, "POST")
	if errJ != nil{
		return errJ
	}
	return nil
}

func (e *APIClient)SendJobResult(j Usecases.Job, uid string) error{
	data, errJ := json.Marshal(j)
	if errJ != nil{
		return errJ
	}
	_, errJ = e.ExtServ.SendMessage(data, "JobComplete", true, uid, "POST")
	if errJ != nil{
		return errJ
	}

	return nil	
	
}

func (e *APIClient)SendAuditStatus(status Domain.AuditStatus, uid string) error{

	data, errJ := json.Marshal(status)
	if errJ != nil{
		return errJ
	}
	_, errJ = e.ExtServ.SendMessage(data, "AuditStatus", true, uid, "POST")
	if errJ != nil{
		return errJ
	}

	return nil
		
}

func (e *APIClient)SendAuditEvent(event []byte, uid string)error{
	_, errJ := e.ExtServ.SendMessage(event, "Syscall", true, uid, "POST")
	if errJ != nil{
		return errJ
	}

	return nil
}

func (e *APIClient)Register(agent Usecases.MessageAgent, uid string)error{
	data, errJ := json.Marshal(agent)
	if errJ != nil{
		return errJ
	}
	_, errJ = e.ExtServ.SendMessage(data, "Register",true, uid, "POST")
	if errJ != nil{
		return errJ
	}

	return nil
}

func (e *APIClient)DeRegister(uid string) error{
	_, errJ := e.ExtServ.SendMessage(nil, "DeRegister", true, uid, "POST")
	if errJ != nil{
		return errJ
	}

	return nil

}
