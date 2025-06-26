package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestK3sHAServerValidateResource(t *testing.T) {

	resource.Test(t, resource.TestCase{
		IsUnitTest:               true,
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			PlanOnly:    true,
			Config:      providerConfig + `resource "k3s_ha_server" "main" {}`,
			ExpectError: regexp.MustCompile(`(.*)K3s High Availability must be \>\=3 and odd in count(.*)`),
		}},
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		IsUnitTest:               true,
		Steps: []resource.TestStep{{
			PlanOnly: true,
			Config: providerConfig + `resource "k3s_ha_server" "main" {
				node {
					host="192.168.1.1"
					password="abc123"
				}
				node {
					host="192.168.1.2"
					password="abc123"
				}
				node {
					host="192.168.1.3"
					password="abc123"
				}
				node {
					host="192.168.1.4"
					password="abc123"
				}
			}`,
			ExpectError: regexp.MustCompile(`(.*)K3s High Availability must be \>\=3 and odd in count(.*)`),
		}},
	})
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		IsUnitTest:               true,
		Steps: []resource.TestStep{{
			PlanOnly: true,
			Config: providerConfig + `resource "k3s_ha_server" "main" {
				node {
					host="192.168.1.1"
					password="abc123"
				}
				node {
					host="192.168.1.2"
				}
				node {
					host="192.168.1.3"
					password="abc123"
				}
			}`,
			ExpectError: regexp.MustCompile(`(.*)Error: Missing auth(.*)`),
		}},
	})
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		IsUnitTest:               true,
		Steps: []resource.TestStep{{
			PlanOnly:           true,
			ExpectNonEmptyPlan: true,
			Config: providerConfig + `resource "k3s_ha_server" "main" {
				node {
					host="192.168.1.1"
					password="abc123"
				}
				node {
					host="192.168.1.2"
					password="abc123"
				}
				node {
					host="192.168.1.3"
					password="abc123"
				}
			}`,
		}},
	})
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		IsUnitTest:               true,
		Steps: []resource.TestStep{{
			PlanOnly:           true,
			ExpectNonEmptyPlan: true,
			Config: providerConfig + `resource "k3s_ha_server" "main" {
				password="abc123"
				node {
					host="192.168.1.1"
				}
				node {
					host="192.168.1.2"
				}
				node {
					host="192.168.1.3"
				}
			}`,
		}},
	})
}
