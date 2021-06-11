package Infrastructure

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"

	"audit-client/Usecases"

	"github.com/elastic/go-libaudit"
	"github.com/elastic/go-libaudit/rule"
	"github.com/elastic/go-libaudit/rule/flags"
)

var (
	recv = 8192
)

type LibauditdHandler struct {
	daemon   *libaudit.AuditClient
	wg       *sync.WaitGroup
	jobQueue chan<- Usecases.Event
}

func NewLibauditHandler(wait *sync.WaitGroup, jobQueue chan<- Usecases.Event) (LibauditdHandler, error) {
	var diagWriter io.Writer
	client, errC := libaudit.NewAuditClient(diagWriter, recv)
	if errC != nil {
		return LibauditdHandler{}, fmt.Errorf("there was an error creating the audit client: %v", errC)
	}

	libauditH := LibauditdHandler{}
	libauditH.daemon = client
	libauditH.jobQueue = jobQueue
	libauditH.wg = wait

	return libauditH, nil
}

func (l *LibauditdHandler) ConvertRule(r string) ([]byte, error) {
	d, errP := flags.Parse(r)
	if errP != nil {
		return nil, fmt.Errorf("Parse error: %v", errP)
	}

	data, errB := rule.Build(d)
	if errB != nil {
		return nil, fmt.Errorf("Build error: %v", errB)
	}
	return data, nil

}

func (l *LibauditdHandler) AddRule(rule string) error {
	data, errC := l.ConvertRule(rule)
	if errC != nil {
		return errC
	}

	errR := l.daemon.AddRule(data)
	if errR != nil {
		return fmt.Errorf("Audit daemon could not add the rule: %s, err: %v", rule, errR)
	}

	return nil

}

func (l *LibauditdHandler) DeleteRule(rule string) error {
	data, errC := l.ConvertRule(rule)
	if errC != nil {
		return errC
	}

	if errD := l.daemon.DeleteRule(data); errD != nil {
		return fmt.Errorf("Audit daemon could not delete the rule: %s, err: %v", rule, errD)
	}
	return nil
}

func (l *LibauditdHandler) GetStatus() ([]byte, error) {
	status, err := l.daemon.GetStatus()
	if err != nil {
		return nil, fmt.Errorf("libaudithandler: GetStatus : %v", err)
	}
	data, errM := json.Marshal(status)
	if errM != nil {
		return nil, fmt.Errorf("libaudithandler: Cant marshal: %v", errM)
	}
	return data, nil
}

func (l *LibauditdHandler) SetEnabled() error {
	fmt.Println("enabling auditing in the kernel")
	if err := l.daemon.SetEnabled(true, libaudit.WaitForReply); err != nil {
		return fmt.Errorf("there was an error enabling the audit:\r\n%w", err)
	}
	return nil
}

func (l *LibauditdHandler) SetRateLimit(rate int) error {
	fmt.Println("setting rate limit to : %v", rate)
	if err := l.daemon.SetRateLimit(uint32(rate), libaudit.WaitForReply); err != nil {
		return fmt.Errorf("there was an error setting the rate limit:\r\n%s", err)
	}
	return nil
}

func (l *LibauditdHandler) SetBackLogLimit(backlog int) error {
	fmt.Println("setting rate limit to : %v", backlog)
	if err := l.daemon.SetBacklogLimit(uint32(backlog), libaudit.WaitForReply); err != nil {
		return fmt.Errorf("there was an error setting backlog limit:\r\n%s", err)
	}
	return nil
}

func (l *LibauditdHandler) Close() {
	l.daemon.Close()
}

func (l *LibauditdHandler) SetPID() error {
	fmt.Println("sending message to kernel registering our PID (%v) as the audit daemon", os.Getpid())
	if err := l.daemon.SetPID(libaudit.WaitForReply); err != nil {
		return fmt.Errorf("there was an error registering the pid:\r\n%s", err)
	}
	return nil
}

func (l *LibauditdHandler) DeleteRules() error {
	_, errD := l.daemon.DeleteRules()
	if errD != nil {
		return errD
	}
	return nil

}

func (l *LibauditdHandler) CallBackEventHandler(e *libaudit.AuditEvent, err error) {

	l.wg.Add(1)
	defer l.wg.Done()

	if err != nil {
		var nerr *libaudit.ErrorAuditParse
		if errors.As(err, &nerr) {
			fmt.Printf("parser error: %v: %v\n", nerr, nerr.Raw)
		} else {
			fmt.Printf("callback received error: %v\n", err)
		}
		return
	}

	if !reflect.DeepEqual(libaudit.GeneralInfo{}, e.Custom) {
		data, err := json.Marshal(e.Custom)
		if err != nil {
			libaudit.LogMessage(fmt.Errorf("could not marshal the event : %v", err).Error(), libaudit.ERROR)
		}

		l.jobQueue <- Usecases.Event{Data: data}

	}

}

func (l *LibauditdHandler) GetAuditEvent(done <-chan bool) {
	go libaudit.GetAuditMessages(l.daemon, l.CallBackEventHandler, done)
}
