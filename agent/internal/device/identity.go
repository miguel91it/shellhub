package device

import (
	"errors"
	"io/ioutil"
	"math"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/shellhub-io/shellhub/pkg/models"
)

var ErrNoInterfaceFound = errors.New("No interface found")

func NewDeviceIdentity() (*models.DeviceIdentity, error) {
	d := &models.DeviceIdentity{}

	iface, err := primaryIface()
	if err != nil {
		return nil, err
	}

	d.MAC = iface.HardwareAddr.String()

	return d, nil
}

func primaryIface() (*net.Interface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, ErrNoInterfaceFound
	}

	var ifdev *net.Interface
	min := uint64(math.MaxUint16)

	for i, iface := range interfaces {
		if iface.Flags&net.FlagLoopback > 0 {
			continue
		}

		data, err := readSysFs(iface.Name, "type")
		if err != nil {
			break
		}

		iftype, err := strconv.ParseUint(data, 10, 16)
		if err != nil {
			break
		}

		if iftype != syscall.ARPHRD_ETHER {
			break
		}

		data, err = readSysFs(iface.Name, "ifindex")
		if err != nil {
			break
		}

		ifindex, err := strconv.ParseUint(data, 10, 16)
		if err != nil {
			break
		}

		if ifindex < min {
			min = ifindex
			ifdev = &interfaces[i]
		}
	}

	if ifdev == nil {
		return nil, ErrNoInterfaceFound
	}

	return ifdev, nil
}

func readSysFs(iface, file string) (string, error) {
	data, err := ioutil.ReadFile(filepath.Join("/sys/class/net", iface, file))
	return strings.TrimSpace(string(data)), err
}
