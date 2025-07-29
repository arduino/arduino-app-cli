package results

import "fmt"

type VersionResult struct {
	AppName string `json:"appName"`
	Version string `json:"version"`
}

func (r VersionResult) String() string {
	return fmt.Sprintf("%s v%s", r.AppName, r.Version)
}

func (r VersionResult) Data() interface{} {
	return r
}
