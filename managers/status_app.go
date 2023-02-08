package managers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/splunk/kube-bundler/api/v1alpha1"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
)

type AppStatusResponse struct {
	AppStatuses []AppStatus `json:"app_statuses"`
}

type AppStatus struct {
	App         string           `json:"app"`
	Endpoints   []EndpointStatus `json:"endpoints"`
	LastUpdated time.Time        `json:"last_updated"`
}

type EndpointStatus struct {
	Endpoint string `json:"endpoint"`
	Status   int    `json:"status"`
}

const (
	interval = 60
	timeout  = 30
)

type AppContext struct {
	App        v1alpha1.Application
	CancelFunc context.CancelFunc
}

type tmplData struct {
	Suffix    string
	Namespace string
}

var appStatusResults = sync.Map{}
var appListTracker = sync.Map{}

var appHealthStatusCheckGauge = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "app_health_status_check",
		Help: "The up/down health check results of every application.",
	}, []string{"app", "endpoint"})

// routinely checks for changes to application list and starts/stops goroutines accordingly
func (sm *StatusManager) updateAppList(ctx context.Context) {
	for {
		updatedMap := make(map[string]struct{})
		var updatedList v1alpha1.ApplicationList
		err := sm.resourceMgr.List(ctx, defaultNamespace, &updatedList)
		if err != nil {
			log.WithField("err", err).Warn("couldn't get applications")
			time.Sleep(30 * time.Second)
			continue
		}

		// create goroutines for new applications
		for _, app := range updatedList.Items {
			updatedMap[app.Name] = struct{}{}
			value, exists := appListTracker.Load(app.Name)

			if !exists {
				appCtx, cancelFunc := context.WithCancel(ctx)
				appContext := AppContext{
					App:        app,
					CancelFunc: cancelFunc,
				}
				appListTracker.Store(app.Name, &appContext)
				go sm.runApplicationCheck(appCtx, app.Name)
			} else {
				// update application object in appListTracker
				value.(*AppContext).App = app
			}
		}

		// stop goroutines for removed applications
		appListTracker.Range(func(key, value interface{}) bool {
			appName := fmt.Sprintf("%v", key)
			_, exists := updatedMap[appName]
			if !exists {
				value.(*AppContext).CancelFunc()
				appListTracker.Delete(appName)
			}
			return true
		})

		time.Sleep(interval * time.Second)
	}
}

func (sm *StatusManager) runApplicationCheck(ctx context.Context, appName string) {
	for {
		value, exists := appListTracker.Load(appName)
		if exists {
			app := value.(*AppContext).App
			appStatus := AppStatus{
				App:         app.Name,
				Endpoints:   []EndpointStatus{},
				LastUpdated: time.Now(),
			}

			var suffix, namespace string
			for _, parm := range app.Spec.ParameterDefinitions {
				if parm.Name == "suffix" {
					suffix = parm.Default
				}
				if parm.Name == "namespace" {
					namespace = parm.Default
				}
				// if this is the ssh daemonset bundle, keep track of the port
				if parm.Name == "ssh_port" {
					sshPort = parm.Default
				}
			}

			for _, status := range app.Spec.Status {
				if len(strings.TrimSpace(status.Endpoint)) != 0 {

					data := tmplData{suffix, namespace}
					t := template.New("Template")
					t, _ = t.Parse(status.Endpoint)
					builder := &strings.Builder{}
					if err := t.Execute(builder, data); err != nil {
						log.WithFields(log.Fields{"err": err, "endpoint": status.Endpoint}).Warn("couldn't parse endpoint")
						continue
					}
					endpoint := builder.String()

					endpointStatus := EndpointStatus{
						Endpoint: endpoint,
						Status:   -1,
					}
					err := sm.endpointHealthCheck(endpoint, status.ExpectedCode)
					if err != nil {
						// Target is down
						appHealthStatusCheckGauge.WithLabelValues(app.Name, endpoint).Set(0)
						log.WithFields(log.Fields{"app": app.Name, "endpoint": endpoint}).Warn(err)
						endpointStatus.Status = 0
					} else {
						appHealthStatusCheckGauge.WithLabelValues(app.Name, endpoint).Set(1)
						endpointStatus.Status = 1
					}
					appStatus.Endpoints = append(appStatus.Endpoints, endpointStatus)
				}
			}
			appStatusResults.Store(app.Name, appStatus)
		}

		timer := time.NewTimer(interval * time.Second)
		select {
		case <-ctx.Done():
			appStatusResults.Delete(appName)
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}
}

func (sm *StatusManager) endpointHealthCheck(endpoint string, expectedCode string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return err
	}

	switch u.Scheme {
	case "http", "https":
		var client http.Client

		// allowing in-secure https call for vault application health check
		if strings.Contains(endpoint, "vault") {
			customTransport := http.DefaultTransport.(*http.Transport).Clone()
			customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
			client = http.Client{Timeout: timeout * time.Second, Transport: customTransport}
		} else {
			client = http.Client{Timeout: timeout * time.Second}
		}

		resp, err := client.Get(endpoint)
		if err != nil {
			// Target is down
			return err
		}
		// If expectedCode is provided, check if status code matches expected code
		if expectedCode != "" {
			statusCodeStr := strconv.Itoa(resp.StatusCode)
			if statusCodeStr != expectedCode {
				// return error - mention both the expectedCode and the actual code
				return errors.New(fmt.Sprintf("GET returned response: %s and the expectedCode is %s", statusCodeStr, expectedCode))
			} else {
				return nil
			}
		}

		// Target is up, but may have a non-200 response
		if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
			return errors.New(fmt.Sprintf("GET returned non-2XX response: %d", resp.StatusCode))
		}
		return nil
	case "tcp":
		// Connect using tcp
		_, err := net.DialTimeout("tcp", u.Host, timeout*time.Second)
		if err != nil {
			return err
		}
		return nil
	}
	return errors.New("Endpoint scheme must be http/https/tcp")
}

func (sm *StatusManager) retrieveApplicationStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := AppStatusResponse{
		AppStatuses: []AppStatus{},
	}

	appStatusResults.Range(func(key, value interface{}) bool {
		response.AppStatuses = append(response.AppStatuses, value.(AppStatus))
		return true
	})

	json.NewEncoder(w).Encode(response)
}
