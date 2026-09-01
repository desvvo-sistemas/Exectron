package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DockerConfig guarda tudo que um perfil do tipo docker precisa para subir.
// Mode define de onde o container sai: compose, dockerfile ou image.
type DockerConfig struct {
	Mode          string `json:"mode"`
	ComposeFile   string `json:"composeFile"`
	Service       string `json:"service"`
	ProjectName   string `json:"projectName"`
	Dockerfile    string `json:"dockerfile"`
	Context       string `json:"context"`
	Image         string `json:"image"`
	ContainerName string `json:"containerName"`
	Ports         string `json:"ports"`
	EnvVars       string `json:"envVars"`
	Volumes       string `json:"volumes"`
	ExtraArgs     string `json:"extraArgs"`
	Command       string `json:"command"`
	Build         bool   `json:"build"`
	Recreate      bool   `json:"recreate"`
	RemoveOnStop  bool   `json:"removeOnStop"`
}

type DockerInfo struct {
	Available         bool   `json:"available"`
	EngineRunning     bool   `json:"engineRunning"`
	Version           string `json:"version"`
	ComposeAvailable  bool   `json:"composeAvailable"`
	ComposeCommand    string `json:"composeCommand"`
	ComposeVersion    string `json:"composeVersion"`
	Containers        int    `json:"containers"`
	RunningContainers int    `json:"runningContainers"`
	Images            int    `json:"images"`
	Message           string `json:"message"`
}

type DockerContainer struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Image     string `json:"image"`
	State     string `json:"state"`
	Status    string `json:"status"`
	Ports     string `json:"ports"`
	Compose   string `json:"compose"`
	Service   string `json:"service"`
	CreatedAt string `json:"createdAt"`
	Running   bool   `json:"running"`
	URL       string `json:"url"`
}

type DockerImage struct {
	ID           string `json:"id"`
	Repository   string `json:"repository"`
	Tag          string `json:"tag"`
	Reference    string `json:"reference"`
	Size         string `json:"size"`
	CreatedSince string `json:"createdSince"`
	Dangling     bool   `json:"dangling"`
}

type DockerFiles struct {
	ComposeFiles []string `json:"composeFiles"`
	Dockerfiles  []string `json:"dockerfiles"`
}

var (
	composeOnce  sync.Once
	composeCache string
)

// composeCommand descobre uma unica vez se o ambiente usa o plugin
// "docker compose" ou o binario legado "docker-compose".
func composeCommand() string {
	composeOnce.Do(func() {
		composeCache = ""
		if commandExists("docker") {
			if _, err := runDockerCLI(8*time.Second, "compose", "version"); err == nil {
				composeCache = "docker compose"
				return
			}
		}
		if commandExists("docker-compose") {
			composeCache = "docker-compose"
		}
	})
	return composeCache
}

func composeBinary(extra ...string) (string, []string) {
	if composeCommand() == "docker-compose" {
		return "docker-compose", extra
	}
	return "docker", append([]string{"compose"}, extra...)
}

func hiddenCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = hiddenProcessAttr()
	return cmd
}

func runDockerCLI(timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	output, err := hiddenCommand(ctx, "docker", args...).CombinedOutput()
	text := string(output)
	if err != nil {
		return text, errors.New(dockerErrorText(text, err))
	}
	return text, nil
}

func runComposeCLI(timeout time.Duration, args ...string) (string, error) {
	name, full := composeBinary(args...)
	if name == "" {
		return "", errors.New("docker compose nao encontrado neste sistema")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	output, err := hiddenCommand(ctx, name, full...).CombinedOutput()
	text := string(output)
	if err != nil {
		return text, errors.New(dockerErrorText(text, err))
	}
	return text, nil
}

func dockerErrorText(output string, err error) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return err.Error()
}

