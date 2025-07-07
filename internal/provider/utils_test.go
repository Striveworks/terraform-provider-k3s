package provider_test

import (
	"maps"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	provider "striveworks.us/terraform-provider-k3s/internal/provider"
)

func TestParseK3sRegister(t *testing.T) {

	t.Run("Null Registry", func(t *testing.T) {
		value := basetypes.NewStringNull()
		res, err := provider.ParseYamlString(value)
		if err != nil {
			t.Errorf("Exepected no error on null registry")
		}
		empty := make(map[any]any)
		if !maps.Equal(res, empty) {
			t.Errorf("Exepected %s to be equal to %s", res, empty)
		}
	})
	t.Run("Malformed Registry", func(t *testing.T) {
		value := basetypes.NewStringValue("hello\n\tworld")
		_, err := provider.ParseYamlString(value)
		if err == nil {
			t.Errorf("Exepected error on malformed registry")
		}

	})

	t.Run("Proper Registry", func(t *testing.T) {
		value := basetypes.NewStringValue("hello: world")
		res, err := provider.ParseYamlString(value)
		if err != nil {
			t.Errorf("Exepected not error on Proper registry")
		}
		if res == nil {
			t.Errorf("Exepected Proper registry")
		}
	})
}
