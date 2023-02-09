package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	DefaultTimeout   = 30
	SleepBetweenCall = 2

	usage = `Wait for each of one or more targets to successfully respond

Usage of %s:
  %s [flags] target ...

`
)

var timeoutArg int

func init() {
	flag.IntVar(&timeoutArg, "t", DefaultTimeout, "connection timeout in seconds")
}

type Log struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

// print logs in json format
func printLogs(level string, message string) {
	log := &Log{Level: level, Message: message}
	b, _ := json.Marshal(log)
	fmt.Println(string(b))
}

func supportedTarget(target string) bool {
	u, err := url.Parse(target)
	if err != nil {
		return false
	}

	switch u.Scheme {
	case "http", "https", "tcp":
		return true
	}
	return false
}

func connectHost(target string) error {
	timeout := time.Duration(2 * time.Second)
	u, err := url.Parse(target)
	if err != nil {
		return err
	}

	switch u.Scheme {
	case "http":
		fallthrough
	case "https":
		// Connect using http/https
		client := http.Client{Timeout: timeout}
		resp, err := client.Get(target)
		if err != nil {
			return err
		}
		if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
			return errors.New(fmt.Sprintf("GET returned non-2XX response: %d", resp.StatusCode))
		}
		return nil
	case "tcp":
		_, err := net.DialTimeout("tcp", u.Host, timeout)
		return err
	default:
		return fmt.Errorf("unknown scheme: %v", u.Scheme)
	}
}

// validate list of hosts are reachable
// return true if all hosts are reachable else return false
func tryConnect(targetList []string) bool {
	ok := true
	for _, target := range targetList {
		err := connectHost(target)
		if err != nil {
			printLogs("ERROR", fmt.Sprintf("Site unreachable to %s, error: %s\n", target, err))
			ok = false
			return ok
		} else {
			printLogs("INFO", fmt.Sprintf("Site reachable - %s", target))
		}
	}
	return ok
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), usage, os.Args[0], os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	targets := flag.Args()

	// Verify SKIPCHECKS is set
	if os.Getenv("WAITFOR_SKIPCHECKS") == "true" {
		printLogs("INFO", fmt.Sprintf("Skipping waitfor checks for targets: %v", strings.TrimSpace(os.Getenv("TARGETS"))))
		os.Exit(0)
	}

	// Verify targets value is not empty
	if len(targets) == 0 {
		printLogs("FATAL", "target list cannot be empty; please specify one or more targets as non-flag arguments")
		os.Exit(1)
	}

	for _, target := range targets {
		if !supportedTarget(target) {
			printLogs("FATAL", "target missing scheme or scheme not supported: "+target)
			os.Exit(2)
		}
	}

	// do polling for timeout seconds to validate hosts
	start := time.Now()
	for {
		if tryConnect(targets) {
			// if all hosts are reachble, returns the program with exit 0
			printLogs("INFO", "Everything is up")
			os.Exit(0)
		}
		elapsed := time.Since(start)
		end := float64(elapsed) / float64(time.Second)
		if end >= float64(timeoutArg) {
			break
		} else {
			time.Sleep(SleepBetweenCall * time.Second)
		}
	}
	os.Exit(3)
}
