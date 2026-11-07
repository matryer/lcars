// Package buildinfo identifies the binary separately from its authoring contract.
package buildinfo

import (
	_ "embed"
	"runtime"
	"runtime/debug"
	"strings"
)

//go:embed VERSION
var version string

// Set by release builds. Ordinary go builds use Go's embedded VCS information.
var commit, builtAt string

const ContractVersion = "1.0"
const ProtocolVersion = 1

type Info struct {
	Version         string `json:"version"`
	ContractVersion string `json:"contractVersion"`
	ProtocolVersion int    `json:"protocolVersion"`
	Commit          string `json:"commit"`
	BuiltAt         string `json:"builtAt"`
	CommitTime      string `json:"commitTime"`
	Modified        bool   `json:"modified"`
	GoVersion       string `json:"goVersion"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
}

func Current() Info {
	info := Info{Version: strings.TrimSpace(version), ContractVersion: ContractVersion, ProtocolVersion: ProtocolVersion, Commit: commit, BuiltAt: builtAt, GoVersion: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH}
	if build, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range build.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info.Commit == "" {
					info.Commit = setting.Value
				}
			case "vcs.time":
				info.CommitTime = setting.Value
			case "vcs.modified":
				info.Modified = setting.Value == "true"
			}
		}
	}
	return info
}
