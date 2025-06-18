package provider

// func TestAccExampleDataSource(t *testing.T) {
// 	resource.Test(t, resource.TestCase{
// 		PreCheck:                 func() { testAccPreCheck(t) },
// 		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
// 		Steps: []resource.TestStep{
// 			// Read testing
// 			{
// 				Config: providerConfig + `
// data "k3s_config" "server" {
//   data_dir = "/etc/k3s"
//   config  = yamlencode({
// 	  "etcd-expose-metrics" = "" // flag for true
// 	  "etcd-s3-timeout"     = "5m30s",
// 	  "node-label"		    = ["foo=bar"]
// 	})
// }
// 				`,
// 				ConfigStateChecks: []statecheck.StateCheck{
// 					statecheck.ExpectKnownValue(
// 						"data.k3s_config.server",
// 						tfjsonpath.New("data_dir"),
// 						knownvalue.StringExact("/etc/k3s"),
// 					),
// 				},
// 			},
// 		},
// 	})

// 	resource.Test(t, resource.TestCase{
// 		PreCheck:                 func() { testAccPreCheck(t) },
// 		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
// 		Steps: []resource.TestStep{
// 			// Read testing
// 			{
// 				Config: providerConfig + `
// data "k3s_config" "server" {
//   config  = yamlencode({
// 	  "etcd-expose-metrics" = "" // flag for true
// 	  "etcd-s3-timeout"     = "5m30s",
// 	  "node-label"		    = ["foo=bar"]
// 	})
// }`,
// 				ConfigStateChecks: []statecheck.StateCheck{
// 					statecheck.ExpectKnownValue(
// 						"data.k3s_config.server",
// 						tfjsonpath.New("data_dir"),
// 						knownvalue.StringExact("/var/lib/rancher/k3s"),
// 					),
// 				},
// 			},
// 		},
// 	})
// }
