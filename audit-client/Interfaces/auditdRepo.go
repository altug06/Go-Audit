package Interfaces

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"

	"audit-client/Domain"
	"audit-client/Usecases"
)

var (
	PARSE  = "parse"
	BUILD  = "build"
	ADD    = "add"
	DELETE = "delete"

	rate    = 1000
	backlog = 8192
)

type AuditdHandler interface {
	AddRule(string) error
	DeleteRule(string) error
	GetStatus() ([]byte, error)
	SetEnabled() error
	GetAuditEvent(<-chan bool)
	SetRateLimit(int) error
	DeleteRules() error
	SetBackLogLimit(int) error
	SetPID() error
	Close()
}

type Event struct {
	Data []byte
	ID   string
}

type AuditdRepo struct {
	auditd      AuditdHandler
	ruleChanged bool
	done        chan bool
	rules       []string
}

type AuditRules struct {
	AuditRules []string `json:"audit_rules"`
}

func NewAuditd(auditdHandler AuditdHandler, done chan bool) AuditdRepo {
	auditDRepo := AuditdRepo{}
	auditDRepo.auditd = auditdHandler
	auditDRepo.done = done
	return auditDRepo
}

func (auditR *AuditdRepo) Init(filename string, isManager bool) error {
	errS := auditR.auditd.DeleteRules()
	if errS != nil {
		return errS
	}

	errST := auditR.SetStatus()
	if errST != nil {
		fmt.Println("Status")
		return errST
	}

	errS = auditR.SetRules(filename, isManager)
	if errS != nil {
		fmt.Println("rules")
		return errS
	}
	return nil

}

func (auditR *AuditdRepo) Close() {
	auditR.auditd.Close()
}

func (auditR *AuditdRepo) GetStatus() (Domain.AuditStatus, error) {
	data, errS := auditR.auditd.GetStatus()
	if errS != nil {
		return Domain.AuditStatus{}, errS
	}

	var status Domain.AuditStatus

	errU := json.Unmarshal(data, &status)
	if errU != nil {
		return Domain.AuditStatus{}, nil
	}
	return status, nil
}

func (auditR *AuditdRepo) ListRules() []string {
	return auditR.rules
}

func (auditR *AuditdRepo) SetStatus() error {

	status, err := auditR.GetStatus()
	if err != nil {
		return err
	}

	if status.Enabled == 0 {
		err = auditR.auditd.SetEnabled()
		if err != nil {
			return err
		}
	}

	if status.BacklogLimit != uint32(backlog) {
		err = auditR.auditd.SetBackLogLimit(backlog)
		if err != nil {
			return err
		}
	}

	if status.RateLimit != uint32(rate) {
		err = auditR.auditd.SetRateLimit(rate)
		if err != nil {
			return err
		}
	}

	err = auditR.auditd.SetPID()
	if err != nil {
		return err
	}

	return nil
}

func (auditR *AuditdRepo) RuleExists(rule string) bool {
	for _, r := range auditR.rules {
		if r == rule {
			return true
		}
	}
	return false
}

func (auditR *AuditdRepo) SetRule(rule string) error {
	if !auditR.RuleExists(rule) {
		err := auditR.auditd.AddRule(rule)
		if err != nil {
			return err
		}
		auditR.rules = append(auditR.rules, rule)
		return nil
	} else {
		return fmt.Errorf("Rule already exists : %s", rule)
	}

}

func (auditR *AuditdRepo) DeleteRule(rule string) error {
	if auditR.RuleExists(rule) {
		err := auditR.auditd.DeleteRule(rule)
		if err != nil {
			return err
		}
		fmt.Println("peki burdamıyım")
		auditR.DeleteFromSlice(rule)
		return nil

	} else {
		return Usecases.ErrRuleNotFound
	}

}

func (auditR *AuditdRepo) SetRules(filename string, isManager bool) error {
	var ar AuditRules
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("cant ReadFile: %w\n", err)
	}

	err = json.Unmarshal(buf, &ar)
	if err != nil {
		return fmt.Errorf("cant unmarshal: %w\n", err)
	}

	for _, rule := range ar.AuditRules {
		if isManager {
			err := auditR.auditd.AddRule(rule)
			if err != nil {
				fmt.Println(fmt.Errorf("WARNING: Rule: %s couldnt be added due to = %v", rule, err).Error())
				continue
			}
		}
		auditR.rules = append(auditR.rules, rule)
	}

	if !(len(auditR.rules) > 0) {
		return errors.New("No rules were able to added to audit daemon")
	}
	return nil
}

func (auditR *AuditdRepo) DeleteFromSlice(rule string) {
	for i, r := range auditR.rules {
		if r == rule {
			auditR.rules[i] = auditR.rules[len(auditR.rules)-1]
			auditR.rules[len(auditR.rules)-1] = ""
			auditR.rules = auditR.rules[:len(auditR.rules)-1]
			return
		}
	}
}

func (auditR *AuditdRepo) StartAudit() {
	auditR.auditd.GetAuditEvent(auditR.done)
}

func (auditR *AuditdRepo) StopAudit() {
	auditR.done <- true
}
