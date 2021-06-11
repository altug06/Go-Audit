package Agents

import(
	
	"net/http"
	"encoding/json"
	"strings"
	"log"
	"io/ioutil"
	"reflect"
	"fmt"
	"time"
	"sync"

	"../messages"

	"github.com/google/uuid"
)

type AuditAgents struct{
	Agents 	map[uuid.UUID]*Agent
}

type Agent struct{
	Rules 			map[string]string
	Hostname 		string
	UserAgent		string
	ID				uuid.UUID
	JobCh			chan messages.Job
	HostInfo		*messages.Host
	AuditMessages	[]string
	JobTrack		map[uuid.UUID]messages.Job
	RulesLock 		sync.RWMutex
	IsPurged		bool
	AuditStatus		*messages.AuditStatus
	lastSeen		time.Time
}


func (a *Agent) SetRule(key string, value string){
	a.RulesLock.Lock()
	a.Rules[key] = value
	a.RulesLock.Unlock()
}

func (a *Agent) DeleteRule(key string){
	a.RulesLock.Lock()
	if _, ok := a.Rules[key]; ok{
		delete(a.Rules, key)
	}
	a.RulesLock.Unlock()
}

func (a *Agent) GetRule(key string) (string, bool){
	a.RulesLock.RLock()
	value, ok := a.Rules[key]
	a.RulesLock.RUnlock()
	return value, ok
}

func (a *AuditAgents)OperationalJobs(uid uuid.UUID, jobType string) http.HandlerFunc{
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request){
		if agent, ok := a.Agents[uid]; ok{
			job := messages.Job{
				JobType: jobType,
				Retry: 0,
				JobID: uuid.New(),
				Status: "Waiting in Job queue",
			}
			log.Println(job)
			agent.JobTrack[job.JobID] = job
			agent.JobCh <- job	
		}else{
			http.Error(res, "Agent Not Found", http.StatusBadRequest)
		}
	})
}

func (a *AuditAgents)GetProcesses(uid uuid.UUID) http.HandlerFunc{
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request){
		if agent, ok := a.Agents[uid]; ok{
			if reflect.DeepEqual(messages.Host{}, agent.HostInfo) {
				log.Printf("No host info available")
				http.Error(res,"No host info available", http.StatusBadRequest)
			}

			body, errM := json.Marshal(agent.HostInfo)
			if errM != nil{
				log.Printf("Error marshaling host info: %v", errM)
				http.Error(res, "internal server error", http.StatusInternalServerError)
			}
			
			res.Header().Set("Content-Type","application/json")
			res.Write(body)
			
		}else{
			http.Error(res, "Agent Not Found", http.StatusBadRequest)
		}
	})
}

func (a *AuditAgents)PurgeRules(uid uuid.UUID) http.HandlerFunc{
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request){
		if agent, ok := a.Agents[uid]; ok{
			if len(agent.Rules)>0{
				job := messages.Job{
					JobType: "Purge",
					Retry: 0,
					JobID: uuid.New(),
					Status: "Waiting in Job queue",
				}
				agent.IsPurged = true
				agent.JobTrack[job.JobID] = job
				for rule, status := range agent.Rules{
					if status == "currentlyOk"{
						agent.SetRule(rule, "to be deleted")
					}
				}
				agent.JobCh <- job
			}else{
				http.Error(res, "No available rules present on the agent", http.StatusBadRequest)
			}
		}else{
			http.Error(res, "Agent Not Found", http.StatusBadRequest)
		}
	})

}


func (a *AuditAgents)GetAuditLogs(logs []messages.AuditLog) http.HandlerFunc{
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request){
		if len(logs) > 0{
			m := &messages.LogTrail{Logs:logs}
			encoded, errM := json.Marshal(m)
			if errM != nil{
				log.Println(fmt.Errorf("this is no good: %v",errM ))
				http.Error(res, "internal server error", http.StatusInternalServerError)
			}
			res.Header().Set("Content-Type","application/json")
			res.Header().Set("Access-Control-Allow-Origin", "*")
			res.Write(encoded)
			
		}else{
			http.Error(res, "No available audit log", http.StatusBadRequest)
		}
	})
}

