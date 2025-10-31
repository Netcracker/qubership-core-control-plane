Test scenario
- Deploy cloud core and istio core components to correspanding namespaces
- Deploy and register test service
```
    kubectl apply -f test_service.yaml
```
- Run test kubernetes job cloud core with no istio enabled for workspace
```
    kubectl apply -f test_job.yaml
```
- Label namespace to work with istio
```
    kubectl label namespace cloud-core istio.io/dataplane-mode=ambient
```
- Run test kubernetes job cloud core with istio enabled for workspace
- Label namespace to work with istio waypoint
```
    kubectl label ns cloud-core istio.io/use-waypoint=waypoint
```
- Run test kubernetes job cloud core with istio and its waypoint enabled for workspace