package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/robfig/cron"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"time"
)

// TODO
// - switch to queue system to limit messages sent to slack
// - save somewhere

type Status struct {
	URL     string
	Status  string
	Latency time.Duration
}

type SiteConfig struct {
	URL          string `json:"url"`
	SlackChannel string `json:"channel,omitempty"`
	When         string `json:"cron-expression,omitempty"`
}

type SlackConfig struct {
	Webhook string `json:"WebHook"`
	Channel string `json:"channel"`
}

type Config struct {
	Slack SlackConfig  `json:"slack"`
	When  string       `json:"cron-expression"`
	Sites []SiteConfig `json:"sites"`
}

func main() {
	// Setup config
	configFile, err := ioutil.ReadFile("./pill.json")
	if err != nil {
		log.Fatalf("Error: %v", err)
		os.Exit(1)
	}

	var config Config
	json.Unmarshal(configFile, &config)

	// setup cron jobs
	c := cron.New()
	when := config.When
	sites := config.Sites
	channel := config.Slack.Channel
	webhook := config.Slack.Webhook

	// create msg queue
	queue := make(chan int)

	for i, site := range sites {
		queue <- i
		if site.SlackChannel != "" {
			channel = site.SlackChannel
		}
		if site.When != "" {
			when = site.When
		}
		c.AddFunc(when, func() { trace(site.URL, channel, webhook, queue) })
	}

	limiter := time.Tick(time.Millisecond * 200)

	for _ := range queue {
		<-limiter
	}

	c.Start() // start...
	select {} // and don't stop
	close(queue)
}

func trace(URL, channel, webhook string, queue chan<- Status) {
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

	latency := endTime.Sub(startTime)
	if resp.StatusCode != 200 {
		reportProblem(URL, channel, webhook, latency, resp)
	}

	save(URL, resp.StatusCode, latency)
	return
}

func save(URL string, code int, latency time.Duration) {
	log.Printf("GET\t%s (%v) - %v", URL, code, latency)
}

func reportProblem(URL, channel, webhook string, latency time.Duration, resp *http.Response) {
	statusCode := resp.StatusCode
	msg := fmt.Sprintf("%s isn't swallowing any pills\n> (%v:%v) %s", URL, statusCode, latency, http.StatusText(statusCode))
	pingSlack(msg, channel, webhook)
}

func pingSlack(msg, channel, webhook string) {
	data := url.Values{}
	username := "Pill Pusher"
	icon := ":pill:"
	payload := fmt.Sprintf("{'channel': '%s', 'username': '%s', 'text': '%s', 'icon_emoji': '%s'}", channel, username, msg, icon)
	data.Set("payload", payload)

	client := &http.Client{}
	req, _ := http.NewRequest("POST", webhook, bytes.NewBufferString(data.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		log.Print(err.Error())
	}
	if resp.StatusCode != 200 {
		log.Print(resp.Status)
	}
}