func (a *AuditAgents)NewAgent(data []byte) (Agent, bool, error){

	var aMessage messages.AgentMessage
	var m Agent

	errU := json.Unmarshal(data, &aMessage)
	if errU != nil{
		return m, false, errU
	}

	if a.IsRegistered(aMessage.ID, aMessage.Hostname){
		return m, false, nil
	}

	log.Println("NewAgent is created:", aMessage.ID)
	m.ID = aMessage.ID
	m.Hostname = aMessage.Hostname
	m.HostInfo = aMessage.HostInfo
	m.Rules = make(map[string]string)
	m.AuditMessages = []string{}
	m.lastSeen = time.Now()
	for _, rule := range aMessage.Rules{
		m.Rules[rule] = "currentlyOk"
	}
	m.JobTrack = make(map[uuid.UUID]messages.Job)
	//increase job queue size
	m.JobCh = make(chan messages.Job, 100)

	return m, true, nil
}


func (a *AuditAgents)Register() http.HandlerFunc{
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request){

		data, err := ioutil.ReadAll(req.Body)
		if err != nil{
			log.Printf("Error reading body: %v", err)
			http.Error(res, "can't read body", http.StatusInternalServerError)
			return
		}
		
		m, isRegistered, errR := a.NewAgent(data)
		if errR != nil{
			http.Error(res, "", http.StatusInternalServerError)
			return
		}

		if !isRegistered{
			http.Error(res, "already registered", http.StatusBadRequest)
			return
		}
		log.Println(m)
		a.Agents[m.ID] = &m

		res.WriteHeader(http.StatusOK)
	})
}


func (a *AuditAgents) ListAll() http.HandlerFunc{
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request){
		agents := []*messages.AgentMessage{}
		log.Println(a.Agents)

		//cant send chan
		for _, agent := range a.Agents{
			m := GetBaseMessage(agent, "AgentList")
			log.Println(m.(*messages.AgentMessage).Rules)
			agents = append(agents, m.(*messages.AgentMessage))
		}
		
		encoded, errM := json.Marshal(agents)
		if errM != nil{
			http.Error(res, "internal server error", http.StatusInternalServerError)
		}

		res.Header().Set("Content-Type","application/json")
		res.Header().Set("Access-Control-Allow-Origin", "*")
		res.Write(encoded)
	})
}


func (a *AuditAgents) DeleteAll() http.HandlerFunc{
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request){
		if _, ok := req.URL.Query()["rule"]; !ok{
			http.Error(res,"missing parameter", http.StatusBadRequest)
		}
		log.Printf("rule to be deleted: " + req.URL.Query().Get("rule"))
		rule := req.URL.Query().Get("rule")

		job := messages.Job{
			JobType: "Delete",
			Rule:	rule,
			Retry: 0,
			JobID: uuid.New(),
			Status: "Waiting in Job queue",
		}

		for _, agent := range a.Agents{
			if !agent.IsPurged{
				if status, ok := agent.GetRule(rule); ok{
					if status == "currentlyOk"{
						log.Printf("rule status ok")
						agent.SetRule(rule, "to be deleted")
						agent.JobTrack[job.JobID] = job
						agent.JobCh <- job
					}else{
						http.Error(res, "there is another job currently assigned to this rule on this agent", http.StatusBadRequest)
					}
				}
			}
		}
	})
}


func GetBaseMessage(data interface{}, Type string) (interface{}){
	switch Type{
	case "Status":
		m := new(messages.BaseMessage)
		m.MessageType = "Statusok"
		return m
	case "Job":
		m := &messages.BaseMessage{
			MessageType: "Job",
			Data: data.(messages.Job),
		}
		return m
	case "AgentList":
		agent := new(messages.AgentMessage)
		agent.Hostname = data.(*Agent).Hostname
		agent.ID = data.(*Agent).ID
		agent.RuleCount = len(data.(*Agent).Rules)
		agent.HostInfo = data.(*Agent).HostInfo
		return agent
	default:
		return nil
	}
}



func (a *AuditAgents)AddRule(uid uuid.UUID) http.HandlerFunc{
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request){
		if agent, ok := a.Agents[uid]; ok{
			if !agent.IsPurged{
				if _, ok := req.URL.Query()["rule"]; !ok{
					http.Error(res,"missing parameter", http.StatusBadRequest)
				}
				log.Printf("rule to be added: " + req.URL.Query().Get("rule"))
				rule := req.URL.Query().Get("rule")
			
				if status,ok := agent.GetRule(rule); !ok{
					job := messages.Job{
						JobType: "AddRule",
						Rule:	rule,
						Retry: 0,
						JobID: uuid.New(),
						Status: "Waiting in Job queue",
					}
					agent.JobTrack[job.JobID] = job
					agent.JobCh <- job
					agent.SetRule(rule, "to be added") 
				}else{
					if status == "currentlyOk" {
						http.Error(res, "this rule is already present on the agent", http.StatusBadRequest)
						return
					}else if status == "to be deleted" {
						http.Error(res, "this rule is currently getting deleted", http.StatusBadRequest)
						return
					}
				}
			}else{
				http.Error(res, "IsPurged : true", http.StatusBadRequest)
			}
			
		}else{
			http.Error(res, "Agent Not Found", http.StatusNotFound)
		}
	})

}

