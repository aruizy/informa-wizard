package catalog

import "gitlab.informa.tools/ai/wizard/informa-wizard/internal/model"

type Component struct {
	ID          model.ComponentID
	Name        string
	Description string
}

var mvpComponents = []Component{
	{ID: model.ComponentEngram, Name: "Engram", Description: "Persistent cross-session memory"},
	{ID: model.ComponentSDD, Name: "SDD", Description: "Spec-driven development workflow"},
	{ID: model.ComponentSkills, Name: "Skills", Description: "Curated coding skill library"},
	{ID: model.ComponentContext7, Name: "Context7", Description: "Latest framework and library docs"},
	{ID: model.ComponentPermission, Name: "Permissions", Description: "Security-first defaults and guardrails"},
	{ID: model.ComponentTheme, Name: "Theme", Description: "Gentleman Kanagawa theme overlay (future)"},
	{ID: model.ComponentMonday, Name: "Monday", Description: "Monday.com MCP integration for task management"},
	{ID: model.ComponentDevSkills, Name: "Dev Skills", Description: "External dev skills from a shared repository"},
	{ID: model.ComponentDevAgents, Name: "Dev Agents", Description: "External dev agent orchestrators from a shared repository"},
}

func MVPComponents() []Component {
	components := make([]Component, len(mvpComponents))
	copy(components, mvpComponents)
	return components
}
