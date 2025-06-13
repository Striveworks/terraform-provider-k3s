output "ssh_key" {
  value     = tls_private_key.ssh_keys.private_key_openssh
  sensitive = true
}
output "nodes" {
  value     = openstack_compute_instance_v2.k8s_node[*].access_ip_v4
  sensitive = true
}
output "user" {
  value     = var.user
  sensitive = true
}
