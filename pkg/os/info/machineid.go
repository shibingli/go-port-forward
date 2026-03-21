package info

import (
	"crypto/hmac"
	"crypto/md5"
	"fmt"
	"os"

	"go-port-forward/pkg/machineid"
)

// MachineID 根据应用ID生成唯一的机器标识 | Generate unique machine identifier based on application ID
func MachineID(appID string) (id string, err error) {
	var mid string
	mid, err = machineid.ID()
	if err != nil {
		return
	}

	var hostname string
	hostname, err = os.Hostname()
	if err != nil {
		return
	}

	mac := hmac.New(md5.New, []byte(mid))
	mac.Write([]byte(appID + ":" + hostname))

	id = fmt.Sprintf("%x", mac.Sum(nil))
	return
}
