package endpoints

import (

	"net/http"
	"encoding/json"
	"log"
	"io/ioutil"
	"path"
	"strings"
	"fmt"

	"../messages"
	"../Agents"
	"../worker"
	
	"github.com/google/uuid"
)


//var AuditLogs []messages.AuditLog

type Router struct{
	Sys			*SyscallHandler
	Agents		*Agents.AuditAgents
}

type SyscallHandler struct{
	Queue		chan *worker.Job
}

//WebAuth middleware 
func WebAuth(h func(uid uuid.UUID) http.HandlerFunc) http.HandlerFunc{
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request){
		head, _ := ShiftPath(req.URL.Path)
		uid, errU := uuid.Parse(head)
		if errU != nil{
			log.Println(errU)
			http.Error(res,"not authorized", http.StatusUnauthorized)
			return
		}
		h(uid).ServeHTTP(res,req)
	})
}

//AgentAuth middleware 
func AgentAuth(h func(uid uuid.UUID) http.HandlerFunc) http.HandlerFunc{
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request){
		id := req.Header.Get("Auth")
		uid, errU := uuid.Parse(id)
		if errU != nil{
			log.Printf(fmt.Errorf("couldnt parse auth token: %v", errU).Error())
			http.Error(res,"not authorized: invalid token", http.StatusUnauthorized)
			return
		}
		h(uid).ServeHTTP(res,req)
	})
}

func (sys *SyscallHandler) ServeHTTP(res http.ResponseWriter, req *http.Request){

	m := new(messages.GeneralInfo)
	m.Data = make(map[string]*json.RawMessage)

	data, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		http.Error(res, "can't read body", http.StatusInternalServerError)
		return
	}

	err = json.Unmarshal(data, &m)
	//log.Printf("New event sent from agent")
	work := worker.NewJob(m, "SYSCALL")
	sys.Queue <- work
	res.WriteHeader(http.StatusOK)
}

func (r *Router) ServeHTTP(res http.ResponseWriter, req *http.Request){

	var head string
	head, req.URL.Path = ShiftPath(req.URL.Path)
	
	switch head {
	case "Syscall":
		id := req.Header.Get("Auth")
		if id != "" {
			uid, errU := uuid.Parse(id)
			if errU != nil{
				log.Printf(fmt.Errorf("couldnt parse auth token: %v", errU).Error())
				http.Error(res,"not authorized: invalid token", http.StatusUnauthorized)
				return
			}
			if _, ok := r.Agents.Agents[uid]; ok{
				r.Sys.ServeHTTP(res,req)
			}else{
				log.Printf("No such agent was found")
				http.Error(res,"No such agent was found", http.StatusBadRequest)
				return
			}
		}else{
			log.Printf("no auth token was found")
			http.Error(res,"not authorized: no token", http.StatusUnauthorized)
			return
		}
	case "Register":
		r.Agents.Register().ServeHTTP(res,req)
	case "ListAll":
		r.Agents.ListAll().ServeHTTP(res,req)
	case "DeleteFromAll":
		r.Agents.DeleteAll().ServeHTTP(res,req)
	case "GetRules":
		WebAuth(r.Agents.GetRules).ServeHTTP(res,req)
	case "StatusCheck":
		AgentAuth(r.Agents.StatusCheckIn).ServeHTTP(res,req)
	case "JobComplete":
		AgentAuth(r.Agents.JobComplete).ServeHTTP(res,req)
	case "DeleteRule":
		WebAuth(r.Agents.DeleteRule).ServeHTTP(res,req)
	case "DeRegister":
		AgentAuth(r.Agents.DeRegister).ServeHTTP(res,req)
	case "GetProcesses":
		WebAuth(r.Agents.GetProcesses).ServeHTTP(res,req)
	case "UpdataProcessInfo":
		AgentAuth(r.Agents.UpdateInfo).ServeHTTP(res,req)
	case "GetRulesHostname":
		head, _ := ShiftPath(req.URL.Path)
		r.Agents.GetRulesByHostname(head).ServeHTTP(res,req)
	// case "GetAuditLogs":
	// 	r.Agents.GetAuditLogs(AuditLogs).ServeHTTP(res,req)
	case "AddRule":
		WebAuth(r.Agents.AddRule).ServeHTTP(res,req)
	case "SaveConfig":
		head, _ := ShiftPath(req.URL.Path)
		uid, errU := uuid.Parse(head)
		if errU != nil{
			log.Println(errU)
			http.Error(res,"not authorized", http.StatusUnauthorized)
			return
		}
		r.Agents.OperationalJobs(uid, "SaveConfig").ServeHTTP(res,req)
	case "ShutDown":
		head, _ = ShiftPath(req.URL.Path)
		uid, errU := uuid.Parse(head)
		if errU != nil{
			log.Println(errU)
			http.Error(res,"not authorized", http.StatusBadRequest)
		}
		r.Agents.OperationalJobs(uid, "ShutDown").ServeHTTP(res,req)
	case "PurgeRules":
		head, _ := ShiftPath(req.URL.Path)
		uid, errU := uuid.Parse(head)
		if errU != nil{
			log.Println(errU)
			http.Error(res,"not authorized", http.StatusUnauthorized)
		}
		r.Agents.PurgeRules(uid).ServeHTTP(res,req)
	case "GetJobs":
		head, _ := ShiftPath(req.URL.Path)
		uid, errU := uuid.Parse(head)
		if errU != nil{
			log.Println(errU)
			http.Error(res,"not authorized", http.StatusUnauthorized)
		}
		r.Agents.GetJobs(uid).ServeHTTP(res,req)
	case "StartAudit":
		head, _ = ShiftPath(req.URL.Path)
		uid, errU := uuid.Parse(head)
		if errU != nil{
			log.Println(errU)
			http.Error(res,"not authorized", http.StatusBadRequest)
		}
		r.Agents.OperationalJobs(uid, "StartAudit").ServeHTTP(res,req)
	case "StopAudit":
		head, _ = ShiftPath(req.URL.Path)
		uid, errU := uuid.Parse(head)
		if errU != nil{
			log.Println(errU)
			http.Error(res,"not authorized", http.StatusBadRequest)
		}
		r.Agents.OperationalJobs(uid, "StopAudit").ServeHTTP(res,req)
	case "GetAuditStatus":
		WebAuth(r.Agents.GetAuditStatus).ServeHTTP(res,req)
	case "AuditStatus":
		AgentAuth(r.Agents.AuditStatus).ServeHTTP(res,req)
	default:
		http.Error(res, "Not Found", http.StatusNotFound)
	}

}

func ShiftPath(p string) (head, tail string) {
    p = path.Clean("/" + p)
    i := strings.Index(p[1:], "/") + 1
    if i <= 0 {
        return p[1:], "/"
    }
    return p[1:i], p[i:]
}
