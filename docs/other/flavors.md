# Application Flavors

Flavors are predefined application configurations used by bundles to automatically apply a desired configuration. Flavors are primarily used by the bundle to determine how many replicas to deploy for failure tolerance or performance.

By default, there is an application flavor called `default` that specifies a single replica for all replica parameters. The `default` flavor is used when no flavor is specified.

Bundles use the configured flavor to configure themeselves according to the flavor's parameters.

Note that flavors are simply a mechanism to enforce consistent configuration, and the services provided by the bundles must have intrinsic support for the functionality described by the configuration. The bundle *must* apply this configuration in the correct places (e.g. a Statefulset's `replicas` field) for this property to hold. Flavors do not make a service automatically HA!

Flavors are defined by the user and currently support the following fields:

* `statefulQuorumReplicas` - the number of replicas that a quorum-based stateful service should run. This number is usually odd and greater numbers can tolerate more failures. Quorum-based services are services such as zookeeper or etcd would usually deploy 3 or 5 replicas and tolerate failure in typical N/2+1 fashion (where N is the number of total replicas).

In the case of 3 replicas, the service can tolerate losing 1 replica and continue to function. In the case of 5, the service can tolerate losing 2 replicas and continue to function. Replica loss beyond this tolerance means the quorum-based service would fail.

* `statefulReplicationReplicas` - the number of replicas that a replication-based stateful service should run. This number is usually one greater than the number of tolerated failures and greater numbers can tolerate more failures. Replication-based services are services such as a replicated SQL database where all data is synchronously or asynchronously copied to another node. In the case of failure of the primary copy, one of the replicated copies is usually promoted to being primary.

In the case of 2 replicas, the service can tolerate the loss of 1 replica. In the case of 3 replicas, the service can tolerate the loss of 2 replicas. Replica loss beyond this tolerance typically means data would be lost.

* `statelessReplicas` - the number of replicas that a stateless service should run. The typical minimum for a production-ready service is 3, but applications may choose to increase the number of replicas for performance or other reasons.

* `antiAffinity` - whether services should apply required or optional anti-affinity. Valid values are `optional` and `required`. When using optional, the kubernetes scheduler may place more than one replica on a single node.

* `minimumNodes` - the minimum number of nodes the flavor requires to install. This prevents installing on a cluster that won’t support the flavor requirements

## Example Flavor: ha3

Assuming we have an application that runs on a minimum of 3 nodes and wants to support the best possible HA configuration, we can build a flavor that will tolerate the loss of any 1 node in the cluster (and the corresponding loss of 1 replica of each type service, whether they be stateful quorum, stateful replication, or stateless applications).

In addition, we will set `antiAffinity: required` to ensure that no more than 1 replica of each service is scheduled to a given node.

```
apiVersion: bundle.splunk.com/v1alpha1
kind: Flavor
metadata:
  name: ha3
spec:
  statefulQuorumReplicas: 3
  statefulReplicationReplicas: 2
  statelessReplicas: 3
  antiAffinity: required
  minimumNodes: 3
```

An `ha3` cluster is allowed to have more than 3 nodes, but note that additional nodes beyond the initial 3 used to schedule services do not contribute to the availability properties of the cluster. They simply add extra capacity.

## Example Flavor: ha5

Similarly, we can build an `ha5` flavor that will tolerate the loss of any 2 nodes.

```
apiVersion: bundle.splunk.com/v1alpha1
kind: Flavor
metadata:
  name: ha5
spec:
  statefulQuorumReplicas: 5
  statefulReplicationReplicas: 3
  statelessReplicas: 3
  antiAffinity: required
  minimumNodes: 5
```
