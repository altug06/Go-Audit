package Infrastructure

import(
	"net/url"
	"net/http"
	"time"
	"io/ioutil"
	"io"
	"bytes"
	"net"
	"crypto/tls"
	"errors"

	"audit-client/Interfaces"


)


type HTTPClient struct{
	BaseURL		*url.URL
	client		*http.Client
}


func NewClient(host string) HTTPClient{

	TLSConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true, 
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		},
		NextProtos: []string{"https"},
	}

	h := &http.Client{
		Timeout: time.Second * 10,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
			}).Dial,
			MaxIdleConns:        30,
			MaxIdleConnsPerHost: 30,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			TLSClientConfig: TLSConfig,
		},
	}

	c := HTTPClient{
		BaseURL : &url.URL{	
			Scheme : "https",
			Host :	host,
		},
		client : h,
	}

	return c
}

func (c *HTTPClient)Ping() error{
	rel := &url.URL{Path: "ping"}
	endpoint := c.BaseURL.ResolveReference(rel)

	req, errR := http.NewRequest("GET", endpoint.String(), nil)
	if errR != nil{
		return &Interfaces.APIClientError{Action: "NewRequest", Err: errR}
	}

	_, errR = c.Do(req, true)
	if errR != nil {
		return errR
	}

	return nil
}

func (c *HTTPClient)SendMessage(data []byte, endpoint string, drain bool, uid string, method string) (io.ReadCloser, error){
	
	var req *http.Request

	req, errR := c.NewRequest(method, endpoint, data)
	if errR != nil{
		return nil, errR
	}
	
	req.Header.Set("Auth",uid)

	resp, err := c.Do(req, drain)
	if err != nil{
		return nil, err
	}

	if resp.StatusCode != http.StatusOK{
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
		return nil, &Interfaces.APIStatusError{StatusCode: resp.StatusCode}
	}

	if drain{
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
		return nil, nil
	}

	return resp.Body, nil
}


func (c *HTTPClient) NewRequest(method string, path string, body []byte) (*http.Request, error) {
	
	rel := &url.URL{Path: path}
	endpoint := c.BaseURL.ResolveReference(rel)
	
	if body != nil{
		req, errR:= http.NewRequest(method, endpoint.String(), bytes.NewReader(body))
		if errR != nil{
			return nil, &Interfaces.APIClientError{Action: "NewRequest", Err: errR}
		}

		req.Header.Set("Content-Type", "applicaiton/json")
		return req, nil
	}else{
		req, errR := http.NewRequest(method, endpoint.String(), nil)
		if errR != nil{
			return nil, &Interfaces.APIClientError{Action: "NewRequest", Err: errR}
		}
		req.Header.Set("Content-Type", "applicaiton/json")
		return req, nil
	}

}


func(c *HTTPClient) Do(req *http.Request, drain bool) (*http.Response, error){
	resp, errR := c.client.Do(req)
	if errR != nil {
		var u *url.Error
		if errors.As(errR, &u){
			return nil, &Interfaces.APIClientError{Action: "DoRequest", Err: errR, Op:"ServerIssues"}
		}
		return nil, errR
	}
	
	return resp, nil
}
