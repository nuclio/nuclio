# Known issues

## 503 Error Code While Scaling Down (Kubernetes Only)

This is a rare issue that primarily occurs in low-latency setups.
When scaling down a Nuclio function pod, Kubernetes may return a 503 error due to the delay between sending a `SIGTERM` signal and stopping traffic to the pod.

To mitigate this, we've introduced a 5-second wait time before halting event processing after receiving `SIGTERM`. However, this does not fully eliminate the issue.

**Possible Solution**: Implement a client-side retry mechanism when a 503 response is received with an empty body.