// GetDockerInfo resume o estado do daemon para o cabecalho da aba Docker.
func (a *App) GetDockerInfo() DockerInfo {
	info := DockerInfo{}
	if !commandExists("docker") {
		info.Message = "Docker CLI nao encontrada. Instale o Docker Desktop e reabra o app."
		return info
	}
	info.Available = true

	if output, err := runDockerCLI(8*time.Second, "version", "--format", "{{.Server.Version}}"); err == nil {
		info.Version = strings.TrimSpace(output)
		info.EngineRunning = info.Version != ""
	} else {
		info.Message = "Docker instalado, mas o daemon nao respondeu: " + strings.TrimSpace(err.Error())
	}
	if info.Version == "" {
		if output, err := runDockerCLI(8*time.Second, "--version"); err == nil {
			info.Version = strings.TrimSpace(output)
		}
	}

	if compose := composeCommand(); compose != "" {
		info.ComposeAvailable = true
		info.ComposeCommand = compose
		if output, err := runComposeCLI(8*time.Second, "version", "--short"); err == nil {
			info.ComposeVersion = strings.TrimSpace(output)
		}
	}

	if !info.EngineRunning {
		return info
	}

	if containers, err := a.ListDockerContainers(true); err == nil {
		info.Containers = len(containers)
		for _, container := range containers {
			if container.Running {
				info.RunningContainers++
			}
		}
	}
	if images, err := a.ListDockerImages(); err == nil {
		info.Images = len(images)
	}
	if info.Message == "" {
		info.Message = "Docker " + info.Version + " pronto"
	}
	return info
}

// ListDockerContainers devolve os containers do daemon. Com all=false apenas os ativos.
func (a *App) ListDockerContainers(all bool) ([]DockerContainer, error) {
	args := []string{"ps", "--no-trunc", "--format", "{{json .}}"}
	if all {
		args = append(args, "-a")
	}
	output, err := runDockerCLI(20*time.Second, args...)
	if err != nil {
		return nil, err
	}

	containers := []DockerContainer{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var raw struct {
			ID        string `json:"ID"`
			Names     string `json:"Names"`
			Image     string `json:"Image"`
			State     string `json:"State"`
			Status    string `json:"Status"`
			Ports     string `json:"Ports"`
			Labels    string `json:"Labels"`
			CreatedAt string `json:"CreatedAt"`
		}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		labels := parseDockerLabels(raw.Labels)
		container := DockerContainer{
			ID:        shortDockerID(raw.ID),
			Name:      strings.Split(raw.Names, ",")[0],
			Image:     raw.Image,
			State:     strings.ToLower(raw.State),
			Status:    raw.Status,
			Ports:     raw.Ports,
			Compose:   labels["com.docker.compose.project"],
			Service:   labels["com.docker.compose.service"],
			CreatedAt: raw.CreatedAt,
		}
		container.Running = container.State == "running" || strings.HasPrefix(strings.ToLower(raw.Status), "up")
		if port := firstPublishedPort(raw.Ports); port > 0 {
			container.URL = fmt.Sprintf("http://127.0.0.1:%d", port)
		}
		containers = append(containers, container)
	}
	return containers, nil
}

func (a *App) ListDockerImages() ([]DockerImage, error) {
	output, err := runDockerCLI(20*time.Second, "images", "--format", "{{json .}}")
	if err != nil {
		return nil, err
	}
	images := []DockerImage{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var raw struct {
			ID           string `json:"ID"`
			Repository   string `json:"Repository"`
			Tag          string `json:"Tag"`
			Size         string `json:"Size"`
			CreatedSince string `json:"CreatedSince"`
		}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		image := DockerImage{
			ID:           shortDockerID(raw.ID),
			Repository:   raw.Repository,
			Tag:          raw.Tag,
			Size:         raw.Size,
			CreatedSince: raw.CreatedSince,
			Dangling:     raw.Repository == "<none>" || raw.Tag == "<none>",
		}
		image.Reference = raw.Repository + ":" + raw.Tag
		if image.Dangling {
			image.Reference = image.ID
		}
		images = append(images, image)
	}
	return images, nil
}

