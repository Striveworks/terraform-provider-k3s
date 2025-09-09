
output "nodes" {
  value = openstack_compute_instance_v2.k8s_node[*].access_ip_v4
}
output "user" {
  value     = var.user
  sensitive = true
}
