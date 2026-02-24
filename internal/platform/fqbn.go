package platform

import "github.com/arduino/arduino-cli/pkg/fqbn"

type FQBN struct {
	inner *fqbn.FQBN
}

func (f FQBN) PlatformName() string {
	return f.inner.Vendor + ":" + f.inner.Architecture
}

func (f FQBN) String() string {
	return f.inner.String()
}
