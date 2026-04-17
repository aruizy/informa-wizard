package screens

import (
	"strings"

	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/model"
	"gitlab.informa.tools/ai/wizard/informa-wizard/internal/tui/styles"
)

func PresetOptions() []model.PresetID {
	return []model.PresetID{
		model.PresetFull,
		model.PresetEcosystemOnly,
		model.PresetCustom,
	}
}

var presetDescriptions = map[model.PresetID]string{
	model.PresetFull:          "Everything: SDD, skills, permissions & dev tools",
	model.PresetEcosystemOnly: "Core tools: SDD, skills & dev tools",
	model.PresetCustom:        "Pick individual components yourself",
}

func RenderPreset(selected model.PresetID, cursor int) string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Select Ecosystem Preset"))
	b.WriteString("\n\n")

	for idx, preset := range PresetOptions() {
		isSelected := preset == selected
		focused := idx == cursor
		b.WriteString(renderRadio(string(preset), isSelected, focused))
		b.WriteString(styles.SubtextStyle.Render("    "+presetDescriptions[preset]) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(renderOptions([]string{"Back"}, cursor-len(PresetOptions())))
	b.WriteString("\n")
	b.WriteString(styles.HelpStyle.Render("j/k: navigate • enter: select • esc: back"))

	return b.String()
}