func (a *App) StartDockerContainer(id string) ([]DockerContainer, error) {
	return a.dockerContainerAction("start", "container iniciado", id)
}

func (a *App) StopDockerContainer(id string) ([]DockerContainer, error) {
	return a.dockerContainerAction("stop", "container parado", id)
}

func (a *App) RestartDockerContainer(id string) ([]DockerContainer, error) {
	return a.dockerContainerAction("restart", "container reiniciado", id)
}

func (a *App) RemoveDockerContainer(id string) ([]DockerContainer, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("selecione um container")
	}
	a.emitProgress("docker", "removendo container "+id)
	if _, err := runDockerCLI(60*time.Second, "rm", "-f", id); err != nil {
		a.emitProgress("docker", "falha ao remover container: "+err.Error())
		return nil, err
	}
	a.emitProgress("docker", "container removido")
	return a.ListDockerContainers(true)
}

func (a *App) dockerContainerAction(action string, done string, id string) ([]DockerContainer, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("selecione um container")
	}
	a.emitProgress("docker", action+" "+id)
	if _, err := runDockerCLI(90*time.Second, action, id); err != nil {
		a.emitProgress("docker", "falha no "+action+": "+err.Error())
		return nil, err
	}
	a.emitProgress("docker", done)
	return a.ListDockerContainers(true)
}

// DockerContainerLogs devolve o final do log do container para a aba de saida.
func (a *App) DockerContainerLogs(id string, tail int) (string, error) {
	if strings.TrimSpace(id) == "" {
		return "", errors.New("selecione um container")
	}
	if tail <= 0 {
		tail = 250
	}
	output, err := runDockerCLI(25*time.Second, "logs", "--tail", strconv.Itoa(tail), id)
	if err != nil {
		return output, err
	}
	if strings.TrimSpace(output) == "" {
		return "Container sem saida registrada.", nil
	}
	return output, nil
}

func (a *App) RemoveDockerImage(id string) ([]DockerImage, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("selecione uma imagem")
	}
	a.emitProgress("docker", "removendo imagem "+id)
	if _, err := runDockerCLI(90*time.Second, "rmi", "-f", id); err != nil {
		a.emitProgress("docker", "falha ao remover imagem: "+err.Error())
		return nil, err
	}
	a.emitProgress("docker", "imagem removida")
	return a.ListDockerImages()
}

// PullDockerImage baixa uma imagem transmitindo o progresso para a interface.
func (a *App) PullDockerImage(image string) ([]DockerImage, error) {
	image = strings.TrimSpace(image)
	if image == "" {
		return nil, errors.New("informe a imagem. Ex: postgres:16")
	}
	a.emitProgress("docker", "baixando imagem "+image)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	cmd := hiddenCommand(ctx, "docker", "pull", image)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	a.streamProgress(stdout, "docker")
	if err := cmd.Wait(); err != nil {
		a.emitProgress("docker", "falha ao baixar imagem "+image)
		return nil, fmt.Errorf("falha ao baixar %s: %w", image, err)
	}
	a.emitProgress("docker", "imagem "+image+" disponivel")
	return a.ListDockerImages()
}

func (a *App) streamProgress(reader io.Reader, scope string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			a.emitProgress(scope, line)
		}
	}
}