func (a *AuditAgents)CheckAgent(hostname string)(*Agent, bool){
	for _, agent := range a.Agents{
		if agent.Hostname == hostname{
			return agent, true
		}
	}
	return nil, false
}

func (a *AuditAgents)GetRulesByHostname(hostname string) http.HandlerFunc{
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request){
		agent, isAgent := a.CheckAgent(hostname)
		if isAgent {
			data, errM := json.Marshal(agent.Rules)
			if errM != nil{
				log.Printf("Error reading body: %v", errM)
				http.Error(res, "can't unmarshal into struct", http.StatusBadRequest)
			}
			res.Header().Set("Content-Type","application/json")
			res.Write(data)
		}else{
			http.Error(res, "Agent Not Found", http.StatusNotFound)
		}
	})	
}

func (a *AuditAgents) StatusCheckIn(uid uuid.UUID) http.HandlerFunc{
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request){
		if agent, ok := a.Agents[uid]; ok{
			log.Println("checking in:", uid)
			if len(agent.JobCh) >= 1{
				log.Printf(uid.String())
				job := <- agent.JobCh
				j := agent.JobTrack[job.JobID]
				j.Status = "Dispacted to agent"
				agent.JobTrack[job.JobID] = j
				message := GetBaseMessage(job, "Job")
				b, errM := json.Marshal(message)
				if errM != nil {
					log.Printf("cant encode body: %v", errM)
				}
				res.Header().Set("Content-Type","application/json")
				res.Header().Set("Access-Control-Allow-Origin", "*")
				res.WriteHeader(http.StatusOK)
				res.Write(b)
			}else{
				m := GetBaseMessage(nil, "Status")
				b, errM := json.Marshal(m)
				if errM != nil {
					log.Printf("cant encode body: %v", errM)
				}
				res.Header().Set("Content-Type","application/json")
				res.Header().Set("Access-Control-Allow-Origin", "*")
				res.Write(b)
			}
			agent.lastSeen = time.Now()
		}else{
			http.Error(res, "Agent Not Found", http.StatusNotFound)
		}
	})
}



