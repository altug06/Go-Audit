package Domain

type AuditDaemon interface {
	ListRules() []string
	Init(string, bool) error
	GetStatus() (AuditStatus, error)
	Close()
	SetRule(string) error
	DeleteRule(string) error
	StopAudit()
	StartAudit()
}

type HostInfo interface {
	GetHostInfo()
}

type Host struct {
	Hostname      string  `json:"host"`
	KernelVersion string  `json:"kernelversion"`
	OS            string  `json:"os"`
	CpuCoreNumber int     `json:"cpucore"`
	CPUusage      float64 `json:"cpuSage"`
}

type AuditStatusMask uint32

// Mask types for AuditStatus.
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

type Auditd struct {
	Status AuditStatus
	Rules  []string
}
