package k3s_test

import (
	"testing"

	"gopkg.in/yaml.v3"
	"striveworks.us/terraform-provider-k3s/internal/k3s"
)

func TestNewK3sServerComponent(t *testing.T) {
	t.Parallel()

	server, err := k3s.NewK3sServerComponent(
		t.Context(),
		`node-label: [test=node]
write-kubeconfig-mode: "0700"`,
		`mirrors:
  "registry.k8s.io":
    "endpoint": ["1234"]`,
		"",
		"",
		make(map[string]string),
	)
	if err != nil {
		t.Fatalf("Expected nil err but found: %v", err.Error())
	}
	{
		expected, _ := yaml.Marshal(map[any]any{
			"node-label":            []interface{}{"test=node"},
			"write-kubeconfig-mode": "0700",
		})

		result, _ := yaml.Marshal(server.Config())

		if string(result) != string(expected) {
			t.Fatalf("Expected %s err but found: %v", result, expected)
		}
	}

	expected, _ := yaml.Marshal(map[any]any{
		"mirrors": map[any]any{
			"registry.k8s.io": map[any]any{
				"endpoint": []interface{}{"1234"},
			},
		}})

	result, _ := yaml.Marshal(server.Registry())

	if string(result) != string(expected) {
		t.Fatalf("Expected %s err but found: %v", string(result), string(expected))
	}

}
