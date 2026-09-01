package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/getlantern/systray"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx context.Context

	mu       sync.Mutex
	configs  []ProjectConfig
	settings AppSettings
	active   map[string]*runningProcess
	tray     sync.Once
}

type ProjectConfig struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Path        string       `json:"path"`
	Runtime     string       `json:"runtime"`
	Command     string       `json:"command"`
	NodeVersion string       `json:"nodeVersion"`
	Solution    string       `json:"solution"`
	ProjectFile string       `json:"projectFile"`
	AppSettings string       `json:"appSettings"`
	Docker      DockerConfig `json:"docker"`
}

type CommandPreset struct {
	Label   string `json:"label"`
	Command string `json:"command"`
	Runtime string `json:"runtime"`
}

type ProcessStatus struct {
	Running         bool   `json:"running"`
	Message         string `json:"message"`
	ProjectID       string `json:"projectId"`
	ProjectName     string `json:"projectName"`
	Port            int    `json:"port"`
	URL             string `json:"url"`
	DocsActive      bool   `json:"docsActive"`
	DocsURL         string `json:"docsUrl"`
	DetectedRuntime string `json:"detectedRuntime"`
	StartedAt       string `json:"startedAt"`
	Output          string `json:"output"`
}

type NodeInfo struct {
	CurrentVersion    string              `json:"currentVersion"`
	InstalledVersions []string            `json:"installedVersions"`
	AvailableVersions []NodeVersionOption `json:"availableVersions"`
	ManagedDirectory  string              `json:"managedDirectory"`
	Message           string              `json:"message"`
}

type NodeVersionOption struct {
	Version   string `json:"version"`
	Channel   string `json:"channel"`
	Installed bool   `json:"installed"`
}

type AppSettingEntry struct {
	Section string `json:"section"`
	Key     string `json:"key"`
	Value   string `json:"value"`
}

type AppSettings struct {
	ActiveNodeVersion  string              `json:"activeNodeVersion"`
	CachedNodeVersions []NodeVersionOption `json:"cachedNodeVersions"`
}

type runningProcess struct {
	cmd       *exec.Cmd
	project   ProjectConfig
	startedAt time.Time
	output    []string
	port      int
	docsURL   string
	done      chan error
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.active = map[string]*runningProcess{}
	migrateLegacyState()
	_ = a.loadConfigs()
	_ = a.loadSettings()
	a.startTray()
	go a.hideWhenMinimized()
}

func (a *App) beforeClose(ctx context.Context) bool {
	wailsruntime.WindowHide(ctx)
	return true
}

func (a *App) ShowWindow() {
	wailsruntime.WindowShow(a.ctx)
	wailsruntime.WindowUnminimise(a.ctx)
}

func (a *App) HideToTray() {
	wailsruntime.WindowHide(a.ctx)
}

func (a *App) OpenSettings() {
	wailsruntime.WindowShow(a.ctx)
	wailsruntime.WindowUnminimise(a.ctx)
	wailsruntime.EventsEmit(a.ctx, "navigate:settings")
}

func (a *App) QuitApplication() {
	_ = a.StopAll()
	systray.Quit()
	wailsruntime.Quit(a.ctx)
}

func (a *App) startTray() {
	a.tray.Do(func() {
		go systray.Run(func() {
			systray.SetIcon(appIcon)
			systray.SetTitle("Exectron")
			systray.SetTooltip("Exectron - runner local")

			open := systray.AddMenuItem("Abrir", "Abrir Exectron")
			hide := systray.AddMenuItem("Segundo plano", "Ocultar janela")
			settings := systray.AddMenuItem("Configuracoes", "Abrir configuracoes")
			systray.AddSeparator()
			quit := systray.AddMenuItem("Sair", "Encerrar aplicacao")

			for {
				select {
				case <-open.ClickedCh:
					a.ShowWindow()
				case <-hide.ClickedCh:
					a.HideToTray()
				case <-settings.ClickedCh:
					a.OpenSettings()
				case <-quit.ClickedCh:
					a.QuitApplication()
					return
				}
			}
		}, func() {})
	})
}

func (a *App) hideWhenMinimized() {
	ticker := time.NewTicker(700 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		if a.ctx == nil {
			continue
		}
		if wailsruntime.WindowIsMinimised(a.ctx) {
			wailsruntime.WindowHide(a.ctx)
		}
	}
}

func (a *App) emitProgress(scope string, message string) {
	if a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, "app:progress", map[string]string{
		"scope":   scope,
		"message": message,
		"time":    time.Now().Format("15:04:05"),
	})
}

// jsonSlice devolve um slice nao-nil. Um slice nil vira `null` no JSON, e o
// frontend chama `.length` direto no retorno dos metodos: numa base recem
// criada, sem nenhum perfil salvo, isso derrubava a tela inicial inteira.
func jsonSlice[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

func (a *App) GetCommandPresets() []CommandPreset {
	return []CommandPreset{
		{Label: "Node - npm run dev", Command: "npm run dev", Runtime: "node"},
		{Label: "Node - npm start", Command: "npm start", Runtime: "node"},
		{Label: "Node - pnpm dev", Command: "pnpm dev", Runtime: "node"},
		{Label: "Node - yarn dev", Command: "yarn dev", Runtime: "node"},
		{Label: ".NET - dotnet watch", Command: "dotnet watch run", Runtime: "dotnet"},
		{Label: ".NET - dotnet run", Command: "dotnet run", Runtime: "dotnet"},
		{Label: ".NET - dotnet run sem restore", Command: "dotnet run --no-restore", Runtime: "dotnet"},
		{Label: "Go - go run", Command: "go run .", Runtime: "go"},
		{Label: "Python - uvicorn", Command: "uvicorn main:app --reload", Runtime: "python"},
		{Label: "Python - flask", Command: "flask run", Runtime: "python"},
	}
}

func (a *App) ChooseDirectory() (string, error) {
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Selecionar pasta do projeto",
	})
}

