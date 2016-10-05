package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/robfig/cron"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptrace"
	"os"
	"time"
)

// TODO
// - slack notification
// - save somewhere

type SlackConfig struct {
	SlackKey     string `json:"key"`
	SlackChannel string `json:"channel"`
}

type Config struct {
	Slack SlackConfig `json:"slack"`
	When  string      `json:"cron-expression"`
	Sites []string    `json:"sites"`
}

func main() {
	configFile, err := ioutil.ReadFile("./healthconfig.json")
	if err != nil {
		log.Fatalf("Error: %v", err)
		os.Exit(1)
	}

	var config Config
	json.Unmarshal(configFile, &config)
	c := cron.New()
	when := config.When
	sites := config.Sites

	for _, site := range sites {
		c.AddFunc(when, func() { trace(site) })
	}

	c.Start() // start...
	select {} // and don't stop
}

func trace(URL string) {
	req, _ := http.NewRequest("GET", URL, nil)

	var startTime, DNSTime time.Time
	trace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) { startTime = time.Now() },
		DNSDone:  func(_ httptrace.DNSDoneInfo) { DNSTime = time.Now() },
		ConnectStart: func(_, _ string) {
			if DNSTime.IsZero() {
				DNSTime = time.Now()
			}
		},
		ConnectDone: func(net, addr string, err error) {
			if err != nil {
				log.Fatalf("unable to connect to host %v: %v", addr, err)
			}
		},
	}

	client := &http.Client{}
	req = req.WithContext(httptrace.WithClientTrace(context.Background(), trace))
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("failed to read response: %v", err)
	}

	resp.Body.Close()

	endTime := time.Now()
	if startTime.IsZero() {
		startTime = DNSTime
	}

	if resp.StatusCode != 200 {
		reportProblem(URL, resp)
	}

	// save(URL, resp.StatusCode, endTime.Sub(startTime))
	log.Printf("GET\t%s (%v) - %v", URL, resp.StatusCode, endTime.Sub(startTime))
	return
}

func reportProblem(URL string, resp *http.Response) {
	statusCode := resp.StatusCode
	msg := fmt.Sprintf("%s - (%v) %s", URL, statusCode, http.StatusText(statusCode))
	pingSlack(msg)
}

func pingSlack(msg string) {
	log.Println(msg)
}