func (a *AuditAgents)JobComplete(uid uuid.UUID) http.HandlerFunc{
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request){
		if agent, ok := a.Agents[uid]; ok{
			data, err := ioutil.ReadAll(req.Body)
			if err != nil{
				log.Printf("Error reading body: %v", err)
				http.Error(res, "can't read body", http.StatusBadRequest)
			}

			var JobResult messages.Job
			errU := json.Unmarshal(data, &JobResult)
			if errU != nil{
				log.Printf("Error reading body: %v", err)
				http.Error(res, "can't unmarshal into struct", http.StatusBadRequest)
			}
			fmt.Println(JobResult)
			if job, ok := agent.JobTrack[JobResult.JobID]; ok{
				switch JobResult.JobType {
				case "Delete":
					if _, ok := agent.GetRule(JobResult.Rule); ok{
						if JobResult.Status == "JobSuccess" {
							log.Printf("Delete job success")
							job.Status = JobResult.Status
							job.Message = "Rule was successfully deleted"
							agent.DeleteRule(JobResult.Rule)
							
						}else{
							agent.SetRule(JobResult.Rule, "DeleteFailed : retrying")
							job.Status = JobResult.Status
							job.Message = JobResult.Message
							if job.Retry < 5{
								job.Retry += 1
								JobResult.Retry += 1
								agent.JobCh <- JobResult
								log.Printf("agent couldnt execute the job, agentID: %s", agent.ID)
							}else{
								agent.SetRule(JobResult.Rule,"Delete Operation failed : retrycount=5")
							}
						}
					}
				case "AddRule":
					if _, ok := agent.GetRule(JobResult.Rule); ok{
						if JobResult.Status == "JobSuccess"{
							job.Status = JobResult.Status
							job.Message = "Rule was sucessfully added"
							agent.SetRule(JobResult.Rule,"currentlyOk")
						}else{
							job.Status = JobResult.Status
							job.Message = JobResult.Message
							if JobResult.Status == "InvalidRule"{
								agent.DeleteRule(JobResult.Rule)
								log.Printf("this is invalid rule: %s", JobResult.Rule)
							}else if strings.Contains(JobResult.Message, "rule exists"){
								agent.DeleteRule(JobResult.Rule)
								log.Printf("this is a duplicated rule: %s", JobResult.Rule)
							}else{
								agent.SetRule(JobResult.Rule,"Addrule : retrying")
								job.Status = JobResult.Status
								job.Message = JobResult.Message
								if JobResult.Retry < 5{
									job.Retry += 1
									agent.JobCh <- JobResult
									log.Printf("agent couldnt execute the job, agentID: %s", agent.ID)
								}else{
									agent.DeleteRule(JobResult.Rule)
								}
							}
						}
					}
				case "Purge":
					if JobResult.Status == "JobSuccess"{
						for rule := range agent.Rules{
							agent.DeleteRule(rule)
						}
						agent.IsPurged = false
						job.Status = JobResult.Status
						job.Message = "Rules were successfully purged"
					}else{
						for rule, status := range agent.Rules{
							if status == "currentlyOk"{
								agent.SetRule(rule, "PurgeFailed: retrying")
							}
						}
						job.Status = JobResult.Status
						job.Message = JobResult.Message
						if JobResult.Retry < 5{
							job.Retry += 1
							JobResult.Retry += 1
							agent.JobCh <- JobResult
							log.Printf("agent couldnt execute the job, agentID: %s", agent.ID)
						}else{
							agent.SetRule(JobResult.Rule, "Purge Operation failed : retrycount=5")
						}
					}
				case "ShutDown":
					if JobResult.Status == "JobSuccess"{
						job.Status = JobResult.Status
						job.Message = "Agent was successfully terminated"
					}else{
						job.Status = JobResult.Status
						job.Message = JobResult.Message
						if JobResult.Retry < 5{
							job.Retry += 1
							JobResult.Retry += 1
							agent.JobCh <- JobResult
							log.Printf("agent couldnt execute the job, agentID: %s", agent.ID)
						}else{
							job.Message = "ShutDown Operation failed : retrycount=5"
						}
					}
				case "SaveConfig":
					if JobResult.Status == "JobSuccess"{
						job.Status = JobResult.Status
						job.Message = "Current config was successfully changed"
					}else{
						job.Status = JobResult.Status
						job.Message = JobResult.Message
						if JobResult.Retry < 5{
							job.Retry += 1
							JobResult.Retry += 1
							agent.JobCh <- JobResult
							log.Printf("agent couldnt execute the job, agentID: %s", agent.ID)
						}else{
							job.Message = "Save config failed: retrycount=5"
						}
					}
				case "StartAudit":
					job.Status = JobResult.Status
					job.Message = "Current config was successfully changed"
				case "StopAudit":
					job.Status = JobResult.Status
					job.Message = "Current config was successfully changed"

				}

				agent.JobTrack[JobResult.JobID] = job
			}else{
				http.Error(res, "No such job was issued by the control server", http.StatusBadRequest)
			}
		}else{
			http.Error(res, "Agent was not found", http.StatusNotFound)
		}
	})
}

func (a *AuditAgents)GetJobs(uid uuid.UUID) http.HandlerFunc{
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request){
		if agent, ok := a.Agents[uid]; ok{
			if len(agent.JobTrack)>0{
				data, errM := json.Marshal(agent.JobTrack)
				if errM != nil{
					log.Printf("Error reading body: %v", errM)
					http.Error(res, "can't unmarshal into struct", http.StatusBadRequest)
				}
				res.Header().Set("Content-Type","application/json")
				res.Write(data)
			}else{
				http.Error(res, "No job to be tracked", http.StatusBadRequest)
			}
			
		}else{
			http.Error(res, "Agent Not Found", http.StatusNotFound)
		}
	})
}

func (a *AuditAgents)GetRules(uid uuid.UUID) http.HandlerFunc{
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request){
		if agent, ok := a.Agents[uid]; ok{
			data, errM := json.Marshal(agent.Rules)
			if errM != nil{
				log.Printf("Error reading body: %v", errM)
				http.Error(res, "can't unmarshal into struct", http.StatusBadRequest)
			}
			res.Header().Set("Content-Type","application/json")
			res.Write(data)
		}else{
			http.Error(res, "Agent Not Found", http.StatusNotFound)
		}
	})
	
}

