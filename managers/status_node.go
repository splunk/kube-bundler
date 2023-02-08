package managers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NodeInfo struct {
	Name      string `json:"name"`
	IpAddress string `json:"ipAddress"`
	Role      string `json:"role,omitempty"`
}

type NodeStatus struct {
	Node      NodeInfo       `json:"node"`
	Status    string         `json:"status,omitempty"`
	Condition []DetailStatus `json:"condition,omitempty"`
	Network   []DetailStatus `json:"network,omitempty"`
	Preflight []DetailStatus `json:"preflight,omitempty"`
}

type DetailStatus struct {
	Name    string `json:"name,omitempty"`
	Value   string `json:"value,omitempty"`
	Healthy string `json:"healthy,omitempty"`
}

const (
	getNodeStatusInterval = 1 * time.Minute
	getNodesInterval      = 5 * time.Minute
)

var k0sNode = sync.Map{}
var clusterNodesResults = sync.Map{}
var clusterListTracker = sync.Map{}

//the default value of the ssh daemonset port (changes to port are updated by application status)
var sshPort = "2222"

var clusterChecks = map[string]string{
	"MemoryPressure": "False",
	"PIDPressure":    "False",
	"Ready":          "True",
}

var clusterNodeCheckGauge = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "cluster_node_check",
		Help: "The up/down check results of every node.",
	}, []string{"node", "ipAddress", "role"})

var clusterNodeStatusCheckGauge = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "cluster_node_status_check",
		Help: "The up/down detailed health check results of every node.",
	}, []string{"node", "category", "check_name"})

func (sm *StatusManager) updateClusterList(ctx context.Context) {
	for {
		updatedMap := make(map[string]struct{})
		nodes, err2 := sm.kbClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err2 != nil {
			log.WithField("err", err2).Warn("couldn't get cluster nodes")
		} else {
			// create goroutines for new cluster node
			for _, node := range nodes.Items {
				updatedMap[node.Name] = struct{}{}
				k0sNode.Store(node.Name, node)
				_, exists := clusterListTracker.Load(node.Name)
				if !exists {
					clusterCtx, cancelFunc := context.WithCancel(ctx)
					clusterListTracker.Store(node.Name, cancelFunc)
					go sm.runNodeCheck(clusterCtx, node)
				}
			}

			// stop goroutines for removed applications
			clusterListTracker.Range(func(key, value interface{}) bool {
				_, exists := updatedMap[key.(string)]
				if !exists {
					cancelFunc := value.(context.CancelFunc)
					cancelFunc()
					clusterListTracker.Delete(key)
					k0sNode.Delete(key)
					clusterNodesResults.Delete(key)
				}
				return true
			})
		}

		time.Sleep(getNodesInterval)
	}
}

