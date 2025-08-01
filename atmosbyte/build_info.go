package main

import "runtime/debug"

// BuildInfo contém informações de versão da aplicação
type BuildInfo struct {
	Version   string
	Commit    string
	Date      string
	GoVersion string
	Module    string
}

// GetBuildInfo extrai informações de build usando debug.BuildInfo
func GetBuildInfo() BuildInfo {
	info := BuildInfo{
		Version:   "dev",
		Commit:    "unknown",
		Date:      "unknown",
		GoVersion: "unknown",
		Module:    "unknown",
	}

	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		info.GoVersion = buildInfo.GoVersion
		info.Module = buildInfo.Main.Path

		// Se a versão do módulo principal estiver disponível
		if buildInfo.Main.Version != "(devel)" && buildInfo.Main.Version != "" {
			info.Version = buildInfo.Main.Version
		}

		// Extrai informações de VCS se disponíveis
		for _, setting := range buildInfo.Settings {
			switch setting.Key {
			case "vcs.revision":
				if len(setting.Value) >= 7 {
					info.Commit = setting.Value[:7] // Short commit hash
				} else {
					info.Commit = setting.Value
				}
			case "vcs.time":
				info.Date = setting.Value
			}
		}
	}

	return info
}
