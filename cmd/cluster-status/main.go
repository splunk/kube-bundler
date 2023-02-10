package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/splunk/kube-bundler/managers"
)

const (
	Online            string = "online"
	Offline                  = "offline"
	Healthy                  = "Healthy"
	HealthyValue             = "true"
	NotHealthy               = "Not Healthy"
	NotHealthyValue          = "false"
	CheckNodeInterval        = 1 * time.Minute
	GetNodesInterval         = 5 * time.Minute
)

var sysctlChecks = map[string][]string{
	"ipForwarding":  {"net.ipv4.ip_forward", "1"},
	"routeLocalnet": {"net.ipv4.conf.all.route_localnet", "1"},
}

var NodeNetworkStatus = sync.Map{}

var NodeStatusResults = sync.Map{}

var nodeListTracker = sync.Map{}

var port = os.Getenv("PORT")
var nodeName = os.Getenv("NODE_NAME")
var healthStatusDeploymentUrl = "http://app-status.default:8080/"

func main() {
	handler := http.NewServeMux()
	log.Printf("Daemonset Port Number --> %s", port)
	ctx := context.Background()

	go updateClusterNodes(ctx)

	// "/online" endpoint returns 200 ok
	send200 := func(w http.ResponseWriter, r *http.Request) {}
	handler.HandleFunc("/online", send200)
	handler.HandleFunc("/status", retrieveNodeStatus)

	err := http.ListenAndServe(":"+port, handler)
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Warn("couldn't start http server")
		return
	}

}

func updateClusterNodes(ctx context.Context) {
	for {
		updatedMap := make(map[string]struct{})
		nodes, err := getClusterNodes()
		if err != nil {
			log.WithField("err", err).Warn("Couldn't get cluster nodes")
		} else {

			// create goroutines for new cluster node
			for _, node := range nodes {
				updatedMap[node.Name] = struct{}{}
				_, exists := nodeListTracker.Load(node.Name)
				if !exists {
					nodeCtx, cancelFunc := context.WithCancel(ctx)
					nodeListTracker.Store(node.Name, cancelFunc)
					if node.Name == nodeName {
						go getStatus(nodeCtx, node)
					} else {
						go checkNodeOnline(nodeCtx, node)
					}
				}
			}

			// stop goroutines for removed nodes
			nodeListTracker.Range(func(key, value interface{}) bool {
				_, exists := updatedMap[key.(string)]
				if !exists {
					cancelFunc := value.(context.CancelFunc)
					cancelFunc()
					nodeListTracker.Delete(key)
					NodeNetworkStatus.Delete(key)
				}
				return true
			})
		}

		time.Sleep(GetNodesInterval)
	}
}

func retrieveNodeStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	value, exists := NodeStatusResults.Load(nodeName)
	if !exists {
		return
	}

	err := json.NewEncoder(w).Encode(value.(managers.NodeStatus))
	if err != nil {
		return
	}
}

func getClusterNodes() ([]managers.NodeInfo, error) {

	var client = &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(healthStatusDeploymentUrl + "cluster-nodes")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var nodeInfoList []managers.NodeInfo
	err = json.NewDecoder(resp.Body).Decode(&nodeInfoList)
	if err != nil {
		return nil, err
	}

	return nodeInfoList, nil
}

func getStatus(ctx context.Context, nodeInfo managers.NodeInfo) {
	for {
		// The result of the network check should not determine whether a node itself is healthy or not
		_, networkStatus := getNetworkStatus()
		nodeStatus := managers.NodeStatus{
			Node: nodeInfo,
			//Status:    preflightHealthy,
			Network: networkStatus,
			//Preflight: preflightStatus,
		}
		NodeStatusResults.Store(nodeInfo.Name, nodeStatus)

		time.Sleep(CheckNodeInterval)
	}
}

// Check node in cluster is online and update result in map
func checkNodeOnline(ctx context.Context, nodeInfo managers.NodeInfo) {
	for {
		detailStatus := managers.DetailStatus{
			Name: nodeInfo.Name,
		}
		client := http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get("http://" + nodeInfo.IpAddress + ":" + port + "/online")
		if err != nil {
			log.WithFields(log.Fields{"err": err, "node": nodeInfo.Name}).Warn("Couldn't reach node")
			detailStatus.Value = Offline
			detailStatus.Healthy = NotHealthyValue
		} else {
			if resp.StatusCode == 200 {
				detailStatus.Value = Online
				detailStatus.Healthy = HealthyValue
			} else {
				detailStatus.Value = Offline
				detailStatus.Healthy = NotHealthyValue
			}
		}

		NodeNetworkStatus.Store(nodeInfo.Name, detailStatus)

		timer := time.NewTimer(CheckNodeInterval)
		select {
		case <-ctx.Done():
			NodeNetworkStatus.Delete(nodeInfo.Name)
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}

}

func getNetworkStatus() (string, []managers.DetailStatus) {

	var detailStatuses []managers.DetailStatus
	var isHealthy = Healthy

	NodeNetworkStatus.Range(func(key, value interface{}) bool {
		detailStatus := value.(managers.DetailStatus)
		if detailStatus.Healthy == "false" {
			isHealthy = NotHealthy
		}
		detailStatuses = append(detailStatuses, detailStatus)
		return true
	})

	return isHealthy, detailStatuses
}

func checkSysctlParameters() (string, []managers.DetailStatus) {
	isHealthy := Healthy
	var detailStatuses []managers.DetailStatus

	for checkName, paramInfo := range sysctlChecks {
		param := paramInfo[0]
		expected := paramInfo[1]

		output, err := exec.Command("sysctl", "-n", param).Output()
		if err != nil {
			log.WithFields(log.Fields{"err": err, "sysctlParam": param}).Warn("couldn't get sysctl parameter")
			isHealthy = NotHealthy
			detailStatuses = append(detailStatuses, managers.DetailStatus{checkName, err.Error(), NotHealthyValue})
		} else {
			value := strings.TrimSuffix(string(output), "\n")
			if value != expected {
				log.WithFields(log.Fields{"sysctlParam": param, "expectedValue": expected}).Warn("Sysctl preflight check failed: parameter did not have expected value")
				isHealthy = NotHealthy
				detailStatuses = append(detailStatuses, managers.DetailStatus{checkName, value, NotHealthyValue})
			} else {
				detailStatuses = append(detailStatuses, managers.DetailStatus{checkName, value, HealthyValue})
			}
		}
	}

	return isHealthy, detailStatuses
}
