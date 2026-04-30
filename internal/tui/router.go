package tui

type Route struct {
	Forward  Screen
	Backward Screen
}

var linearRoutes = map[Screen]Route{
	ScreenWelcome:                {Forward: ScreenDetection},
	ScreenDetection:              {Forward: ScreenAgents, Backward: ScreenWelcome},
	ScreenAgents:                 {Forward: ScreenPreset, Backward: ScreenDetection},
	ScreenPreset:                 {Forward: ScreenDependencyTree, Backward: ScreenAgents},
	ScreenClaudeModelPicker:      {Forward: ScreenDependencyTree, Backward: ScreenPreset},
	ScreenSDDMode:                {Forward: ScreenStrictTDD, Backward: ScreenPreset},
	ScreenStrictTDD:              {Forward: ScreenDependencyTree, Backward: ScreenSDDMode},
	ScreenModelPicker:            {Forward: ScreenStrictTDD, Backward: ScreenSDDMode},
	ScreenDependencyTree:         {Forward: ScreenReview, Backward: ScreenPreset},
	ScreenSkillPicker:            {Forward: ScreenDevSkillPicker, Backward: ScreenDependencyTree},
	ScreenDevSkillPicker:         {Forward: ScreenReview, Backward: ScreenSkillPicker},
	ScreenMonday:                 {Forward: ScreenWelcome, Backward: ScreenWelcome},
	ScreenReview:                 {Forward: ScreenInstalling, Backward: ScreenDependencyTree},
	ScreenInstalling:             {Forward: ScreenComplete, Backward: ScreenReview},
	ScreenComplete:               {Backward: ScreenInstalling},
	ScreenBackups:                {Backward: ScreenWelcome},
	ScreenInstallationView:       {Backward: ScreenWelcome},
	ScreenRestoreConfirm:         {Backward: ScreenBackups},
	ScreenRestoreResult:          {Backward: ScreenBackups},
	ScreenDeleteConfirm:          {Backward: ScreenBackups},
	ScreenDeleteResult:           {Backward: ScreenBackups},
	ScreenRenameBackup:           {Backward: ScreenBackups},
	ScreenUpgrade:                {Backward: ScreenWelcome},
	ScreenSync:                   {Backward: ScreenWelcome},
	ScreenUpgradeSync:            {Backward: ScreenWelcome},
	ScreenModelConfig:            {Backward: ScreenWelcome},
	ScreenProfiles:               {Backward: ScreenWelcome},
	ScreenProfileCreate:          {Backward: ScreenProfiles},
	ScreenProfileDelete:          {Backward: ScreenProfiles},
	ScreenAgentBuilderEngine:     {Backward: ScreenWelcome},
	ScreenAgentBuilderPrompt:     {Backward: ScreenAgentBuilderEngine},
	ScreenAgentBuilderSDD:        {Backward: ScreenAgentBuilderPrompt},
	ScreenAgentBuilderSDDPhase:   {Backward: ScreenAgentBuilderSDD},
	ScreenAgentBuilderGenerating: {Backward: ScreenAgentBuilderPrompt},
	ScreenAgentBuilderPreview:    {Backward: ScreenAgentBuilderPrompt},
	ScreenAgentBuilderInstalling: {Forward: ScreenAgentBuilderComplete},
	ScreenAgentBuilderComplete:   {Backward: ScreenWelcome},
}

func NextScreen(screen Screen) (Screen, bool) {
	route, ok := linearRoutes[screen]
	if !ok || route.Forward == ScreenUnknown {
		return ScreenUnknown, false
	}

	return route.Forward, true
}

func PreviousScreen(screen Screen) (Screen, bool) {
	route, ok := linearRoutes[screen]
	if !ok || route.Backward == ScreenUnknown {
		return ScreenUnknown, false
	}

	return route.Backward, true
}
