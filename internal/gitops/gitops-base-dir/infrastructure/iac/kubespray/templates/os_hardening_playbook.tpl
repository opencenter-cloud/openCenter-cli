---

- name: Harden all OS Systems
  hosts: k8s_cluster
  become: yes
  roles:
    - ansible-hardening