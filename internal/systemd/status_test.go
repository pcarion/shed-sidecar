package systemd

import (
	"context"
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
)

type fakeConn struct {
	objects map[dbus.ObjectPath]*fakeObject
}

func (f fakeConn) Object(_ string, path dbus.ObjectPath) dbus.BusObject {
	return f.objects[path]
}

type fakeObject struct {
	calls map[string]*dbus.Call
}

func (f *fakeObject) Call(method string, flags dbus.Flags, args ...interface{}) *dbus.Call {
	return f.call(method, args...)
}

func (f *fakeObject) CallWithContext(_ context.Context, method string, flags dbus.Flags, args ...interface{}) *dbus.Call {
	return f.call(method, args...)
}

func (f *fakeObject) Go(method string, flags dbus.Flags, ch chan *dbus.Call, args ...interface{}) *dbus.Call {
	call := f.call(method, args...)
	if ch != nil {
		ch <- call
	}
	return call
}

func (f *fakeObject) GoWithContext(_ context.Context, method string, flags dbus.Flags, ch chan *dbus.Call, args ...interface{}) *dbus.Call {
	call := f.call(method, args...)
	if ch != nil {
		ch <- call
	}
	return call
}

func (f *fakeObject) AddMatchSignal(iface, member string, options ...dbus.MatchOption) *dbus.Call {
	return &dbus.Call{}
}

func (f *fakeObject) RemoveMatchSignal(iface, member string, options ...dbus.MatchOption) *dbus.Call {
	return &dbus.Call{}
}

func (f *fakeObject) GetProperty(p string) (dbus.Variant, error) {
	return dbus.Variant{}, errors.New("GetProperty is not implemented by fakeObject")
}

func (f *fakeObject) StoreProperty(p string, value interface{}) error {
	return errors.New("StoreProperty is not implemented by fakeObject")
}

func (f *fakeObject) SetProperty(p string, v interface{}) error {
	return errors.New("SetProperty is not implemented by fakeObject")
}

func (f *fakeObject) Destination() string { return systemdDestination }
func (f *fakeObject) Path() dbus.ObjectPath {
	return dbus.ObjectPath("/org/freedesktop/systemd1/unit/test_2eservice")
}

func (f *fakeObject) call(method string, args ...interface{}) *dbus.Call {
	key := method
	if method == propertiesGet && len(args) == 2 {
		key = method + "." + args[0].(string) + "." + args[1].(string)
	}
	if call, ok := f.calls[key]; ok {
		return call
	}
	return &dbus.Call{Err: errors.New("unexpected call: " + key)}
}

func TestStatusReadsUnitProperties(t *testing.T) {
	unitPath := dbus.ObjectPath("/org/freedesktop/systemd1/unit/test_2eservice")
	client := NewClient(fakeConn{objects: map[dbus.ObjectPath]*fakeObject{
		managerPath: {
			calls: map[string]*dbus.Call{
				managerInterface + ".LoadUnit": {Body: []interface{}{unitPath}},
			},
		},
		unitPath: {
			calls: map[string]*dbus.Call{
				propertiesGet + "." + unitInterface + ".LoadState":   variantCall("loaded"),
				propertiesGet + "." + unitInterface + ".ActiveState": variantCall("active"),
				propertiesGet + "." + unitInterface + ".SubState":    variantCall("running"),
				propertiesGet + "." + unitInterface + ".Description": variantCall("Test Service"),
				propertiesGet + "." + serviceInterface + ".MainPID":  variantCall(uint32(1234)),
			},
		},
	}})

	status, err := client.Status(context.Background(), "test.service", false)
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status.Name != "test.service" || status.LoadState != "loaded" || status.ActiveState != "active" || status.SubState != "running" || status.Description != "Test Service" || status.MainPID != 1234 {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.Raw != "" {
		t.Fatalf("raw status was populated without includeRaw: %q", status.Raw)
	}
}

func TestStatusReturnsPerUnitError(t *testing.T) {
	client := NewClient(fakeConn{objects: map[dbus.ObjectPath]*fakeObject{
		managerPath: {
			calls: map[string]*dbus.Call{
				managerInterface + ".LoadUnit": {Err: errors.New("no such unit")},
			},
		},
	}})

	status, err := client.Status(context.Background(), "missing.service", false)
	if err == nil {
		t.Fatal("Status returned nil error")
	}
	if status.Name != "missing.service" {
		t.Fatalf("unexpected status name: %q", status.Name)
	}
}

func variantCall(value interface{}) *dbus.Call {
	return &dbus.Call{Body: []interface{}{dbus.MakeVariant(value)}}
}
