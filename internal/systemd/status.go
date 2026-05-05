package systemd

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/godbus/dbus/v5"
)

const (
	systemdDestination = "org.freedesktop.systemd1"
	managerPath        = dbus.ObjectPath("/org/freedesktop/systemd1")
	managerInterface   = "org.freedesktop.systemd1.Manager"
	unitInterface      = "org.freedesktop.systemd1.Unit"
	serviceInterface   = "org.freedesktop.systemd1.Service"
	propertiesGet      = "org.freedesktop.DBus.Properties.Get"
)

type Status struct {
	Name        string
	LoadState   string
	ActiveState string
	SubState    string
	Description string
	MainPID     int64
	Raw         string
}

type Conn interface {
	Object(dest string, path dbus.ObjectPath) dbus.BusObject
}

type Client struct {
	conn Conn
}

func NewClient(conn Conn) *Client {
	return &Client{conn: conn}
}

func ConnectSystemBus() (*dbus.Conn, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, fmt.Errorf("connect system bus: %w", err)
	}
	return conn, nil
}

func (c *Client) Status(ctx context.Context, service string, includeRaw bool) (Status, error) {
	if c == nil || c.conn == nil {
		return Status{}, errors.New("systemd client has no D-Bus connection")
	}
	if service == "" {
		return Status{}, errors.New("service name is empty")
	}

	manager := c.conn.Object(systemdDestination, managerPath)
	var unitPath dbus.ObjectPath
	if err := manager.CallWithContext(ctx, managerInterface+".LoadUnit", 0, service).Store(&unitPath); err != nil {
		return Status{Name: service}, fmt.Errorf("load unit %q: %w", service, err)
	}

	unit := c.conn.Object(systemdDestination, unitPath)
	status := Status{Name: service}

	var err error
	if status.LoadState, err = stringProperty(ctx, unit, unitInterface, "LoadState"); err != nil {
		return status, fmt.Errorf("get %s LoadState: %w", service, err)
	}
	if status.ActiveState, err = stringProperty(ctx, unit, unitInterface, "ActiveState"); err != nil {
		return status, fmt.Errorf("get %s ActiveState: %w", service, err)
	}
	if status.SubState, err = stringProperty(ctx, unit, unitInterface, "SubState"); err != nil {
		return status, fmt.Errorf("get %s SubState: %w", service, err)
	}
	if status.Description, err = stringProperty(ctx, unit, unitInterface, "Description"); err != nil {
		return status, fmt.Errorf("get %s Description: %w", service, err)
	}

	mainPID, err := uint32Property(ctx, unit, serviceInterface, "MainPID")
	if err == nil {
		status.MainPID = int64(mainPID)
	}

	if includeRaw {
		raw, err := rawStatus(ctx, service)
		if err != nil && raw == "" {
			return status, err
		}
		status.Raw = raw
	}

	return status, nil
}

func stringProperty(ctx context.Context, obj dbus.BusObject, iface, name string) (string, error) {
	var v dbus.Variant
	if err := obj.CallWithContext(ctx, propertiesGet, 0, iface, name).Store(&v); err != nil {
		return "", err
	}
	value, ok := v.Value().(string)
	if !ok {
		return "", fmt.Errorf("%s.%s is %T, want string", iface, name, v.Value())
	}
	return value, nil
}

func uint32Property(ctx context.Context, obj dbus.BusObject, iface, name string) (uint32, error) {
	var v dbus.Variant
	if err := obj.CallWithContext(ctx, propertiesGet, 0, iface, name).Store(&v); err != nil {
		return 0, err
	}
	value, ok := v.Value().(uint32)
	if !ok {
		return 0, fmt.Errorf("%s.%s is %T, want uint32", iface, name, v.Value())
	}
	return value, nil
}

func rawStatus(ctx context.Context, service string) (string, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "status", "--no-pager", "--full", service)
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		return "", fmt.Errorf("systemctl status %q: %w", service, err)
	}
	return string(out), nil
}
