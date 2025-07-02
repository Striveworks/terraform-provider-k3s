package k3s

import (
	"encoding/base64"
	"reflect"
	"testing"
)

func TestWriteFileCommands(t *testing.T) {
	commands := WriteFileCommands("/tmp/file", base64.StdEncoding.EncodeToString([]byte("hello world")))
	expected := []string{
		"echo \"aGVsbG8gd29ybGQ=\" | sudo tee /tmp/file.tmp > /dev/null",
		"sudo base64 -d /tmp/file.tmp | sudo tee /tmp/file > /dev/null",
		"sudo chown root:root /tmp/file",
		"sudo rm /tmp/file.tmp",
	}

	if !reflect.DeepEqual(commands, expected) {
		t.Error(commands, "Is not equal to", expected)
	}
}
