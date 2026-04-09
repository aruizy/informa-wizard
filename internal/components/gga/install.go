package gga

import (
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/installcmd"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/model"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/system"
)

func InstallCommand(profile system.PlatformProfile) ([][]string, error) {
	return installcmd.NewResolver().ResolveComponentInstall(profile, model.ComponentGGA)
}

func ShouldInstall(enabled bool) bool {
	return enabled
}