func (a *App) ChooseDotnetProject(defaultPath string) (string, error) {
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            "Selecionar arquivo .csproj",
		DefaultDirectory: validDefaultDirectory(defaultPath),
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Projetos .NET (*.csproj)", Pattern: "*.csproj"},
		},
	})
}

func (a *App) ChooseAppSettingsFile(defaultPath string) (string, error) {
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            "Selecionar appsettings.json",
		DefaultDirectory: validDefaultDirectory(defaultPath),
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "AppSettings (*.json)", Pattern: "*.json"},
		},
	})
}

func (a *App) ChooseComposeFile(defaultPath string) (string, error) {
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            "Selecionar arquivo compose",
		DefaultDirectory: validDefaultDirectory(defaultPath),
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Compose (*.yml, *.yaml)", Pattern: "*.yml;*.yaml"},
		},
	})
}

func (a *App) ChooseDockerfile(defaultPath string) (string, error) {
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            "Selecionar Dockerfile",
		DefaultDirectory: validDefaultDirectory(defaultPath),
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Dockerfile", Pattern: "Dockerfile*"},
		},
	})
}

// PreviewDockerCommand mostra na interface a linha exata que o perfil vai executar.
func (a *App) PreviewDockerCommand(config ProjectConfig) (string, error) {
	return a.buildDockerCommand(config)
}

func validDefaultDirectory(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return path
	}
	if err == nil && !info.IsDir() {
		return filepath.Dir(path)
	}
	return ""
}

func (a *App) FindDotnetSolutions(projectPath string) ([]string, error) {
	return a.FindDotnetProjects(projectPath)
}

func (a *App) FindDotnetProjects(projectPath string) ([]string, error) {
	projectPath = strings.TrimSpace(projectPath)
	if projectPath == "" {
		return nil, errors.New("informe o caminho do projeto")
	}
	root, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("caminho invalido: %w", err)
	}

	type candidate struct {
		path  string
		score int
	}
	var candidates []candidate
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			name := strings.ToLower(entry.Name())
			if name == "bin" || name == "obj" || name == "node_modules" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		lower := strings.ToLower(entry.Name())
		if strings.HasSuffix(lower, ".csproj") {
			candidates = append(candidates, candidate{path: path, score: dotnetProjectScore(path)})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
	projects := make([]string, 0, len(candidates))
	for _, item := range candidates {
		projects = append(projects, item.path)
	}
	return projects, nil
}

func (a *App) FindAppSettings(projectPath string) ([]string, error) {
	projectPath = strings.TrimSpace(projectPath)
	if projectPath == "" {
		return nil, errors.New("informe o caminho do projeto")
	}
	root, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, err
	}
	var files []string
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			name := strings.ToLower(entry.Name())
			if name == "bin" || name == "obj" || name == "node_modules" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		name := strings.ToLower(entry.Name())
		if strings.HasPrefix(name, "appsettings") && strings.HasSuffix(name, ".json") {
			files = append(files, path)
		}
		return nil
	})
	return jsonSlice(files), err
}

func (a *App) LoadAppSettingsEntries(path string) ([]AppSettingEntry, error) {
	data, settings, err := readJSONFile(path)
	if err != nil {
		return nil, err
	}
	_ = data
	var entries []AppSettingEntry
	flattenSettings("", settings, &entries)
	return jsonSlice(entries), nil
}

func (a *App) SaveAppSettingsEntry(path string, entry AppSettingEntry) ([]AppSettingEntry, error) {
	_, settings, err := readJSONFile(path)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(entry.Key) == "" {
		return nil, errors.New("informe a chave")
	}
	setSettingValue(settings, entry.Section, entry.Key, entry.Value)
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, err
	}
	return a.LoadAppSettingsEntries(path)
}

func (a *App) ListConfigs() ([]ProjectConfig, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return jsonSlice(append([]ProjectConfig(nil), a.configs...)), nil
}

