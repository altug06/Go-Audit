package Eventhandlers

import(
	"log"
	"bytes"
	"encoding/json"
	"reflect"

	"../messages"

)
var(
	ProcessMap map[string]int
	ConnectionMap map[string]int
)


type Syscall struct{
	Success		string
	Exit		string
	Pid			string
	Ppid		string
	Auid		string
	Uid			string
	Gid			string
	Sgid		string
	Suid		string
	Fsuid		string
	Euid		string
	Egid		string
	Tty			string
	Syscall		string
	Comm		string
	Exe			string
	Message		string

}

func (s *Syscall) AnalyzeExecve(hostname string, exec *json.RawMessage, p chan []byte) string{

	var fulltitle bytes.Buffer
	var message bytes.Buffer
	var exe messages.Execve

	if ProcessMap == nil{
		ProcessMap = make(map[string]int)
	}

	errE := json.Unmarshal(*exec, &exe)
	if errE!= nil{
		log.Fatal(errE)
	}

	args := []string{}

	args = append(args, exe.A0)
	args = append(args, exe.A1)
	args = append(args, exe.A2)
	args = append(args, exe.A3)

	for _, arg := range args {
		if arg != ""  {
			fulltitle.WriteString(arg + " ")
		}
	
	}
	message.WriteString("User: " + s.Uid + " executed " + "\"" + fulltitle.String() + "\"" + " on host: " + hostname)

	auditLog := messages.AuditLog{
		Type : "EXECVE",
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

	proc := fulltitle.String()
	if _, ok := ProcessMap[proc]; !ok{
		//log.Println("sending out the process name")
		p <- []byte(proc + "\n")
		ProcessMap[proc] = 1
	}else{
		ProcessMap[proc] += 1
	}
	
	//log.Println(auditLog)

	return auditLog.Message
	
		
}

func (s *Syscall) AnalyzeConnect(sock *json.RawMessage, hostname string, c chan []byte) string{
	var saddr messages.Saddr

	if ConnectionMap == nil{
		ConnectionMap = make(map[string]int)
	}

	err := json.Unmarshal(*sock, &saddr)
	if err != nil{
		log.Fatal(err)
	}
	
	if reflect.DeepEqual(messages.Saddr{}, saddr){
		return ""
	}else{
		var message	bytes.Buffer
		message.WriteString("User: " + s.Uid + " tried to connect to " + saddr.Ip + ":" + saddr.Port + " on host: " + hostname )

		auditLog := messages.AuditLog{
			Type : "CONNECT",
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

		con := saddr.Ip + " : " + saddr.Port
		if _, ok := ConnectionMap[con]; !ok{
			//log.Println("sending out the connection details")
			c <- []byte(con + "\n")
			ConnectionMap[con] = 1
		}else{
			ConnectionMap[con] += 1
		}

		//log.Println(auditLog)

		return auditLog.Message
	}
}


func (s *Syscall) AnalyzeOpen(file *json.RawMessage, hostname string) string{

	var path messages.Path
	var message	bytes.Buffer

	errP := json.Unmarshal(*file, &path)
	if errP != nil{
		log.Fatal(errP)
	}

	message.WriteString("User: " + s.Uid + " opened " + path.Name + ":" + " with the Success: " + s.Success + " on host " + hostname )

	auditLog := messages.AuditLog{
		Type : "OPEN",
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