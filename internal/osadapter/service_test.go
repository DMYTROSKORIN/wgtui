package osadapter

import (
	"reflect"
	"testing"
)

func TestOpenRCEnableCommands(t *testing.T) {
	got := openRCEnableCommands("wg0")
	want := []string{
		"sh -c 'test -e /etc/init.d/wg-quick.wg0 || ln -s wg-quick /etc/init.d/wg-quick.wg0'",
		"rc-update add wg-quick.wg0 default",
		"rc-service wg-quick.wg0 start",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("openRCEnableCommands(wg0) =\n%v\nwant\n%v", got, want)
	}
}

func TestServiceManagerConstants(t *testing.T) {
	if ServiceManagerSystemd == ServiceManagerOpenRC {
		t.Fatal("ServiceManagerSystemd and ServiceManagerOpenRC must be distinct")
	}
}