func (a *App) SaveConfig(config ProjectConfig) ([]ProjectConfig, error) {
	if config.Runtime == "" {
		config.Runtime = detectRuntime(config.Command)
	}
	// Perfis docker montam o comando a partir do modo escolhido e podem
	// rodar uma imagem pronta sem apontar para uma pasta de projeto.
	if config.Runtime == "docker" {
		if strings.TrimSpace(config.Docker.Mode) == "" {
			config.Docker.Mode = "compose"
		}
		if err := validateDockerConfig(config.Docker); err != nil {
			return nil, err
		}
	} else {
		if strings.TrimSpace(config.Path) == "" {
			return nil, errors.New("informe o caminho do projeto")
		}
		if strings.TrimSpace(config.Command) == "" {
			return nil, errors.New("informe o comando")
		}
	}
	if config.ID == "" {
		config.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if config.Name == "" {
		config.Name = defaultConfigName(config)
	}
	if config.ProjectFile == "" && strings.HasSuffix(strings.ToLower(config.Solution), ".csproj") {
		config.ProjectFile = config.Solution
	}
	if config.Runtime == "dotnet" && config.ProjectFile == "" {
		if projects, err := a.FindDotnetProjects(config.Path); err == nil && len(projects) > 0 {
			config.ProjectFile = projects[0]
		}
	}
	if config.Runtime == "dotnet" && config.AppSettings == "" {
		if files, err := a.FindAppSettings(config.Path); err == nil && len(files) > 0 {
			config.AppSettings = files[0]
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	replaced := false
	for index := range a.configs {
		if a.configs[index].ID == config.ID {
			a.configs[index] = config
			replaced = true
			break
		}
	}
	if !replaced {
		a.configs = append(a.configs, config)
	}
	return jsonSlice(append([]ProjectConfig(nil), a.configs...)), a.saveConfigsLocked()
}

func (a *App) DeleteConfig(id string) ([]ProjectConfig, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	next := a.configs[:0]
	for _, config := range a.configs {
		if config.ID != id {
			next = append(next, config)
		}
	}
	a.configs = next
	return jsonSlice(append([]ProjectConfig(nil), a.configs...)), a.saveConfigsLocked()
}

func (a *App) Start(config ProjectConfig) (ProcessStatus, error) {
	a.emitProgress("process", "validando configuracao do projeto")
	a.mu.Lock()
	if a.active == nil {
		a.active = map[string]*runningProcess{}
	}
	if a.active[config.ID] != nil {
		status := a.statusLocked(config.ID)
		a.mu.Unlock()
		a.emitProgress("process", "este projeto ja esta em execucao")
		return status, errors.New("este projeto ja esta em execucao")
	}
	a.mu.Unlock()

	if config.Runtime == "docker" {
		if err := validateDockerConfig(config.Docker); err != nil {
			a.emitProgress("process", err.Error())
			return ProcessStatus{}, err
		}
	} else if strings.TrimSpace(config.Path) == "" || strings.TrimSpace(config.Command) == "" {
		a.emitProgress("process", "caminho e comando sao obrigatorios")
		return ProcessStatus{}, errors.New("caminho e comando sao obrigatorios")
	}
	if strings.TrimSpace(config.ID) == "" {
		config.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if strings.TrimSpace(config.Name) == "" {
		config.Name = defaultConfigName(config)
	}
	if path := strings.TrimSpace(config.Path); path != "" {
		if _, err := os.Stat(path); err != nil {
			a.emitProgress("process", "caminho invalido")
			return ProcessStatus{}, fmt.Errorf("caminho invalido: %w", err)
		}
	}
	if config.Runtime == "dotnet" {
		if strings.TrimSpace(config.ProjectFile) == "" {
			a.emitProgress("process", "selecione o arquivo .csproj do projeto .NET")
			return ProcessStatus{}, errors.New("selecione o arquivo .csproj do projeto .NET")
		}
		if _, err := os.Stat(config.ProjectFile); err != nil {
			a.emitProgress("process", "arquivo .csproj nao existe")
			return ProcessStatus{}, fmt.Errorf("o arquivo .csproj selecionado nao existe: %s", config.ProjectFile)
		}
	}

	commandLine := config.Command
	a.emitProgress("process", "preparando ambiente de execucao")
	if config.Runtime == "dotnet" {
		commandLine = a.prepareDotnetCommand(config)
		a.emitProgress("process", "comando .NET resolvido: "+commandLine)
		a.emitProgress("process", "diretorio .NET: "+runDirectory(config))
	}
	if config.Runtime == "docker" {
		built, err := a.buildDockerCommand(config)
		if err != nil {
			a.emitProgress("process", err.Error())
			return ProcessStatus{}, err
		}
		commandLine = built
		a.emitProgress("process", "comando docker resolvido: "+commandLine)
		a.prepareDockerRun(config)
	}

	cmd := exec.Command(shellName(), shellFlag(), commandLine)
	cmd.Dir = runDirectory(config)
	cmd.SysProcAttr = processGroupAttr()
	cmd.Env = a.processEnvironment(config)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ProcessStatus{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return ProcessStatus{}, err
	}

	running := &runningProcess{
		cmd:       cmd,
		project:   config,
		startedAt: time.Now(),
		done:      make(chan error, 1),
	}

	if err := cmd.Start(); err != nil {
		a.emitProgress("process", "falha ao iniciar processo: "+err.Error())
		return ProcessStatus{}, err
	}
	a.emitProgress("process", fmt.Sprintf("processo iniciado com PID %d", cmd.Process.Pid))

	a.mu.Lock()
	a.active[config.ID] = running
	// O compose e o docker run raramente imprimem a porta no log, entao a
	// publicacao declarada no perfil ja vira a porta exibida no runner.
	if config.Runtime == "docker" {
		running.port = firstDockerHostPort(config.Docker)
	}
	a.mu.Unlock()

	go a.captureOutput(stdout, running)
	go a.captureOutput(stderr, running)
	go func() {
		running.done <- cmd.Wait()
		a.mu.Lock()
		if a.active[config.ID] == running {
			delete(a.active, config.ID)
		}
		a.mu.Unlock()
	}()

	status := a.waitForStartup(running)
	if !status.Running {
		a.emitProgress("process", status.Message)
		if tail := outputTail(status.Output, 8); tail != "" {
			a.emitProgress("process", "saida final: "+tail)
			return status, errors.New(status.Message + "\n\n" + tail)
		}
		return status, errors.New(status.Message)
	}
	if status.Port > 0 {
		a.emitProgress("process", fmt.Sprintf("porta detectada: %d", status.Port))
	} else {
		a.emitProgress("process", "processo rodando; porta ainda nao detectada")
	}
	return status, nil
}

func (a *App) Stop(projectID string) (ProcessStatus, error) {
	a.mu.Lock()
	active := a.active[projectID]
	if active == nil {
		a.mu.Unlock()
		return ProcessStatus{Running: false, Message: "nenhuma aplicacao em execucao"}, nil
	}
	a.mu.Unlock()

	err := stopProcessTree(active.cmd)

	select {
	case <-active.done:
	case <-time.After(4 * time.Second):
		_ = killProcessTree(active.cmd)
	}

	a.mu.Lock()
	if a.active[projectID] == active {
		delete(a.active, projectID)
	}
	a.mu.Unlock()

	// Matar a CLI do docker nao encerra o container: o down/rm e explicito.
	if active.project.Runtime == "docker" {
		a.cleanupDockerRun(active.project)
	}

	return ProcessStatus{Running: false, Message: "aplicacao parada"}, err
}

func (a *App) StopAll() error {
	a.mu.Lock()
	ids := make([]string, 0, len(a.active))
	for id := range a.active {
		ids = append(ids, id)
	}
	a.mu.Unlock()

	var lastErr error
	for _, id := range ids {
		if _, err := a.Stop(id); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (a *App) GetStatus(projectID string) ProcessStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.statusLocked(projectID)
}

func (a *App) GetStatuses() []ProcessStatus {
	a.mu.Lock()
	defer a.mu.Unlock()

	statuses := make([]ProcessStatus, 0, len(a.active))
	for id := range a.active {
		statuses = append(statuses, a.statusLocked(id))
	}
	return statuses
}

func (a *App) GetNodeInfo() NodeInfo {
	a.mu.Lock()
	activeVersion := a.settings.ActiveNodeVersion
	cachedVersions := append([]NodeVersionOption(nil), a.settings.CachedNodeVersions...)
	a.mu.Unlock()

	installed := listManagedNodeVersions()
	available := markInstalled(cachedVersions, installed)
	if len(available) == 0 {
		available = markInstalled(defaultNodeVersions(), installed)
	}

	info := NodeInfo{
		InstalledVersions: jsonSlice(installed),
		AvailableVersions: jsonSlice(available),
		ManagedDirectory:  nodeVersionsDir(),
	}

	if activeVersion != "" {
		info.CurrentVersion = activeVersion
	} else if output, err := runShort("node", "-v"); err == nil {
		info.CurrentVersion = strings.TrimSpace(output)
	}
	if len(info.InstalledVersions) == 0 {
		info.Message = "Nenhuma versao gerenciada instalada"
	}
	return info
}

func (a *App) RefreshNodeVersionList() (NodeInfo, error) {
	a.emitProgress("node", "buscando lista oficial de versoes do Node")
	versions, err := fetchNodeVersions()
	if err != nil {
		a.emitProgress("node", "falha ao buscar versoes: "+err.Error())
		return a.GetNodeInfo(), err
	}

	a.mu.Lock()
	a.settings.CachedNodeVersions = versions
	err = a.saveSettingsLocked()
	a.mu.Unlock()
	if err != nil {
		a.emitProgress("node", "falha ao salvar lista de versoes: "+err.Error())
		return a.GetNodeInfo(), err
	}
	a.emitProgress("node", fmt.Sprintf("%d versoes Node salvas", len(versions)))
	return a.GetNodeInfo(), nil
}

func (a *App) InstallNodeVersion(version string) (NodeInfo, error) {
	version = normalizeNodeVersion(version)
	if version == "" {
		return a.GetNodeInfo(), errors.New("informe a versao do Node")
	}
	if err := a.installNodeVersion(version); err != nil {
		return a.GetNodeInfo(), err
	}
	return a.GetNodeInfo(), nil
}

func (a *App) UseNodeVersion(version string) (NodeInfo, error) {
	version = normalizeNodeVersion(version)
	if version == "" {
		return a.GetNodeInfo(), errors.New("informe a versao do Node")
	}
	if _, err := os.Stat(nodeExecutable(version)); err != nil {
		return a.GetNodeInfo(), fmt.Errorf("Node %s ainda nao esta instalado. Use baixar/instalar antes de trocar.", version)
	}

	a.mu.Lock()
	a.settings.ActiveNodeVersion = version
	err := a.saveSettingsLocked()
	a.mu.Unlock()
	if err != nil {
		return a.GetNodeInfo(), err
	}
	if err := a.activateNodeVersion(version); err != nil {
		return a.GetNodeInfo(), err
	}

	return a.GetNodeInfo(), nil
}

func (a *App) DeleteNodeVersion(version string) (NodeInfo, error) {
	version = normalizeNodeVersion(version)
	if version == "" {
		return a.GetNodeInfo(), errors.New("informe a versao do Node")
	}

	installed := listManagedNodeVersions()
	if len(installed) <= 1 {
		return a.GetNodeInfo(), errors.New("mantenha ao menos uma versao Node instalada")
	}

	a.mu.Lock()
	active := normalizeNodeVersion(a.settings.ActiveNodeVersion)
	if version == active {
		next, ok := nearestNodeVersion(version, installed)
		if !ok {
			a.mu.Unlock()
			return a.GetNodeInfo(), errors.New("nenhuma outra versao Node instalada para assumir")
		}
		a.settings.ActiveNodeVersion = next
		if err := a.saveSettingsLocked(); err != nil {
			a.mu.Unlock()
			return a.GetNodeInfo(), err
		}
		a.emitProgress("node", "versao ativa alterada para "+next+" antes da exclusao")
	}
	a.mu.Unlock()

	target := managedNodeRoot(version)
	if _, err := os.Stat(target); err != nil {
		return a.GetNodeInfo(), fmt.Errorf("versao Node %s nao instalada", version)
	}
	a.emitProgress("node", "excluindo Node "+version)
	if err := os.RemoveAll(target); err != nil {
		a.emitProgress("node", "falha ao excluir Node "+version+": "+err.Error())
		return a.GetNodeInfo(), err
	}
	a.emitProgress("node", "Node "+version+" excluido")
	return a.GetNodeInfo(), nil
}

func (a *App) loadConfigs() error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		a.configs = defaultConfigs()
		return a.saveConfigsLocked()
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &a.configs)
}

func (a *App) saveConfigsLocked() error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(a.configs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (a *App) loadSettings() error {
	path, err := settingsFilePath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &a.settings)
}

func (a *App) saveSettingsLocked() error {
	path, err := settingsFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(a.settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// defaultConfigs devolve os perfis de uma instalacao nova. A lista comeca
// vazia de proposito: cada maquina monta os proprios perfis.
func defaultConfigs() []ProjectConfig {
	return []ProjectConfig{}
}

const (
	appDirName    = "exectron"
	legacyDirName = "starter-project"
)

func appConfigDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.TempDir(), appDirName)
	}
	return filepath.Join(base, appDirName)
}

func legacyConfigDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, legacyDirName)
}

