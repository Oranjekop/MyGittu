package ui

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	native "github.com/egoist/mygo/ui"
	"github.com/go-keyring/keyring"
	"mygittu/internal/commitai"
	"mygittu/internal/ipc"
)

type aiSettingsForm struct {
	config     commitai.Config
	key        string
	models     []string
	modelError string
}

func (f *aiSettingsForm) apply(config commitai.Config) {
	if f.config.Provider != config.Provider || f.config.BaseURL != config.BaseURL {
		f.models, f.modelError = nil, ""
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
		f.models, f.modelError = nil, ""
		for _, provider := range providers {
			if provider.Name == selected {
				f.config.BaseURL = provider.BaseURL
				break
			}
		}
	} else if selected == names[0] && previous != "" {
		f.config = commitai.Config{}
		f.key = ""
		f.models, f.modelError = nil, ""
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
			f.models, f.modelError = nil, ""
		}
		native.Text(c, d.copy("API 密钥", "API key"))
		input := native.TextInput(c, &f.key).Password().FillWidth().Disabled(d.busy)
		input.Placeholder(d.copy("留空沿用已保存的密钥", "Leave blank to keep the saved key"))
		_, addressError := f.config.Base()
		if IconButton(c, d.copy("获取模型 ID", "Fetch model IDs"), "download", false).Disabled(d.busy || addressError != nil).Clicked() {
			d.fetchAIModels()
		}
		if len(f.models) > 0 {
			label := d.copy("选择模型", "Choose a model")
			selected := f.config.Model
			if !slices.Contains(f.models, selected) {
				selected = ""
			}
			native.Select(c, &selected, f.models).Label(label).Placeholder(label).FillWidth().Disabled(d.busy)
			if selected != "" {
				f.config.Model = selected
			}
		}
		if f.modelError != "" {
			native.Text(c, f.modelError).FillWidth().TextColor(c.Theme().Danger)
		}
	}
	_, configErr := f.config.Endpoint()
	changed := f.config != d.prefs.AI || strings.TrimSpace(f.key) != ""
	if IconButton(c, d.copy("保存 AI 设置", "Save AI settings"), "save", true).FillWidth().Disabled(d.busy || !changed || (f.config.Provider != "" && configErr != nil)).Clicked() {
		d.saveAISettings()
	}
	if f.config.Provider != "" && configErr == nil &&
		IconButton(c, d.copy("移除保存的密钥", "Remove saved key"), "remove", false).Disabled(d.busy).Clicked() {
		config := f.config
		removeError := d.copy("无法移除系统凭据中的 AI 密钥", "Could not remove the AI key from the system credential store")
		d.task(func(context.Context) (func(), error) {
			err := keyring.Delete(aiKeyService, aiKeyAccount(config))
			if err != nil && !errors.Is(err, keyring.ErrNotFound) {
				return nil, errors.New(removeError)
			}
			return func() { f.key = "" }, nil
		})
	}
}

const aiKeyService = "MyGittu AI API"

func aiKeyAccount(config commitai.Config) string {
	base, _ := config.Base()
	return fmt.Sprintf("%x", sha256.Sum256([]byte(config.Provider+"\x00"+base)))
}

func aiRequestKey(config commitai.Config, entered string) (string, error) {
	if entered != "" {
		return entered, nil
	}
	secret, err := keyring.Get(aiKeyService, aiKeyAccount(config))
	if err == nil {
		return string(secret), nil
	}
	form := aiSettingsForm{config: config}
	if key := strings.TrimSpace(os.Getenv(form.environment())); key != "" {
		return key, nil
	}
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return "", errors.New("could not read the AI key from the system credential store")
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
	storeError := d.copy("无法将 AI 密钥保存到系统凭据存储", "Could not save the AI key to the system credential store")
	d.task(func(ctx context.Context) (func(), error) {
		if key != "" {
			if err := keyring.Set(aiKeyService, aiKeyAccount(prefs.AI), []byte(key)); err != nil {
				return nil, errors.New(storeError)
			}
		}
		e := d.Client.Call(ctx, "settings.update", ipc.Params{Settings: prefs}, &prefs)
		return func() {
			d.aiForm.key = ""
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
	d.generateCommitMessage()
}

func (d *Desktop) generateCommitMessage() {
	if d.busy || d.selected == "" || d.stagedCount == 0 {
		return
	}
	id, config := d.selected, d.prefs.AI
	prompt := d.prefs.AIPrompt
	d.aiGenerating = true
	d.task(func(ctx context.Context) (func(), error) {
		ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		var staged string
		err := d.Client.Call(ctx, "git.diff.staged-for-message", ipc.Params{RepoID: id}, &staged)
		var result string
		if err == nil {
			var key string
			key, err = aiRequestKey(config, "")
			if err == nil {
				result, err = commitai.Generate(ctx, config, key, staged, prompt)
			}
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

func (d *Desktop) promptSettings(c *native.Context) {
	label := d.copy("提交信息提示词", "Commit message prompt")
	native.Text(c, label).Bold()
	defaultPrompt := commitai.DefaultPrompt()
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

func (d *Desktop) fetchAIModels() {
	f := &d.aiForm
	config, key := f.config, strings.TrimSpace(f.key)
	f.models, f.modelError = nil, ""
	d.task(func(ctx context.Context) (func(), error) {
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		credential, err := aiRequestKey(config, key)
		var models []string
		if err == nil {
			models, err = commitai.ListModels(ctx, config, credential)
		}
		return func() {
			if f.config.Provider != config.Provider || f.config.BaseURL != config.BaseURL {
				return
			}
			if err != nil {
				f.modelError = fmt.Sprintf(d.copy("无法获取模型列表：%s。可手动填写模型 ID。", "Could not list models: %s. Enter the model ID manually."), err)
				return
			}
			f.models = models
		}, nil
	})
}