func (a *AuditAgents)GetAuditStatus(uid uuid.UUID) http.HandlerFunc{
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request){
		if agent, ok := a.Agents[uid]; ok{
			data, errM := json.Marshal(agent.AuditStatus)
			if errM != nil{
				log.Printf("Error reading body: %v", errM)
				http.Error(res, "can't unmarshal into struct", http.StatusBadRequest)
			}
			res.Header().Set("Content-Type","application/json")
			res.Write(data)
		}else{
			http.Error(res, "Agent Not Found", http.StatusNotFound)
		}
	})
	
} 

func (a *AuditAgents)AuditStatus(uid uuid.UUID) http.HandlerFunc{
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request){
		if agent, ok := a.Agents[uid]; ok{
			data, err := ioutil.ReadAll(req.Body)
			if err != nil{
				log.Printf("Error reading body: %v", err)
				http.Error(res, "can't read body", http.StatusBadRequest)
			}

			var status *messages.AuditStatus
			errU := json.Unmarshal(data, &status)
			if errU != nil{
				log.Printf("Error unmarshaling data: %v", err)
				http.Error(res, "body not valid", http.StatusBadRequest)
			}
			agent.AuditStatus = status

		}else{
			http.Error(res, "Agent Not Found", http.StatusNotFound)
		}
	})
}

func (a *AuditAgents)UpdateInfo(uid uuid.UUID) http.HandlerFunc{
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request){
		if agent, ok := a.Agents[uid]; ok{
			data, err := ioutil.ReadAll(req.Body)
			if err != nil{
				log.Printf("Error reading body: %v", err)
				http.Error(res, "can't read body", http.StatusBadRequest)
			}
			var hostInfo *messages.Host
			errU := json.Unmarshal(data, &hostInfo)
			if errU != nil{
				log.Printf("Error unmarshaling data: %v", err)
				http.Error(res, "body not valid", http.StatusBadRequest)
			}
			agent.HostInfo = hostInfo

		}else{
			log.Printf("no agent found with this uid")
			http.Error(res, "Agent Not Found", http.StatusNotFound)
		}
	})
}


func (a *AuditAgents)DeleteRule(uid uuid.UUID) http.HandlerFunc{
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request){
		if agent, ok := a.Agents[uid]; ok{
			if !agent.IsPurged{
				if _, ok := req.URL.Query()["rule"]; !ok{
					http.Error(res,"missing parameter", http.StatusBadRequest)
				}
				log.Printf(req.URL.Query().Get("rule"))
				rule := req.URL.Query().Get("rule")
				if status,ok := agent.GetRule(rule); ok{
					if status == "currentlyOk"{
						job := messages.Job{
							JobType: "Delete",
							Rule:	rule,
							Retry: 0,
							JobID: uuid.New(),
							Status: "Waiting in Job queue",
						}
						agent.JobTrack[job.JobID] = job
						agent.JobCh <- job
						agent.SetRule(rule, "to be deleted")
					}else{
						http.Error(res, "there is another job currently assigned to this rule on this agent", http.StatusBadRequest)
					}
				}else{
					http.Error(res, "invalid rule", http.StatusBadRequest)
				}
			}else{
				http.Error(res, "Purge: true", http.StatusBadRequest)
			}
		}else{
			http.Error(res, "Agent Not Found", http.StatusNotFound)
		}
	})
}

func (a *AuditAgents) DeRegister(uid uuid.UUID) http.HandlerFunc{
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request){
		if _, ok := a.Agents[uid]; ok{
			delete(a.Agents, uid)
			log.Printf("Agent deregistered")
		}else{
			log.Printf("no agent found with this uid")
			http.Error(res, "Agent Not Found", http.StatusNotFound)
		}
	})
}

func (a *AuditAgents) CleanUp(){

	fmt.Println("clean up routine has started")
	for{

		time.Sleep(2*time.Minute)

		for uid, agent := range a.Agents{
			agent.RulesLock.RLock()
			if time.Since(agent.lastSeen) > 1 * time.Minute{
				delete(a.Agents, uid)
			}
			agent.RulesLock.Unlock()
		}
		fmt.Println("clean up routine continues ")

	}
}

func (a *AuditAgents) IsRegistered(id uuid.UUID, hostname string) bool{
	if _, ok := a.Agents[id]; ok{
		return true
	}

	for _, agent := range a.Agents{
		if agent.Hostname == hostname{
			return true
		}
	}
	return false
}