func configFilePath() (string, error) {
	return filepath.Join(appConfigDir(), "projects.json"), nil
}

func settingsFilePath() (string, error) {
	return filepath.Join(appConfigDir(), "settings.json"), nil
}

// migrateLegacyState traz perfis e preferencias da pasta antiga na primeira
// execucao depois do rename, para ninguem perder o que ja tinha configurado.
func migrateLegacyState() {
	legacy := legacyConfigDir()
	if legacy == "" {
		return
	}
	current := appConfigDir()
	if _, err := os.Stat(filepath.Join(current, "projects.json")); err == nil {
		return
	}
	for _, name := range []string{"projects.json", "settings.json"} {
		data, err := os.ReadFile(filepath.Join(legacy, name))
		if err != nil {
			continue
		}
		if err := os.MkdirAll(current, 0755); err != nil {
			return
		}
		_ = os.WriteFile(filepath.Join(current, name), data, 0644)
	}
}

func (a *App) captureOutput(reader io.Reader, process *runningProcess) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		a.mu.Lock()
		if len(process.output) >= 300 {
			process.output = process.output[1:]
		}
		process.output = append(process.output, line)
		if process.port == 0 {
			process.port = extractPort(line)
		}
		a.mu.Unlock()
	}
}

func (a *App) waitForStartup(process *runningProcess) ProcessStatus {
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-process.done:
			message := "processo finalizou durante a inicializacao"
			if err != nil {
				message += ": " + err.Error()
			}
			a.mu.Lock()
			if a.active[process.project.ID] == process {
				delete(a.active, process.project.ID)
			}
			output := strings.Join(process.output, "\n")
			a.mu.Unlock()
			return ProcessStatus{
				Running:     false,
				Message:     message,
				ProjectID:   process.project.ID,
				ProjectName: process.project.Name,
				Output:      output,
			}
		default:
		}

		a.mu.Lock()
		if a.active[process.project.ID] != process {
			status := a.statusLocked(process.project.ID)
			a.mu.Unlock()
			return status
		}
		port := process.port
		a.mu.Unlock()

		if port > 0 && portResponds(port) {
			a.detectDocs(process, port)
			break
		}
		time.Sleep(400 * time.Millisecond)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	return a.statusLocked(process.project.ID)
}

