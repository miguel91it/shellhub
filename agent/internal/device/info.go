package device

import (
	"github.com/shellhub-io/shellhub/agent/internal/osrelease"
	"github.com/shellhub-io/shellhub/pkg/models"
)

func NewDeviceInfo() (*models.DeviceInfo, error) {
	attr := &models.DeviceInfo{}

	id, err := osrelease.GetValue("ID")
	if err != nil {
		return nil, err
	}

	name, err := osrelease.GetValue("PRETTY_NAME")
	if err != nil {
		return nil, err
	}

	attr.ID = id
	attr.PrettyName = name

	return attr, nil
}
