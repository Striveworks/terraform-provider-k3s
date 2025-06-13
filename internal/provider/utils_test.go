package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func TestParseK3sRegister(t *testing.T) {

	t.Run("Null Registry", func(t *testing.T) {
		value := basetypes.NewStringNull()
		res, err := ParseK3sRegistry(&value)
		if err != nil {
			t.Errorf("Exepected no error on null registry")
		}
		if res != nil {
			t.Errorf("Exepected null registry")
		}
	})
	t.Run("Malformed Registry", func(t *testing.T) {
		value := basetypes.NewStringValue("hello\n\tworld")
		res, err := ParseK3sRegistry(&value)
		if err == nil {
			t.Errorf("Exepected error on malformed registry")
		}
		if res != nil {
			t.Errorf("Exepected null registry")
		}
	})

	t.Run("Proper Registry", func(t *testing.T) {
		value := basetypes.NewStringValue("hello: world")
		res, err := ParseK3sRegistry(&value)
		if err != nil {
			t.Errorf("Exepected not error on Proper registry")
		}
		if res == nil {
			t.Errorf("Exepected Proper registry")
		}
	})
}