func (a *App) detectDocs(process *runningProcess, port int) {
	for _, path := range []string{"/swagger", "/swagger/index.html", "/api-docs", "/docs", "/openapi.json"} {
		url := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)
		if httpOK(url) {
			a.mu.Lock()
			process.docsURL = url
			a.mu.Unlock()
			return
		}
	}
}

func (a *App) statusLocked(projectID string) ProcessStatus {
	active := a.active[projectID]
	if active == nil {
		return ProcessStatus{Running: false, Message: "parado"}
	}
	port := active.port
	url := ""
	if port > 0 {
		url = fmt.Sprintf("http://127.0.0.1:%d", port)
	}
	message := "processo iniciado"
	if port == 0 {
		message = "processo iniciado; porta ainda nao detectada"
	}
	return ProcessStatus{
		Running:         true,
		Message:         message,
		ProjectID:       active.project.ID,
		ProjectName:     active.project.Name,
		Port:            port,
		URL:             url,
		DocsActive:      active.docsURL != "",
		DocsURL:         active.docsURL,
		DetectedRuntime: detectRuntime(active.project.Command),
		StartedAt:       active.startedAt.Format(time.RFC3339),
		Output:          strings.Join(active.output, "\n"),
	}
}

func extractPort(line string) int {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(localhost|127\.0\.0\.1|0\.0\.0\.0):(\d{2,5})`),
		regexp.MustCompile(`(?i)(port|porta)\s*[:=]?\s*(\d{2,5})`),
		regexp.MustCompile(`:(\d{4,5})\b`),
	}
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(line)
		if len(match) == 3 {
			return atoiPort(match[2])
		}
		if len(match) == 2 {
			return atoiPort(match[1])
		}
	}
	return 0
}

func atoiPort(value string) int {
	var port int
	_, _ = fmt.Sscanf(value, "%d", &port)
	if port > 0 && port <= 65535 {
		return port
	}
	return 0
}

func portResponds(port int) bool {
	return httpOK(fmt.Sprintf("http://127.0.0.1:%d", port))
}

func httpOK(url string) bool {
	client := http.Client{Timeout: 900 * time.Millisecond}
	response, err := client.Get(url)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode < 500
}

func parseNVMVersions(output string) []string {
	versionPattern := regexp.MustCompile(`v?\d+\.\d+\.\d+`)
	seen := map[string]bool{}
	var versions []string
	for _, match := range versionPattern.FindAllString(output, -1) {
		version := strings.TrimPrefix(match, "v")
		if !seen[version] {
			seen[version] = true
			versions = append(versions, version)
		}
	}
	return versions
}

func (a *App) processEnvironment(config ProjectConfig) []string {
	env := os.Environ()
	if config.Runtime != "node" && detectRuntime(config.Command) != "node" {
		return env
	}

	version := normalizeNodeVersion(config.NodeVersion)
	if version == "" {
		a.mu.Lock()
		version = a.settings.ActiveNodeVersion
		a.mu.Unlock()
	}
	if version == "" {
		return env
	}

	nodeBin := managedNodeBin(version)
	if _, err := os.Stat(nodeExecutable(version)); err != nil {
		return env
	}
	return prependPath(env, nodeBin)
}

func runDirectory(config ProjectConfig) string {
	if config.Runtime == "dotnet" && strings.TrimSpace(config.ProjectFile) != "" {
		return filepath.Dir(config.ProjectFile)
	}
	if config.Runtime == "docker" {
		return dockerWorkingDirectory(config)
	}
	return config.Path
}

func defaultConfigName(config ProjectConfig) string {
	if path := strings.TrimSpace(config.Path); path != "" {
		return filepath.Base(path)
	}
	if config.Runtime == "docker" {
		if image := strings.TrimSpace(config.Docker.Image); image != "" {
			return image
		}
		if compose := strings.TrimSpace(config.Docker.ComposeFile); compose != "" {
			return filepath.Base(filepath.Dir(compose))
		}
	}
	return "novo perfil"
}

func prependPath(env []string, value string) []string {
	next := append([]string{}, env...)
	for index, item := range next {
		if strings.HasPrefix(strings.ToUpper(item), "PATH=") {
			next[index] = item[:5] + value + string(os.PathListSeparator) + item[5:]
			return next
		}
	}
	return append(next, "PATH="+value)
}

func normalizeNodeVersion(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	return version
}

// managedNodeRoot e a pasta extraida da versao. managedNodeBin, definida por
// plataforma, aponta para o diretorio que precisa entrar no PATH: no Windows e
// a propria raiz, no Linux e a subpasta bin.
func managedNodeRoot(version string) string {
	return filepath.Join(nodeVersionsDir(), normalizeNodeVersion(version))
}

func nodeExecutable(version string) string {
	return filepath.Join(managedNodeBin(version), nodeExecutableName())
}

// nodeVersionsDir mantem instalacoes antigas onde estao: o PATH do sistema ja
// aponta para elas, e mover a pasta quebraria o node ate a proxima troca.
func nodeVersionsDir() string {
	current := filepath.Join(appConfigDir(), "node")
	if _, err := os.Stat(current); err == nil {
		return current
	}
	if legacy := legacyConfigDir(); legacy != "" {
		if _, err := os.Stat(filepath.Join(legacy, "node")); err == nil {
			return filepath.Join(legacy, "node")
		}
	}
	return current
}

func replaceMarkedBlock(content string, start string, end string, block string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	startIndex := strings.Index(content, start)
	endIndex := -1
	if startIndex >= 0 {
		endSearchStart := startIndex + len(start)
		if relativeEnd := strings.Index(content[endSearchStart:], end); relativeEnd >= 0 {
			endIndex = endSearchStart + relativeEnd + len(end)
		}
	}
	if startIndex >= 0 && endIndex >= startIndex {
		prefix := strings.TrimRight(content[:startIndex], "\n")
		suffix := strings.TrimLeft(content[endIndex:], "\n")
		parts := []string{}
		if prefix != "" {
			parts = append(parts, prefix)
		}
		parts = append(parts, strings.ReplaceAll(block, "\r\n", "\n"))
		if suffix != "" {
			parts = append(parts, suffix)
		}
		return strings.Join(parts, "\n\n") + "\n"
	}
	if strings.TrimSpace(content) == "" {
		return strings.ReplaceAll(block, "\r\n", "\n") + "\n"
	}
	return strings.TrimRight(content, "\n") + "\n\n" + strings.ReplaceAll(block, "\r\n", "\n") + "\n"
}

func listManagedNodeVersions() []string {
	entries, err := os.ReadDir(nodeVersionsDir())
	if err != nil {
		return nil
	}
	var versions []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		version := normalizeNodeVersion(entry.Name())
		if _, err := os.Stat(nodeExecutable(version)); err == nil {
			versions = append(versions, version)
		}
	}
	return versions
}

type nodeRelease struct {
	Version string `json:"version"`
	LTS     any    `json:"lts"`
}

func fetchNodeVersions() ([]NodeVersionOption, error) {
	client := http.Client{Timeout: 8 * time.Second}
	response, err := client.Get("https://nodejs.org/dist/index.json")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("indice Node retornou %s", response.Status)
	}

	var releases []nodeRelease
	if err := json.NewDecoder(response.Body).Decode(&releases); err != nil {
		return nil, err
	}

	seenMajor := map[string]bool{}
	var versions []NodeVersionOption
	for _, release := range releases {
		version := normalizeNodeVersion(release.Version)
		if version == "" {
			continue
		}
		major := strings.Split(version, ".")[0]
		if release.LTS != false || len(versions) < 8 {
			if !seenMajor[major] || len(versions) < 12 {
				channel := "Stable"
				if len(versions) == 0 {
					channel = "Latest"
				}
				if release.LTS != false {
					channel = "LTS"
					if name, ok := release.LTS.(string); ok && name != "" {
						channel = "LTS " + name
					}
				}
				versions = append(versions, NodeVersionOption{Version: version, Channel: channel})
				seenMajor[major] = true
			}
		}
		if len(versions) >= 24 {
			break
		}
	}
	return versions, nil
}

func defaultNodeVersions() []NodeVersionOption {
	return []NodeVersionOption{
		{Version: "22.18.0", Channel: "LTS"},
		{Version: "20.19.4", Channel: "LTS"},
		{Version: "18.20.8", Channel: "LTS"},
	}
}

func markInstalled(available []NodeVersionOption, installed []string) []NodeVersionOption {
	installedSet := map[string]bool{}
	for _, version := range installed {
		installedSet[normalizeNodeVersion(version)] = true
	}
	seen := map[string]bool{}
	var result []NodeVersionOption
	for _, item := range available {
		item.Version = normalizeNodeVersion(item.Version)
		if item.Version == "" || seen[item.Version] {
			continue
		}
		item.Installed = installedSet[item.Version]
		if item.Channel == "" {
			item.Channel = "Stable"
		}
		result = append(result, item)
		seen[item.Version] = true
	}
	for _, version := range installed {
		version = normalizeNodeVersion(version)
		if !seen[version] {
			result = append(result, NodeVersionOption{Version: version, Channel: "Installed", Installed: true})
		}
	}
	return result
}

func nearestNodeVersion(current string, installed []string) (string, bool) {
	current = normalizeNodeVersion(current)
	var candidates []string
	for _, version := range installed {
		version = normalizeNodeVersion(version)
		if version != "" && version != current {
			candidates = append(candidates, version)
		}
	}
	if len(candidates) == 0 {
		return "", false
	}
	sortNodeVersions(candidates)
	currentMajor, _, _ := versionParts(current)
	best := candidates[0]
	bestDistance := 1000000
	for _, candidate := range candidates {
		major, _, _ := versionParts(candidate)
		distance := major - currentMajor
		if distance < 0 {
			distance = -distance
		}
		if distance < bestDistance {
			best = candidate
			bestDistance = distance
		}
	}
	return best, true
}

func sortNodeVersions(versions []string) {
	for i := 0; i < len(versions); i++ {
		for j := i + 1; j < len(versions); j++ {
			if compareVersions(versions[j], versions[i]) > 0 {
				versions[i], versions[j] = versions[j], versions[i]
			}
		}
	}
}

func compareVersions(left, right string) int {
	l1, l2, l3 := versionParts(left)
	r1, r2, r3 := versionParts(right)
	if l1 != r1 {
		return l1 - r1
	}
	if l2 != r2 {
		return l2 - r2
	}
	return l3 - r3
}

func versionParts(version string) (int, int, int) {
	var major, minor, patch int
	_, _ = fmt.Sscanf(normalizeNodeVersion(version), "%d.%d.%d", &major, &minor, &patch)
	return major, minor, patch
}

func readJSONFile(path string) ([]byte, map[string]any, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil, errors.New("informe o caminho do appsettings")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	data = trimUTF8BOM(data)
	data = stripJSONComments(data)
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, nil, err
	}
	return data, settings, nil
}

func stripJSONComments(data []byte) []byte {
	var output []byte
	inString := false
	escaped := false
	for i := 0; i < len(data); i++ {
		current := data[i]
		if inString {
			output = append(output, current)
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
				continue
			}
			if current == '"' {
				inString = false
			}
			continue
		}
		if current == '"' {
			inString = true
			output = append(output, current)
			continue
		}
		if current == '/' && i+1 < len(data) {
			next := data[i+1]
			if next == '/' {
				i += 2
				for i < len(data) && data[i] != '\n' && data[i] != '\r' {
					i++
				}
				if i < len(data) {
					output = append(output, data[i])
				}
				continue
			}
			if next == '*' {
				i += 2
				for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
					i++
				}
				i++
				continue
			}
		}
		output = append(output, current)
	}
	return output
}

func trimUTF8BOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		return data[3:]
	}
	return data
}

func flattenSettings(section string, value any, entries *[]AppSettingEntry) {
	object, ok := value.(map[string]any)
	if !ok {
		return
	}
	for key, child := range object {
		childSection := section
		if nested, ok := child.(map[string]any); ok {
			if childSection == "" {
				childSection = key
			} else {
				childSection += ":" + key
			}
			flattenSettings(childSection, nested, entries)
			continue
		}
		*entries = append(*entries, AppSettingEntry{
			Section: section,
			Key:     key,
			Value:   fmt.Sprint(child),
		})
	}
}

func setSettingValue(settings map[string]any, section string, key string, value string) {
	current := settings
	for _, part := range strings.Split(section, ":") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	current[key] = coerceSettingValue(value)
}

func coerceSettingValue(value string) any {
	trimmed := strings.TrimSpace(value)
	if strings.EqualFold(trimmed, "true") {
		return true
	}
	if strings.EqualFold(trimmed, "false") {
		return false
	}
	if strings.EqualFold(trimmed, "null") {
		return nil
	}
	var number float64
	if _, err := fmt.Sscanf(trimmed, "%f", &number); err == nil && regexp.MustCompile(`^-?\d+(\.\d+)?$`).MatchString(trimmed) {
		return number
	}
	return value
}

func (a *App) prepareDotnetCommand(config ProjectConfig) string {
	command := strings.TrimSpace(config.Command)
	lower := strings.ToLower(command)
	isRun := lower == "dotnet run" || strings.HasPrefix(lower, "dotnet run ")
	isWatchRun := lower == "dotnet watch run" || strings.HasPrefix(lower, "dotnet watch run ")
	if !isRun && !isWatchRun {
		return command
	}
	project := strings.TrimSpace(config.ProjectFile)
	if project == "" && strings.HasSuffix(strings.ToLower(config.Solution), ".csproj") {
		project = config.Solution
	}
	if project == "" {
		var err error
		project, err = findDotnetProject(config.Path)
		if err != nil {
			a.emitProgress("process", "nao encontrei .csproj; usando comando informado")
			return command
		}
	}
	if _, err := os.Stat(project); err != nil {
		a.emitProgress("process", "arquivo .csproj nao encontrado: "+project)
		return command
	}
	if isWatchRun {
		args := strings.TrimSpace(strings.TrimPrefix(command, "dotnet watch run"))
		if args != "" {
			return "dotnet watch --project " + shellQuote(project) + " run " + args
		}
		return "dotnet watch --project " + shellQuote(project) + " run"
	}

	args := strings.TrimSpace(strings.TrimPrefix(command, "dotnet run"))
	if args != "" {
		return "dotnet run --project " + shellQuote(project) + " " + args
	}
	return "dotnet run --project " + shellQuote(project)
}

func findDotnetProject(root string) (string, error) {
	type candidate struct {
		path  string
		score int
	}
	var candidates []candidate
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			name := strings.ToLower(entry.Name())
			if name == "bin" || name == "obj" || name == "node_modules" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".csproj") {
			candidates = append(candidates, candidate{path: path, score: dotnetProjectScore(path)})
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", errors.New("nenhum .csproj encontrado")
	}
	best := candidates[0]
	for _, item := range candidates[1:] {
		if item.score > best.score {
			best = item
		}
	}
	return best.path, nil
}

func dotnetProjectScore(path string) int {
	lowerPath := strings.ToLower(path)
	name := strings.ToLower(filepath.Base(path))
	score := 0
	if strings.Contains(name, "api") || strings.Contains(name, "web") {
		score += 40
	}
	if strings.Contains(lowerPath, "src") {
		score += 15
	}
	if strings.Contains(name, "test") || strings.Contains(name, "tests") || strings.Contains(lowerPath, ".test") || strings.Contains(lowerPath, "\\test") {
		score -= 100
	}
	if data, err := os.ReadFile(path); err == nil {
		text := strings.ToLower(string(data))
		if strings.Contains(text, "microsoft.net.sdk.web") {
			score += 80
		}
		if strings.Contains(text, "microsoft.aspnetcore") {
			score += 25
		}
		if strings.Contains(text, "microsoft.net.test.sdk") {
			score -= 120
		}
	}
	return score
}

func outputTail(output string, maxLines int) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return ""
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (a *App) activateNodeVersion(version string) error {
	nodeBin := managedNodeBin(version)
	if _, err := os.Stat(nodeExecutable(version)); err != nil {
		return fmt.Errorf("Node %s nao esta instalado em %s", version, nodeBin)
	}
	a.emitProgress("node", "ativando Node "+version+" no PATH da aplicacao")
	if err := a.activateNodeVersionPlatform(version, nodeBin); err != nil {
		return err
	}
	_ = os.Setenv("PATH", nodeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return nil
}

func (a *App) installNodeVersion(version string) error {
	version = normalizeNodeVersion(version)
	if version == "" {
		return errors.New("versao invalida")
	}
	target := managedNodeRoot(version)
	if _, err := os.Stat(nodeExecutable(version)); err == nil {
		a.emitProgress("node", "Node "+version+" ja esta instalado")
		return nil
	}
	if err := os.MkdirAll(nodeVersionsDir(), 0755); err != nil {
		return err
	}

	archiveURL := nodeArchiveURL(version)
	a.emitProgress("node", "baixando Node "+version)
	response, err := http.Get(archiveURL)
	if err != nil {
		a.emitProgress("node", "falha no download: "+err.Error())
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		a.emitProgress("node", "download falhou: "+response.Status)
		return fmt.Errorf("download do Node falhou: %s", response.Status)
	}

	archivePath := filepath.Join(nodeVersionsDir(), "node-"+url.PathEscape(version)+nodeArchiveExtension())
	a.emitProgress("node", "salvando pacote em disco")
	file, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, response.Body); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	defer os.Remove(archivePath)

	tempDir := filepath.Join(nodeVersionsDir(), "extract-"+version)
	_ = os.RemoveAll(tempDir)
	a.emitProgress("node", "extraindo pacote")
	if err := extractNodeArchive(archivePath, tempDir); err != nil {
		a.emitProgress("node", "falha ao extrair: "+err.Error())
		return err
	}
	defer os.RemoveAll(tempDir)

	extractedRoot := filepath.Join(tempDir, nodeArchiveRootName(version))
	if _, err := os.Stat(extractedRoot); err != nil {
		return fmt.Errorf("pacote Node inesperado: %w", err)
	}
	_ = os.RemoveAll(target)
	if err := os.Rename(extractedRoot, target); err != nil {
		a.emitProgress("node", "falha ao finalizar instalacao: "+err.Error())
		return err
	}
	a.emitProgress("node", "Node "+version+" instalado")
	return nil
}

func detectRuntime(command string) string {
	lower := strings.ToLower(command)
	switch {
	case strings.Contains(lower, "docker"):
		return "docker"
	case strings.Contains(lower, "npm") || strings.Contains(lower, "pnpm") || strings.Contains(lower, "yarn") || strings.Contains(lower, "node"):
		return "node"
	case strings.Contains(lower, "dotnet"):
		return "dotnet"
	case strings.Contains(lower, "go "):
		return "go"
	case strings.Contains(lower, "python") || strings.Contains(lower, "uvicorn") || strings.Contains(lower, "flask"):
		return "python"
	default:
		return "custom"
	}
}

func runShort(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return string(output), err
}

func runShellShort(command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	output, err := exec.CommandContext(ctx, shellName(), shellFlag(), command).CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