// FindDockerFiles varre o projeto atras de composes e Dockerfiles.
func (a *App) FindDockerFiles(projectPath string) (DockerFiles, error) {
	found := DockerFiles{ComposeFiles: []string{}, Dockerfiles: []string{}}
	projectPath = strings.TrimSpace(projectPath)
	if projectPath == "" {
		return found, errors.New("informe o caminho do projeto")
	}
	root, err := filepath.Abs(projectPath)
	if err != nil {
		return found, err
	}
	if _, err := os.Stat(root); err != nil {
		return found, fmt.Errorf("caminho invalido: %w", err)
	}

	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			name := strings.ToLower(entry.Name())
			if name == "bin" || name == "obj" || name == "node_modules" || name == ".git" || name == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		name := strings.ToLower(entry.Name())
		switch {
		case strings.HasPrefix(name, "docker-compose") && (strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")):
			found.ComposeFiles = append(found.ComposeFiles, path)
		case name == "compose.yml" || name == "compose.yaml":
			found.ComposeFiles = append(found.ComposeFiles, path)
		case name == "dockerfile" || strings.HasPrefix(name, "dockerfile."):
			found.Dockerfiles = append(found.Dockerfiles, path)
		}
		return nil
	})
	if err != nil {
		return found, err
	}
	if len(found.ComposeFiles) == 0 && len(found.Dockerfiles) == 0 {
		return found, errors.New("nenhum docker-compose ou Dockerfile encontrado nessa pasta")
	}
	return found, nil
}

// ListComposeServices le os servicos declarados no arquivo compose informado.
func (a *App) ListComposeServices(composeFile string) ([]string, error) {
	composeFile = strings.TrimSpace(composeFile)
	if composeFile == "" {
		return nil, errors.New("selecione o arquivo compose")
	}
	if _, err := os.Stat(composeFile); err != nil {
		return nil, fmt.Errorf("arquivo compose invalido: %w", err)
	}
	if composeCommand() == "" {
		return nil, errors.New("docker compose nao encontrado neste sistema")
	}
	output, err := runComposeCLI(45*time.Second, "-f", composeFile, "config", "--services")
	if err != nil {
		return nil, err
	}
	services := []string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "time=") && !strings.HasPrefix(line, "WARN") {
			services = append(services, line)
		}
	}
	if len(services) == 0 {
		return nil, errors.New("nenhum servico encontrado no compose")
	}
	return services, nil
}

// validateDockerConfig confere o minimo que cada modo precisa antes de salvar ou rodar.
func validateDockerConfig(docker DockerConfig) error {
	switch strings.TrimSpace(docker.Mode) {
	case "dockerfile":
		if strings.TrimSpace(docker.Dockerfile) == "" {
			return errors.New("selecione o Dockerfile do perfil")
		}
	case "image":
		if strings.TrimSpace(docker.Image) == "" {
			return errors.New("informe a imagem do perfil. Ex: postgres:16")
		}
	default:
		if strings.TrimSpace(docker.ComposeFile) == "" {
			return errors.New("selecione o arquivo docker-compose do perfil")
		}
	}
	return nil
}

