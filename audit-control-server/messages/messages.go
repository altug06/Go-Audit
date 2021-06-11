package messages

import(
	"encoding/json"
	"github.com/google/uuid"
)

type GeneralInfo struct {
	Serial 		uint64
	Data		map[string]*json.RawMessage
	HostName	string
	Message		string
}

type id struct {
	name	string
	id		int32
}

type Proctitle struct{
	Proctitle string
}

type Execve struct {
	Argc 	string
	A0 		string
	A1		string
	A2		string
	A3		string
	A4		string
	A5		string
	Message		string
}

type Path struct {
	Item 		string
	Ogid 		string
	Nametype 	string
	Inode 		string
	Dev 		string
	Mode 		string
	Rdev 		string
	Name 		string
	Ouid 		string
	Message		string
}

type Saddr struct {
	Family	string
	Port 	string
	Ip		string
}

type AgentMessage struct{
	Hostname	string
	ID			uuid.UUID
	RuleCount 	int
	Rules		[]string
	HostInfo	*Host
}

type BaseMessage struct{
	MessageType string
	Data		Job
}

type JobResultMessage struct{
	Message 	bool
	JobDone		Job
	Data		interface{}
}

type Job struct{
	JobType string
	Rule 	string
	Retry 	int
	Status	string
	Message	string
	JobID   uuid.UUID
}

type DiskStat struct {
	Device            string  `json:"device"`
	Path              string  `json:"path"`
	Fstype            string  `json:"fstype"`
	Total             uint64  `json:"total"`
	Free              uint64  `json:"free"`
	Used              uint64  `json:"used"`
	UsedPercent       float64 `json:"usedPercent"`
	InodesTotal       uint64  `json:"inodesTotal"`
	InodesUsed        uint64  `json:"inodesUsed"`
	InodesFree        uint64  `json:"inodesFree"`
	InodesUsedPercent float64 `json:"inodesUsedPercent"`
	IopsInProgress    uint64  `json:"iopsInProgress"`
}

type AuditStatusMask uint32

const (
	AuditStatusEnabled AuditStatusMask = 1 << iota
	AuditStatusFailure
	AuditStatusPID
	AuditStatusRateLimit
	AuditStatusBacklogLimit
	AuditStatusBacklogWaitTime
	AuditStatusLost
)

type AuditStatus struct {
	Mask            AuditStatusMask // Bit mask for valid entries.
	Enabled         uint32          // 1 = enabled, 0 = disabled
	Failure         uint32          // Failure-to-log action.
	PID             uint32          // PID of auditd process.
	RateLimit       uint32          // Messages rate limit (per second).
	BacklogLimit    uint32          // Waiting messages limit.
	Lost            uint32          // Messages lost.
	Backlog         uint32          // Messages waiting in queue.
	FeatureBitmap   uint32          // Bitmap of kernel audit features (previously to 3.19 it was the audit api version number).
	BacklogWaitTime uint32          // Message queue wait timeout.
}

type Meminfo struct {
	RSS 		uint64 		`json:"rss"`
	VMS 		uint64 		`json:"vms"`
	Mempercent 	float32		`json:"memper"`
	CPUpercent 	float64		`json:"cpuper"`
}

type Process struct {
	Pid			int32  		`json:"pid"`
	Name		string 		`json:"name"`
	Status		string 		`json:"status"`
	Ppid		int32  		`json:"ppid"`
	Cmdline		string 		`json:"cmdline"`
	Username	string 		`json:"username"`
	MemoryInfo  interface{} `json:"meminfo"`
}

type Host struct {
	Hostname		string		`json:"host"`
	KernelVersion	string		`json:"kernelversion"`
	OS				string		`json:"os"`
	DiskUsage		[]*DiskStat	`json:"diskUsage"`
	MemInfo			*HostMemInfo	`json:"memInfo"`
	Processes		[]*Process	`json:"processInfo"`
	CpuCoreNumber	int			`json:"cpucore"`
	CPUusage		float64		`json:"cpuSage"`
}

type HostMemInfo struct {
	Total		uint64		`json:"total"`
	Free		uint64		`json:"free"`
	UsedPercent	float64		`json:"usedpercent"`
}

type LogTrail struct{
	 Logs []AuditLog
}

type AuditLog struct{
	Type string
	Message string
}