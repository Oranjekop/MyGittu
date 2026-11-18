package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	native "github.com/egoist/mygo/ui"
	"mygittu/internal/commitai"
	"mygittu/internal/ipc"
)

type aiSettingsForm struct {
	config        commitai.Config
	key, savedKey string
}

func (f *aiSettingsForm) apply(config commitai.Config) {
	if f.config != config {
		f.key = f.savedKey
	}
	f.config = config
}

func (d *Desktop) copy(zh, en string) string {
	if d.prefs.Language == "en" {
		return en
	}
	return zh
}

func (d *Desktop) aiSettings(c *native.Context) {
	f := &d.aiForm
	providers := commitai.Providers()
	names := []string{d.copy("未启用", "Disabled")}
	for _, provider := range providers {
		names = append(names, provider.Name)
	}
	previous := f.config.Provider
	selected := previous
	if selected == "" {
		selected = names[0]
	}
	native.Text(c, d.copy("供应商", "Provider")).Bold()
	native.Select(c, &selected, names).FillWidth().Disabled(d.busy)
	if selected != names[0] && selected != previous {
		f.config = commitai.Config{Provider: selected}
		f.key = ""
		for _, provider := range providers {
			if provider.Name == selected {
				f.config.BaseURL = provider.BaseURL
				break
			}
		}
	} else if selected == names[0] && previous != "" {
		f.config = commitai.Config{}
		f.key = ""
	}
	if f.config.Provider != "" {
		previousURL := f.config.BaseURL
		for _, field := range []struct {
			label string
			value *string
		}{
			{d.copy("API 基础地址", "API base URL"), &f.config.BaseURL},
			{d.copy("模型 ID", "Model ID"), &f.config.Model},
		} {
			native.Text(c, field.label)
			native.TextInput(c, field.value).Label(field.label).FillWidth().Disabled(d.busy)
		}
		if previousURL != f.config.BaseURL {
			f.key = ""
		}
		native.Text(c, d.copy("API 密钥（本次运行）", "API key (this session)"))
		input := native.TextInput(c, &f.key).Password().FillWidth().Disabled(d.busy)
		if env := f.environment(); env != "" {
			input.Placeholder(env)
		}
	}
	_, configErr := f.config.Endpoint()
	changed := f.config != d.prefs.AI || strings.TrimSpace(f.key) != f.savedKey
	if IconButton(c, d.copy("保存 AI 设置", "Save AI settings"), "save", true).FillWidth().Disabled(d.busy || !changed || (f.config.Provider != "" && configErr != nil)).Clicked() {
		d.saveAISettings()
	}
}

func (f aiSettingsForm) environment() string {
	for _, p := range commitai.Providers() {
		if f.config.Provider == p.Name && strings.TrimRight(strings.TrimSpace(f.config.BaseURL), "/") == p.BaseURL {
			return p.Environment
		}
	}
	if f.config.Provider == "OpenAI-compatible" {
		return "MYGITTU_AI_API_KEY"
	}
	return ""
}

func (d *Desktop) saveAISettings() {
	prefs := d.prefs
	prefs.AI = d.aiForm.config
	key := strings.TrimSpace(d.aiForm.key)
	d.task(func(ctx context.Context) (func(), error) {
		e := d.Client.Call(ctx, "settings.update", ipc.Params{Settings: prefs}, &prefs)
		return func() {
			d.aiForm.savedKey = key
			d.aiForm.key = key
			d.applyPrefs(prefs)
		}, e
	})
}

func (d *Desktop) requestCommitAI() {
	d.aiError = ""
	if d.prefs.AI.Provider == "" {
		d.openSettings()
		d.settingsTab = 2
		return
	}
	d.aiConfirm = true
}