// dockerWorkingDirectory escolhe de onde a CLI do docker sera chamada quando o
// perfil nao aponta para uma pasta de projeto (caso tipico do modo imagem).
func dockerWorkingDirectory(config ProjectConfig) string {
	if path := strings.TrimSpace(config.Path); path != "" {
		return path
	}
	if compose := strings.TrimSpace(config.Docker.ComposeFile); compose != "" {
		return filepath.Dir(compose)
	}
	if dockerfile := strings.TrimSpace(config.Docker.Dockerfile); dockerfile != "" {
		return filepath.Dir(dockerfile)
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return os.TempDir()
}

// buildDockerCommand traduz o perfil docker na linha de comando executada pelo runner.
func (a *App) buildDockerCommand(config ProjectConfig) (string, error) {
	if !commandExists("docker") {
		return "", errors.New("Docker CLI nao encontrada. Instale o Docker Desktop.")
	}
	docker := config.Docker
	mode := strings.TrimSpace(docker.Mode)
	if mode == "" {
		mode = "compose"
	}

	switch mode {
	case "compose":
		return a.buildComposeCommand(docker)
	case "dockerfile":
		return a.buildDockerfileCommand(config, docker)
	case "image":
		return a.buildImageCommand(docker)
	default:
		return "", fmt.Errorf("modo docker desconhecido: %s", mode)
	}
}

func (a *App) buildComposeCommand(docker DockerConfig) (string, error) {
	compose := composeCommand()
	if compose == "" {
		return "", errors.New("docker compose nao encontrado neste sistema")
	}
	file := strings.TrimSpace(docker.ComposeFile)
	if file == "" {
		return "", errors.New("selecione o arquivo docker-compose do perfil")
	}
	if _, err := os.Stat(file); err != nil {
		return "", fmt.Errorf("arquivo compose nao encontrado: %s", file)
	}

	parts := []string{compose, "-f", shellQuote(file)}
	if project := strings.TrimSpace(docker.ProjectName); project != "" {
		parts = append(parts, "-p", shellQuote(project))
	}
	parts = append(parts, "up")
	if docker.Build {
		parts = append(parts, "--build")
	}
	if docker.Recreate {
		parts = append(parts, "--force-recreate")
	}
	if extra := strings.TrimSpace(docker.ExtraArgs); extra != "" {
		parts = append(parts, extra)
	}
	if service := strings.TrimSpace(docker.Service); service != "" {
		parts = append(parts, service)
	}
	return strings.Join(parts, " "), nil
}

func (a *App) buildDockerfileCommand(config ProjectConfig, docker DockerConfig) (string, error) {
	dockerfile := strings.TrimSpace(docker.Dockerfile)
	if dockerfile == "" {
		return "", errors.New("selecione o Dockerfile do perfil")
	}
	if _, err := os.Stat(dockerfile); err != nil {
		return "", fmt.Errorf("Dockerfile nao encontrado: %s", dockerfile)
	}
	buildContext := strings.TrimSpace(docker.Context)
	if buildContext == "" {
		buildContext = strings.TrimSpace(config.Path)
	}
	if buildContext == "" {
		buildContext = filepath.Dir(dockerfile)
	}
	tag := dockerImageTag(config, docker)

	build := strings.Join([]string{
		"docker", "build",
		"-t", shellQuote(tag),
		"-f", shellQuote(dockerfile),
		shellQuote(buildContext),
	}, " ")

	run, err := a.buildRunCommand(docker, tag)
	if err != nil {
		return "", err
	}
	return build + " && " + run, nil
}

func (a *App) buildImageCommand(docker DockerConfig) (string, error) {
	image := strings.TrimSpace(docker.Image)
	if image == "" {
		return "", errors.New("informe a imagem do perfil. Ex: postgres:16")
	}
	return a.buildRunCommand(docker, image)
}

func (a *App) buildRunCommand(docker DockerConfig, image string) (string, error) {
	parts := []string{"docker", "run"}
	if docker.RemoveOnStop {
		parts = append(parts, "--rm")
	}
	if name := strings.TrimSpace(docker.ContainerName); name != "" {
		parts = append(parts, "--name", shellQuote(name))
	}
	for _, mapping := range splitDockerList(docker.Ports) {
		parts = append(parts, "-p", shellQuote(mapping))
	}
	for _, variable := range splitDockerList(docker.EnvVars) {
		parts = append(parts, "-e", shellQuote(variable))
	}
	for _, volume := range splitDockerList(docker.Volumes) {
		parts = append(parts, "-v", shellQuote(volume))
	}
	if extra := strings.TrimSpace(docker.ExtraArgs); extra != "" {
		parts = append(parts, extra)
	}
	parts = append(parts, shellQuote(image))
	if command := strings.TrimSpace(docker.Command); command != "" {
		parts = append(parts, command)
	}
	return strings.Join(parts, " "), nil
}

func dockerImageTag(config ProjectConfig, docker DockerConfig) string {
	if tag := strings.TrimSpace(docker.Image); tag != "" {
		return tag
	}
	name := strings.TrimSpace(config.Name)
	if name == "" {
		name = filepath.Base(strings.TrimSpace(config.Path))
	}
	cleaned := strings.Map(func(letter rune) rune {
		switch {
		case letter >= 'a' && letter <= 'z':
			return letter
		case letter >= '0' && letter <= '9':
			return letter
		case letter == '-' || letter == '_' || letter == '.':
			return letter
		default:
			return '-'
		}
	}, strings.ToLower(name))
	cleaned = strings.Trim(cleaned, "-._")
	if cleaned == "" {
		cleaned = "exectron"
	}
	return cleaned + ":local"
}

// prepareDockerRun limpa um container antigo de mesmo nome antes de subir de novo.
func (a *App) prepareDockerRun(config ProjectConfig) {
	name := strings.TrimSpace(config.Docker.ContainerName)
	if name == "" || strings.TrimSpace(config.Docker.Mode) == "compose" {
		return
	}
	output, err := runDockerCLI(15*time.Second, "ps", "-aq", "--filter", "name=^/"+name+"$")
	if err != nil || strings.TrimSpace(output) == "" {
		return
	}
	a.emitProgress("docker", "removendo container anterior "+name)
	_, _ = runDockerCLI(60*time.Second, "rm", "-f", name)
}

// cleanupDockerRun derruba o que ficou de pe depois que o processo do runner morre.
// Matar a CLI nao para os containers, entao o encerramento precisa ser explicito.
func (a *App) cleanupDockerRun(config ProjectConfig) {
	docker := config.Docker
	if strings.TrimSpace(docker.Mode) == "compose" {
		file := strings.TrimSpace(docker.ComposeFile)
		if file == "" || composeCommand() == "" {
			return
		}
		args := []string{"-f", file}
		if project := strings.TrimSpace(docker.ProjectName); project != "" {
			args = append(args, "-p", project)
		}
		args = append(args, "down")
		a.emitProgress("docker", "encerrando stack compose")
		if _, err := runComposeCLI(3*time.Minute, args...); err != nil {
			a.emitProgress("docker", "falha ao derrubar compose: "+err.Error())
			return
		}
		a.emitProgress("docker", "stack compose encerrada")
		return
	}

	name := strings.TrimSpace(docker.ContainerName)
	if name == "" {
		return
	}
	a.emitProgress("docker", "removendo container "+name)
	if _, err := runDockerCLI(60*time.Second, "rm", "-f", name); err != nil {
		a.emitProgress("docker", "container "+name+" ja estava removido")
		return
	}
	a.emitProgress("docker", "container "+name+" removido")
}

func splitDockerList(value string) []string {
	replaced := strings.NewReplacer("\r", "\n", ",", "\n", ";", "\n").Replace(value)
	items := []string{}
	for _, line := range strings.Split(replaced, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			items = append(items, line)
		}
	}
	return items
}

