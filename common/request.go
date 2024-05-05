package common

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
)

func NewRequest(method string, url string, body io.Reader, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	resp := http.Client{Transport: tr}
	res, err := resp.Do(req)
	if err != nil {
		return nil, err
	}
	statusOk := strconv.Itoa(res.StatusCode)[0] == '2'
	if !statusOk {
		return nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Println(err)
		}
	}(res.Body)
	return io.ReadAll(res.Body)
}