func (d *Desktop) commitAIDialog(c *native.Context) {
	if !d.aiConfirm {
		return
	}
	native.Modal(c, &d.aiConfirm, func() {
		native.Column(c).Width(400).Gap(panelSpacing).Children(func() {
			native.Text(c, d.copy("生成提交信息", "Generate commit message")).Bold()
			native.Text(c, fmt.Sprintf(d.copy("将暂存区差异发送至 %s。", "Send the staged diff to %s."), d.prefs.AI.BaseURL)).FillWidth()
			if strings.TrimSpace(d.message) != "" {
				native.Text(c, d.copy("生成成功后会替换当前草稿。", "The generated message will replace the current draft.")).FillWidth()
			}
			native.Row(c).Gap(controlSpacing).Children(func() {
				if IconButton(c, d.text("close"), "close", false).Clicked() {
					d.aiConfirm = false
				}
				if IconButton(c, d.copy("发送并生成", "Send and generate"), "sparkle", true).Disabled(d.busy).Clicked() {
					d.aiConfirm = false
					d.generateCommitMessage()
				}
			})
		})
	})
}

func (d *Desktop) generateCommitMessage() {
	if d.busy || d.selected == "" || d.stagedCount == 0 {
		return
	}
	id, config, lang, key := d.selected, d.prefs.AI, d.prefs.Language, d.aiForm.savedKey
	if key == "" {
		form := aiSettingsForm{config: config}
		if env := form.environment(); env != "" {
			key = strings.TrimSpace(os.Getenv(env))
		}
	}
	prompt := d.prefs.AIPrompt
	d.aiGenerating = true
	d.task(func(ctx context.Context) (func(), error) {
		ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		var staged string
		err := d.Client.Call(ctx, "git.diff.staged-for-message", ipc.Params{RepoID: id}, &staged)
		var result string
		if err == nil {
			result, err = commitai.Generate(ctx, config, key, lang, staged, prompt)
		}
		return func() {
			d.aiGenerating = false
			if d.selected != id {
				return
			}
			if err != nil {
				d.aiError = fmt.Sprintf(d.copy("生成失败：%s", "Generation failed: %s"), err)
				return
			}
			d.message = result
			d.saveDraft()
		}, nil
	})
}

func (d *Desktop) aiActionLabel() string {
	if d.aiGenerating {
		return d.copy("生成中…", "Generating…")
	}
	return "AI"
}

func (d *Desktop) promptSettings(c *native.Context) {
	label := d.copy("提交信息提示词", "Commit message prompt")
	native.Text(c, label).Bold()
	defaultPrompt := commitai.DefaultPrompt(d.prefs.Language)
	native.TextArea(c, &d.aiPromptDraft).Label(label).Placeholder(defaultPrompt).FillWidth().Height(150).Disabled(d.busy)
	prompt := strings.TrimSpace(d.aiPromptDraft)
	if prompt == defaultPrompt {
		prompt = ""
	}
	count := utf8.RuneCountInString(prompt)
	if count > commitai.MaxPrompt {
		native.Textf(c, d.copy("提示词最多 %d 字", "Prompt limit is %d characters"), commitai.MaxPrompt).TextColor(c.Theme().Danger)
	}
	native.Row(c).Gap(controlSpacing).Wrap().Children(func() {
		if IconButton(c, d.copy("保存提示词", "Save prompt"), "save", true).Disabled(d.busy || count > commitai.MaxPrompt || prompt == d.prefs.AIPrompt).Clicked() {
			d.saveCommitPrompt(prompt)
		}
		if IconButton(c, d.copy("恢复默认", "Reset"), "undo", false).Disabled(d.busy || (d.aiPromptDraft == defaultPrompt && d.prefs.AIPrompt == "")).Clicked() {
			d.aiPromptDraft = defaultPrompt
			if d.prefs.AIPrompt != "" {
				d.saveCommitPrompt("")
			}
		}
	})
}

func (d *Desktop) saveCommitPrompt(prompt string) {
	prefs := d.prefs
	prefs.AIPrompt = prompt
	d.savePrefs(prefs)
}