// firstDockerHostPort le a porta de host declarada no perfil, para que o runner
// ja mostre a URL antes de qualquer linha de log aparecer.
func firstDockerHostPort(docker DockerConfig) int {
	for _, mapping := range splitDockerList(docker.Ports) {
		fields := strings.Split(mapping, ":")
		if len(fields) < 2 {
			continue
		}
		if port := atoiPort(strings.TrimSpace(fields[len(fields)-2])); port > 0 {
			return port
		}
	}
	return 0
}

// firstPublishedPort extrai a porta de host de uma string como
// "0.0.0.0:8080->80/tcp, :::8080->80/tcp".
func firstPublishedPort(ports string) int {
	for _, entry := range strings.Split(ports, ",") {
		entry = strings.TrimSpace(entry)
		arrow := strings.Index(entry, "->")
		if arrow < 0 {
			continue
		}
		host := entry[:arrow]
		colon := strings.LastIndex(host, ":")
		if colon < 0 {
			continue
		}
		if port := atoiPort(strings.TrimSpace(host[colon+1:])); port > 0 {
			return port
		}
	}
	return 0
}

func parseDockerLabels(labels string) map[string]string {
	parsed := map[string]string{}
	for _, entry := range strings.Split(labels, ",") {
		key, value, found := strings.Cut(entry, "=")
		if found {
			parsed[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return parsed
}

func shortDockerID(id string) string {
	id = strings.TrimPrefix(strings.TrimSpace(id), "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
