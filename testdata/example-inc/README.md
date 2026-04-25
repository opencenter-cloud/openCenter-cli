# example-platform

This sanitized fixture models a synthetic GitOps repository with five imported
clusters. It intentionally keeps only the fields the importer scanner needs and
does not include customer names, account numbers, or production hostnames.

### DEV (k8s-dev)

- **Region:** IAD3
- **Infrastructure:** VMware vSphere - IAD3

#### Cluster details
```
NAME               STATUS   ROLES           AGE   VERSION
k8s-dev-iad3-cp0   Ready    control-plane   90d   v1.32.8
k8s-dev-iad3-cp1   Ready    control-plane   90d   v1.32.8
k8s-dev-iad3-cp2   Ready    control-plane   90d   v1.32.8
k8s-dev-iad3-wn0   Ready    worker          90d   v1.32.8
k8s-dev-iad3-wn1   Ready    worker          90d   v1.32.8
```

### QA (k8s-qa)

- **Region:** ORD1
- **Infrastructure:** VMware vSphere - ORD1

#### Cluster details
```
NAME              STATUS   ROLES           AGE   VERSION
k8s-qa-ord1-cp0   Ready    control-plane   30d   v1.32.8
k8s-qa-ord1-cp1   Ready    control-plane   30d   v1.32.8
k8s-qa-ord1-cp2   Ready    control-plane   30d   v1.32.8
k8s-qa-ord1-wn0   Ready    worker          30d   v1.32.8
k8s-qa-ord1-wn1   Ready    worker          30d   v1.32.8
k8s-qa-ord1-wn2   Ready    worker          30d   v1.32.8
```

### UAT (k8s-uat)

- **Region:** ORD1
- **Infrastructure:** VMware vSphere - ORD1

#### Cluster details
```
NAME               STATUS   ROLES           AGE   VERSION
k8s-uat-ord1-cp0   Ready    control-plane   45d   v1.32.8
k8s-uat-ord1-cp1   Ready    control-plane   45d   v1.32.8
k8s-uat-ord1-cp2   Ready    control-plane   45d   v1.32.8
k8s-uat-ord1-wn0   Ready    worker          45d   v1.32.8
k8s-uat-ord1-wn1   Ready    worker          45d   v1.32.8
k8s-uat-ord1-wn2   Ready    worker          45d   v1.32.8
```

### DR (k8s-dr)

- **Region:** IAD3
- **Infrastructure:** VMware vSphere - IAD3

#### Cluster details
```
NAME              STATUS   ROLES           AGE   VERSION
k8s-dr-iad3-cp0   Ready    control-plane   60d   v1.32.8
k8s-dr-iad3-cp1   Ready    control-plane   60d   v1.32.8
k8s-dr-iad3-cp2   Ready    control-plane   60d   v1.32.8
k8s-dr-iad3-wn0   Ready    worker          60d   v1.32.8
k8s-dr-iad3-wn1   Ready    worker          60d   v1.32.8
```

### PROD (k8s-prod)

- **Region:** ORD1
- **Infrastructure:** VMware vSphere - ORD1

#### Cluster details
```
NAME                STATUS   ROLES           AGE   VERSION
k8s-prod-ord1-cp0   Ready    control-plane   120d  v1.32.7
k8s-prod-ord1-cp1   Ready    control-plane   120d  v1.32.7
k8s-prod-ord1-cp2   Ready    control-plane   120d  v1.32.7
k8s-prod-ord1-wn0   Ready    worker          120d  v1.32.7
k8s-prod-ord1-wn1   Ready    worker          120d  v1.32.7
k8s-prod-ord1-wn2   Ready    worker          120d  v1.32.7
```
