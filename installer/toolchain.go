package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

/*
Provisionamento dos toolchains que os projetos rodados pelo Exectron precisam.
Nada disso e requisito do aplicativo em si, que e um binario compilado: o Go
serve aos perfis Go e o Node aos perfis Node.
*/

const goVersionEndpoint = "https://go.dev/VERSION?m=text"

// provisionGo instala o Go dentro da pasta do aplicativo quando a maquina
// ainda nao tem um. Uma instalacao existente do sistema e respeitada.
func provisionGo(installDir string) error {
	if path, err := exec.LookPath("go"); err == nil {
		if version, err := runShort("go", "version"); err == nil {
			detail("Go ja instalado: %s", strings.TrimSpace(version))
		} else {
			detail("Go ja instalado em %s", path)
		}
		return nil
	}

	goRoot := filepath.Join(installDir, "toolchain", "go")
	if _, err := os.Stat(filepath.Join(goRoot, "bin", goBinaryName())); err == nil {
		detail("Go ja provisionado em %s", goRoot)
		return addToUserPath(filepath.Join(goRoot, "bin"))
	}

	version, err := latestGoVersion()
	if err != nil {
		return fmt.Errorf("nao consegui descobrir a versao do Go: %w", err)
	}
	url := goDownloadURL(version)
	detail("baixando %s", url)

	archive, err := downloadToTemp(url, "go-"+version+archiveExtension())
	if err != nil {
		return err
	}
	defer os.Remove(archive)

	detail("extraindo o toolchain")
	staging := filepath.Join(installDir, "toolchain", "staging-go")
	_ = os.RemoveAll(staging)
	defer os.RemoveAll(staging)
	if err := extractArchive(archive, staging); err != nil {
		return err
	}

	// Os pacotes oficiais trazem tudo dentro de uma pasta "go".
	_ = os.RemoveAll(goRoot)
	if err := os.MkdirAll(filepath.Dir(goRoot), 0755); err != nil {
		return err
	}
	if err := os.Rename(filepath.Join(staging, "go"), goRoot); err != nil {
		return fmt.Errorf("pacote do Go inesperado: %w", err)
	}

	detail("Go %s instalado em %s", version, goRoot)
	return addToUserPath(filepath.Join(goRoot, "bin"))
}

func latestGoVersion() (string, error) {
	body, err := httpGet(goVersionEndpoint)
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(strings.SplitN(string(body), "\n", 2)[0])
	if !strings.HasPrefix(line, "go1") {
		return "", fmt.Errorf("resposta inesperada: %q", line)
	}
	return line, nil
}

func goDownloadURL(version string) string {
	return fmt.Sprintf("https://go.dev/dl/%s.%s-%s%s", version, runtime.GOOS, runtime.GOARCH, archiveExtension())
}

func goBinaryName() string {
	if runtime.GOOS == "windows" {
		return "go.exe"
	}
	return "go"
}

// ------------------------------------------------------------------ node

type nodeRelease struct {
	Version string          `json:"version"`
	LTS     json.RawMessage `json:"lts"`
}

// provisionNode baixa a versao LTS para a mesma pasta que o gerenciador de
// versoes do aplicativo usa, de modo que ela ja aparece na aba Config.
func provisionNode(configDir string) error {
	if path, err := exec.LookPath("node"); err == nil {
		if version, err := runShort("node", "-v"); err == nil {
			detail("Node ja instalado: %s", strings.TrimSpace(version))
		} else {
			detail("Node ja instalado em %s", path)
		}
		return nil
	}

	versionsDir := filepath.Join(configDir, "node")
	if entries, err := os.ReadDir(versionsDir); err == nil && len(entries) > 0 {
		detail("Node gerenciado ja presente em %s", versionsDir)
		return nil
	}

	version, err := latestNodeLTS()
	if err != nil {
		return fmt.Errorf("nao consegui descobrir a versao LTS do Node: %w", err)
	}
	url := nodeDownloadURL(version)
	detail("baixando Node %s", version)

	archive, err := downloadToTemp(url, "node-"+version+archiveExtension())
	if err != nil {
		return err
	}
	defer os.Remove(archive)

	detail("extraindo o Node")
	staging := filepath.Join(versionsDir, "staging")
	_ = os.RemoveAll(staging)
	defer os.RemoveAll(staging)
	if err := os.MkdirAll(versionsDir, 0755); err != nil {
		return err
	}
	if err := extractArchive(archive, staging); err != nil {
		return err
	}

	target := filepath.Join(versionsDir, version)
	_ = os.RemoveAll(target)
	if err := os.Rename(filepath.Join(staging, nodeArchiveRootName(version)), target); err != nil {
		return fmt.Errorf("pacote do Node inesperado: %w", err)
	}

	if err := writeActiveNodeVersion(configDir, version); err != nil {
		detail("aviso: nao consegui marcar a versao ativa: %v", err)
	}
	detail("Node %s instalado em %s", version, target)
	return addToUserPath(managedNodeBinDir(target))
}

func latestNodeLTS() (string, error) {
	body, err := httpGet("https://nodejs.org/dist/index.json")
	if err != nil {
		return "", err
	}
	var releases []nodeRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return "", err
	}
	// A lista vem da mais nova para a mais antiga; lts e false ou o codinome.
	for _, release := range releases {
		if len(release.LTS) > 0 && string(release.LTS) != "false" {
			return strings.TrimPrefix(strings.TrimSpace(release.Version), "v"), nil
		}
	}
	return "", fmt.Errorf("nenhuma versao LTS encontrada")
}

func nodeDownloadURL(version string) string {
	return fmt.Sprintf("https://nodejs.org/dist/v%s/%s%s", version, nodeArchiveRootName(version), archiveExtension())
}

// writeActiveNodeVersion deixa a versao recem baixada marcada como ativa para
// o aplicativo, sem apagar as outras preferencias que ja estejam gravadas.
func writeActiveNodeVersion(configDir string, version string) error {
	path := filepath.Join(configDir, "settings.json")
	settings := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &settings)
	}
	settings["activeNodeVersion"] = version

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// ----------------------------------------------------------------- rede

func httpGet(url string) ([]byte, error) {
	client := http.Client{Timeout: 30 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s respondeu %s", url, response.Status)
	}
	return io.ReadAll(io.LimitReader(response.Body, 8<<20))
}

func downloadToTemp(url string, name string) (string, error) {
	client := http.Client{Timeout: 30 * time.Minute}
	response, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download falhou: %s respondeu %s", url, response.Status)
	}

	file, err := os.CreateTemp("", name)
	if err != nil {
		return "", err
	}
	written, err := io.Copy(file, response.Body)
	closeErr := file.Close()
	if err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	if closeErr != nil {
		_ = os.Remove(file.Name())
		return "", closeErr
	}
	detail("%.1f MB baixados", float64(written)/(1<<20))
	return file.Name(), nil
}

// ---------------------------------------------------------------- arquivos

func writeGzipFile(payload []byte, target string, mode os.FileMode) error {
	reader, err := gzip.NewReader(strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	defer reader.Close()

	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	return err
}

func runShort(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = hiddenProcessAttr()
	output, err := cmd.CombinedOutput()
	return string(output), err
}
