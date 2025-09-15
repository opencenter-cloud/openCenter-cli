# Hardening
---

## kube-apiserver
authorization_modes: ['Node', 'RBAC']
kube_apiserver_request_timeout: 120s
kube_apiserver_service_account_lookup: true

# enable kubernetes audit
kubernetes_audit: true
audit_log_path: "/var/log/kube-apiserver-log.json"
audit_log_maxage: 30
audit_log_maxbackups: 10
audit_log_maxsize: 100

tls_min_version: VersionTLS12
tls_cipher_suites:
  - TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
  - TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
  - TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305

# enable encryption at rest
kube_encrypt_secret_data: true
kube_encryption_resources: [secrets]
kube_encryption_algorithm: "secretbox"

kube_apiserver_enable_admission_plugins:
  - EventRateLimit
  - AlwaysPullImages
  - ServiceAccount
  - NamespaceLifecycle
  - NodeRestriction
  - LimitRanger
  - ResourceQuota
  - MutatingAdmissionWebhook
  - ValidatingAdmissionWebhook
  - PodNodeSelector
  - PodSecurity
kube_apiserver_admission_control_config_file: true
# # Creates config file for PodNodeSelector
# # kube_apiserver_admission_plugins_needs_configuration: [PodNodeSelector]
# # Define the default node selector, by default all the workloads will be scheduled on nodes
# # with label network=srv1
# # kube_apiserver_admission_plugins_podnodeselector_default_node_selector: "network=srv1"
# # EventRateLimit plugin configuration
kube_pod_security_use_default: true
kube_pod_security_default_enforce: baseline
kube_pod_security_default_enforce_version: "v{{ kube_major_version }}"
kube_pod_security_default_audit: restricted
kube_pod_security_default_audit_version: "v{{ kube_major_version }}"
kube_pod_security_default_warn: restricted
kube_pod_security_default_warn_version: "v{{ kube_major_version }}"
kube_pod_security_exemptions_usernames: []
kube_pod_security_exemptions_runtime_class_names: []
kube_pod_security_exemptions_namespaces:
  - kube-system
  - tigera-operator
%{ if length(kube_pod_security_exemptions_namespaces) > 0 ~}
%{ for ns in kube_pod_security_exemptions_namespaces ~}
  - ${ns}
%{ endfor ~}
%{endif ~}

kube_apiserver_admission_event_rate_limits:
  limit_1:
    type: Namespace
    qps: 100
    burst: 150
    cache_size: 2000
  limit_2:
    type: User
    qps: 100
    burst: 150
kube_profiling: true
# Remove anonymous access to cluster. https://github.com/kubernetes-sigs/kubespray/issues/11835 leave it to false until this is included in a release
remove_anonymous_access: false

# ## kube-controller-manager
kube_controller_manager_bind_address: 127.0.0.1
kube_controller_terminated_pod_gc_threshold: 50
kube_controller_feature_gates: ["RotateKubeletServerCertificate=true"] # False until I figure how to deploy a helm chart after the cni is deployed

## kube-scheduler
kube_scheduler_bind_address: 127.0.0.1

## etcd
etcd_deployment_type: kubeadm

# ## kubelet
kubelet_authorization_mode_webhook: true
kubelet_authentication_token_webhook: true
kube_read_only_port: 0
# Note that enabling this also activates *kubelet_csr_approver* which approves automatically the CSRs.
#  To customize its behavior, you can override the Helm values via *kubelet_csr_approver_values*.
kubelet_rotate_server_certificates: ${kubelet_rotate_server_certificates}
kubelet_protect_kernel_defaults: true
kubelet_event_record_qps: 1
kubelet_rotate_certificates: true
kubelet_streaming_connection_idle_timeout: "5m"
kubelet_make_iptables_util_chains: true
kubelet_feature_gates: ["RotateKubeletServerCertificate=true"]
kubelet_seccomp_default: true
kubelet_systemd_hardening: true
# Need to set this since we are not using a dns to resolve hostnames
kubelet_csr_approver_values:
  bypassDnsResolution: true
  providerRegex: "^${cluster_name}"
# # In case you have multiple interfaces in your
# # control plane nodes and you want to specify the right
# # IP addresses, kubelet_secure_addresses allows you
# # to specify the IP from which the kubelet
# # will receive the packets.
%{ if network_plugin == "kube-ovn" ~}
kubelet_secure_addresses: "localhost link-local {{ kube_pods_subnets | regex_replace(',', ' ') }} {{ kube_node_addresses }} {{ loadbalancer_apiserver.address | default('') }} ${subnet_join}"
%{ else ~}
kubelet_secure_addresses: "localhost link-local {{ kube_pods_subnets | regex_replace(',', ' ') }} {{ kube_node_addresses }} {{ loadbalancer_apiserver.address | default('') }}"
%{ endif ~}
# # additional configurations
kube_owner: root
kube_cert_group: root

# # create a default Pod Security Configuration and deny running of insecure pods
# # kube_system namespace is exempted by default
# kube_pod_security_use_default: true
# kube_pod_security_default_enforce: restricted
