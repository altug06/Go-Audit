package main

import (
	"net/http"
	"fmt"
	"os"
	"crypto/tls"
	"time"
	"strconv"
	"net"

	"./endpoints"
	"./Agents"
	"./pkg/Utils"
	"./worker"

	"github.com/google/uuid"
)

type Server struct {
	Server		*http.Server
	router 		*endpoints.Router
	Interface	string
	Port		int
}

var (
	server = "0.0.0.0"
	port = 8081
)

func NewServer(jobQueue chan *worker.Job) (*Server, error){
		
	cerp, err := Utils.GenerateTLSCert(nil, nil, nil, nil, nil, nil, true)
	if err != nil{
		fmt.Fprintf(os.Stderr, "Couldnt create a tls certificate%v\r\n", err)
		return nil, err
	}

	TLSConfig := &tls.Config{
		Certificates:             []tls.Certificate{*cerp},
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		},
	}

	s := &Server{
		router : &endpoints.Router{
			Sys : &endpoints.SyscallHandler{
				Queue : jobQueue,
			},
			Agents: &Agents.AuditAgents{
				Agents: make(map[uuid.UUID]*Agents.Agent),
			},
		},
		Interface : server,
		Port : port,
	}

	go s.router.Agents.CleanUp()

	srv := &http.Server{
		Addr:           server + ":" + strconv.Itoa(port),
		Handler:        s.router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		TLSConfig:      TLSConfig,
	}
	s.Server = srv
	

	return s, nil
	
	

}
func (s *Server) Start() {
	fmt.Println("server is gettings started ")
	err := s.Server.ListenAndServeTLS("","")
	if err != http.ErrServerClosed{
		fmt.Fprintf(os.Stderr, "Http server closed unexpectedly: %v\r\n", err)
	}else{
		fmt.Fprintf(os.Stderr, "Http server closed gracefully: %v\r\n")
	}

}

func connCreator() (net.Conn, error) {
	return net.Dial("udp", "go-logcentral1.jotservers.com:5548")
}


func main() {

	jobQueue := make(chan *worker.Job, 100)
	pool := worker.NewPool(30, jobQueue)
	pool.InitializeWorkers()
		
	errC := worker.NewConnectionPool(20, connCreator, 10)
	if errC != nil{
		fmt.Fprintf(os.Stderr, "Connection pool couldnt initialized: %v\r\n", errC)
	}
	fmt.Println("server is being created")
	instance, errS := NewServer(jobQueue)
	if errS != nil{
		fmt.Fprintf(os.Stderr, "Server object couldnt created\r\n", errS)
		os.Exit(1)
	}
	fmt.Println("server is created")
	instance.Start()
}