package main

import (
	"github.com/shellhub-io/shellhub/agent/pkg/sysinfo"
	"github.com/shellhub-io/shellhub/pkg/models"
)

func GetDeviceIdentity() (*models.DeviceIdentity, error) {
	d := &models.DeviceIdentity{}

	iface, err := sysinfo.PrimaryInterface()
	if err != nil {
		return nil, err
	}

	d.MAC = iface.HardwareAddr.String()

	return d, nil
}

func GetDeviceInfo() (*models.DeviceInfo, error) {
	osrelease, err := sysinfo.GetOSRelease()
	if err != nil {
		return nil, err
	}

	return &models.DeviceInfo{ID: osrelease.ID, PrettyName: osrelease.Name}, nil
}
