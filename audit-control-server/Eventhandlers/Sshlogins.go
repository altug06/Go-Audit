package Eventhandlers

import(
	"bytes"
	//"log"

	"../messages"

)

type Sshlogin struct{
	Hostname 	string
	Addr 		string
	Terminal 	string
	Auid 		string
	Ses 		string
	Exe 		string
	Res 		string
	Op 			string
	Pid 		string
	Uid 		string
	Id 			string

}


func (s *Sshlogin) AnalyzeLogin( hostname string) string{

	var message bytes.Buffer
	if s.Res == "failed"{
		message.WriteString("This IP: " + s.Addr + " failed to connect to ssh")
	}else if s.Res == "success"{
		message.WriteString("Successfull ssh connection from IP: " + s.Addr + " as user: " + s.Uid + " on host: " + hostname)
	}

	auditLog := messages.AuditLog{
		Type : "SSHLOGIN",
		Message : message.String(),
	}

	// m := *mq
	// if len(*mq) > 1000{
	// 	n := len(m) - 1
	// 	m[n] = messages.AuditLog{}
	// 	m = m[:n]
	// 	*mq =append(m, auditLog)
	// }else{
	// 	*mq =append(m, auditLog)
	// }

	//log.Println(auditLog)

	return auditLog.Message

}