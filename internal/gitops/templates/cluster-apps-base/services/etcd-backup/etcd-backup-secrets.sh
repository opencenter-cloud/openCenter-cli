kubectl --namespace etcd-backup \
create secret generic etcd-backup-secrets \
--type Opaque \
--from-literal=ACCESS_KEY="" \
--from-literal=SECRET_KEY="" \
--from-literal=S3_HOST="https://swift.api.{{ .OpenCenter.OpenStack.Region }}.rackspacecloud.com" \
--from-literal=S3_REGION="{{ .OpenCenter.OpenStack.Region }}" \
--from-literal=ETCDCTL_API="3" \
--from-literal=ETCDCTL_ENDPOINTS="https://127.0.0.1:2379" \
--from-literal=ETCDCTL_CACERT="/etc/kubernetes/ssl/etcd/ca.crt" \
--from-literal=ETCDCTL_CERT="/etc/kubernetes/ssl/etcd/server.crt" \
--from-literal=ETCDCTL_KEY="/etc/kubernetes/ssl/etcd/server.key"