package worker

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL    *url.URL
	HTTPClient *http.Client
	Err        error
}

func NewClient(host string, scheme string) *Client {

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
			MaxIdleConns:          30,
			MaxIdleConnsPerHost:   30,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			TLSClientConfig:       TLSConfig,
		},
	}

	c := &Client{
		BaseURL: &url.URL{
			Scheme: scheme,
			Host:   host,
		},
		HTTPClient: h,
	}

	return c
}

func (c *Client) SendMail(message string, server string, endpoint string, drain bool, method string) (interface{}, error) {

	c.Err = nil
	var req *http.Request

	req = c.NewRequest(method, endpoint, message, server)

	if c.Err != nil {
		return nil, c.Err
	}

	resp, err := c.Do(req, drain)
	if err != nil {
		return nil, fmt.Errorf("Cant perform the http request: %v", err)
	}

	return resp, nil
}

func (c *Client) NewRequest(method string, path string, message string, server string) *http.Request {

	rel := &url.URL{Path: path}
	endpoint := c.BaseURL.ResolveReference(rel)

	postdata := url.Values{
		"from":    {"noreply@test.com"},
		"to":      {"altug@test.com"},
		"subject": {"New Process At - " + server},
		"body":    {message},
		"action":  {"sendSES"},
		"html":    {"true"},
	}

	//fmt.Println(strings.NewReader(postdata.Encode()))

	req, errR := http.NewRequest(method, endpoint.String(), strings.NewReader(postdata.Encode()))
	if errR != nil {
		c.Err = fmt.Errorf("Error creating a new http request: %v", errR)
		return nil
	}
	q := url.Values{}
	q.Add("action", "sendSES")
	req.URL.RawQuery = q.Encode()
	//fmt.Println(req.URL.String())

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	return req

}

func (c *Client) Do(req *http.Request, drain bool) (interface{}, error) {

	resp, errR := c.HTTPClient.Do(req)
	if errR != nil {
		return nil, fmt.Errorf("Error perfoming the http request: %v", errR)
	}

	if drain {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
		return nil, nil
	}

	return resp, nil
}
