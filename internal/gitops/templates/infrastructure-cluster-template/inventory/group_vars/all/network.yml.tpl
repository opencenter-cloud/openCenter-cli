{{- $provider := (.OpenCenter.Infrastructure.Provider | default "openstack") -}}
{{- if or (eq $provider "baremetal") (eq $provider "vmware") -}}
---
# Pin kubespray to the interface owning ansible_host (the node's mgmt IP from
# the inventory). Without this, network_facts falls back to ansible_default_ipv4,
# which picks the default-route NIC on multi-NIC boxes and lands
# kube-apiserver/kubelet/etcd on the wrong interface. Note: ansible_all_ipv4_addresses
# is not populated when network_facts runs, so a CIDR-scoped ipaddr filter here
# would evaluate to undefined and fall back to the wrong default anyway.
ip: "{{ "{{" }} ansible_host {{ "}}" }}"
access_ip: "{{ "{{" }} ansible_host {{ "}}" }}"
{{- end }}