func (sm *StatusManager) runNodeCheck(ctx context.Context, node v1.Node) {

	for {
		value, _ := k0sNode.Load(node.Name)
		nodeValue := value.(v1.Node)
		nodeIp := nodeValue.Status.Addresses[0].Address

		detailConditionStatus := sm.checkNodeConditionStatus(node.Name)
		sshDaemonsetStatus := sm.checkSshDaemonset(nodeIp)

		client := http.Client{Timeout: timeout * time.Second}
		resp, err := client.Get("http://" + nodeIp + ":9000/status")
		if err != nil {
			log.WithFields(log.Fields{"node": node.Name, "err": err}).Warn("couldn't get node status")
			nodeStatusWithError := NodeStatus{
				Node: NodeInfo{
					Name:      nodeValue.Name,
					IpAddress: nodeValue.Status.Addresses[0].Address,
					Role:      nodeValue.Labels["node.k0sproject.io/role"],
				},
				Status: "Not Available",
				Condition: append(detailConditionStatus,
					DetailStatus{
						Name:    "GetStatus",
						Value:   err.Error(),
						Healthy: "false",
					}),
			}
			nodeInfo := nodeStatusWithError.Node
			clusterNodeCheckGauge.WithLabelValues(nodeInfo.Name, nodeInfo.IpAddress, nodeInfo.Role).Set(0)
			clusterNodesResults.Store(nodeValue.Name, nodeStatusWithError)
		} else {
			defer func(Body io.ReadCloser) {
				err := Body.Close()
				if err != nil {

				}
			}(resp.Body)

			decoder := json.NewDecoder(resp.Body)
			var nodeStatus NodeStatus
			err2 := decoder.Decode(&nodeStatus)
			if err2 != nil {
				log.WithFields(log.Fields{"node": node.Name, "err": err2}).Warn("couldn't parse node status")
				nodeStatusWithError := NodeStatus{
					Node: NodeInfo{
						Name:      nodeValue.Name,
						IpAddress: nodeValue.Status.Addresses[0].Address,
						Role:      nodeValue.Labels["node.k0sproject.io/role"],
					},
					Status: "Not Available",
					Condition: append(detailConditionStatus,
						DetailStatus{
							Name:    "GetStatus",
							Value:   err2.Error(),
							Healthy: "false",
						}),
				}
				nodeInfo := nodeStatusWithError.Node
				clusterNodeCheckGauge.WithLabelValues(nodeInfo.Name, nodeInfo.IpAddress, nodeInfo.Role).Set(0)
				clusterNodesResults.Store(nodeValue.Name, nodeStatusWithError)
			} else {
				nodeStatus.Condition = detailConditionStatus
				nodeStatus.Preflight = append(nodeStatus.Preflight, sshDaemonsetStatus)
				// Adding network and preflight check results into prometheus gauges
				for _, detail := range nodeStatus.Network {
					if detail.Healthy == "true" {
						clusterNodeStatusCheckGauge.WithLabelValues(nodeValue.Name, "network", "ping to "+detail.Name).Set(1)
					} else {
						clusterNodeStatusCheckGauge.WithLabelValues(nodeValue.Name, "network", "ping to "+detail.Name).Set(0)
					}
				}
				for _, detail := range nodeStatus.Preflight {
					if detail.Name == "diskCapacity" {
						percent, _ := strconv.ParseFloat(strings.TrimSuffix(detail.Value, "%"), 8)
						clusterNodeStatusCheckGauge.WithLabelValues(nodeValue.Name, "system", detail.Name).Set(percent)
					} else {
						if detail.Healthy == "true" {
							clusterNodeStatusCheckGauge.WithLabelValues(nodeValue.Name, "system", detail.Name).Set(1)
						} else {
							clusterNodeStatusCheckGauge.WithLabelValues(nodeValue.Name, "system", detail.Name).Set(0)
						}
					}
				}
				nodeInfo := nodeStatus.Node
				clusterNodeCheckGauge.WithLabelValues(nodeInfo.Name, nodeInfo.IpAddress, nodeInfo.Role).Set(1)
				clusterNodesResults.Store(nodeStatus.Node.Name, nodeStatus)
			}
		}

		timer := time.NewTimer(getNodeStatusInterval)
		select {
		case <-ctx.Done():
			k0sNode.Delete(node.Name)
			clusterNodesResults.Delete(node.Name)
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}
}

func (sm *StatusManager) retrieveClusterNodes(w http.ResponseWriter, r *http.Request) {

	var response []NodeInfo

	clusterNodesResults.Range(func(key, value interface{}) bool {
		nodeStatus := value.(NodeStatus)
		response = append(response, nodeStatus.Node)
		return true
	})

	json.NewEncoder(w).Encode(response)
}

func (sm *StatusManager) retrieveClusterStatus(w http.ResponseWriter, r *http.Request) {

	var response []NodeStatus

	clusterNodesResults.Range(func(key, value interface{}) bool {
		nodeStatus := value.(NodeStatus)
		response = append(response, nodeStatus)
		return true
	})

	json.NewEncoder(w).Encode(response)
}

func (sm *StatusManager) checkNodeConditionStatus(nodeName string) []DetailStatus {

	var detailStatuses []DetailStatus
	value, ok := k0sNode.Load(nodeName)
	if !ok {
		return nil
	} else {

		var nodeData = value.(v1.Node)
		for _, condition := range nodeData.Status.Conditions {
			detailStatus := DetailStatus{}
			if val, ok := clusterChecks[string(condition.Type)]; ok {
				detailStatus.Name = string(condition.Type)
				detailStatus.Value = string(condition.Status)
				if string(condition.Status) == val {
					clusterNodeStatusCheckGauge.WithLabelValues(nodeName, "condition", string(condition.Type)).Set(1)
					detailStatus.Healthy = "true"
				} else {
					clusterNodeStatusCheckGauge.WithLabelValues(nodeName, "condition", string(condition.Type)).Set(0)
					detailStatus.Healthy = "false"
				}
				detailStatuses = append(detailStatuses, detailStatus)
			}
		}
	}

	return detailStatuses
}

func (sm *StatusManager) checkSshDaemonset(nodeIp string) DetailStatus {
	detailStatus := DetailStatus{}
	err := sm.endpointHealthCheck("tcp://"+nodeIp+":"+sshPort, "")
	if err != nil {
		detailStatus = DetailStatus{"sshDaemonset", "down", "false"}
	} else {
		detailStatus = DetailStatus{"sshDaemonset", "up", "true"}
	}
	return detailStatus
}
