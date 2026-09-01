// Instalador do Exectron.
//
// Um unico executavel que deixa a maquina pronta: instala o aplicativo, cria a
// base de perfis vazia, registra atalhos e desinstalador e provisiona os
// toolchains que os projetos precisam (Go e Node). Roda no perfil do usuario,
// sem exigir administrador.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	appName    = "Exectron"
	appSlug    = "exectron"
	appVersion = "1.0.0"
)

type options struct {
	dir       string
	silent    bool
	skipGo    bool
	skipNode  bool
	uninstall bool
	cleanup   string
}

func main() {
	opts := parseFlags()

	// Modo interno, disparado pelo desinstalador: sem banner e sem interacao.
	if opts.cleanup != "" {
		waitAndRemove(opts.cleanup)
		return
	}

	printBanner(opts)

	var err error
	if opts.uninstall {
		err = runUninstall(opts)
	} else {
		err = runInstall(opts)
	}

	if err != nil {
		fmt.Printf("\n  ERRO: %v\n", err)
		waitForEnter(opts)
		os.Exit(1)
	}
	waitForEnter(opts)
}

func parseFlags() options {
	opts := options{}
	flag.StringVar(&opts.dir, "dir", "", "pasta de instalacao (padrao: perfil do usuario)")
	flag.BoolVar(&opts.silent, "silent", false, "instala sem pedir confirmacao nem pausar no final")
	flag.BoolVar(&opts.skipGo, "skip-go", false, "nao provisiona o toolchain Go")
	flag.BoolVar(&opts.skipNode, "skip-node", false, "nao provisiona o Node")
	flag.BoolVar(&opts.uninstall, "uninstall", false, "remove o aplicativo desta maquina")
	flag.StringVar(&opts.cleanup, "cleanup", "", "uso interno: apaga a pasta indicada assim que ela liberar")
	flag.Parse()
	return opts
}

func printBanner(opts options) {
	fmt.Println()
	fmt.Printf("  %s %s - instalador\n", appName, appVersion)
	fmt.Printf("  %s/%s\n", runtime.GOOS, runtime.GOARCH)
	if opts.uninstall {
		fmt.Println("  modo: desinstalacao")
	}
	fmt.Println("  " + strings.Repeat("-", 52))
}

var stepNumber int

func step(format string, args ...any) {
	stepNumber++
	fmt.Printf("\n  [%d] %s\n", stepNumber, fmt.Sprintf(format, args...))
}

func detail(format string, args ...any) {
	fmt.Printf("      %s\n", fmt.Sprintf(format, args...))
}

func runInstall(opts options) error {
	installDir, err := resolveInstallDir(opts)
	if err != nil {
		return err
	}

	step("Instalando o aplicativo em %s", installDir)
	binaryPath, err := installApplication(installDir)
	if err != nil {
		return err
	}
	detail("executavel: %s", binaryPath)

	step("Preparando a base de perfis")
	configDir, err := createEmptyDatabase()
	if err != nil {
		return err
	}
	detail("base vazia em %s", configDir)
	detail("os perfis sao criados por voce dentro do aplicativo")

	step("Registrando atalhos e desinstalador")
	if err := registerApplication(installDir, binaryPath); err != nil {
		// Um atalho que falha nao invalida a instalacao: o app ja esta no disco.
		detail("aviso: %v", err)
	} else {
		detail("atalho criado no menu do sistema")
	}

	if opts.skipGo {
		step("Toolchain Go: ignorado por --skip-go")
	} else {
		step("Verificando o toolchain Go")
		if err := provisionGo(installDir); err != nil {
			detail("aviso: %v", err)
			detail("o aplicativo funciona sem Go; so os projetos Go ficam indisponiveis")
		}
	}

	if opts.skipNode {
		step("Node: ignorado por --skip-node")
	} else {
		step("Verificando o Node")
		if err := provisionNode(configDir); err != nil {
			detail("aviso: %v", err)
			detail("da para instalar o Node depois pela aba Config do aplicativo")
		}
	}

	fmt.Println()
	fmt.Println("  " + strings.Repeat("-", 52))
	fmt.Printf("  %s instalado.\n", appName)
	fmt.Printf("  Abra pelo menu iniciar ou execute: %s\n", binaryPath)
	fmt.Println("  Abra um terminal novo para o PATH atualizado valer.")
	return nil
}

func runUninstall(opts options) error {
	installDir, err := resolveInstallDir(opts)
	if err != nil {
		return err
	}

	if !opts.silent {
		fmt.Printf("\n  Remover %s de %s?\n", appName, installDir)
		fmt.Printf("  Os perfis em %s sao preservados.\n", appConfigDir())
		if !confirm("  Digite s para confirmar: ") {
			fmt.Println("\n  Cancelado.")
			return nil
		}
	}

	step("Removendo atalhos e registro")
	if err := unregisterApplication(installDir); err != nil {
		detail("aviso: %v", err)
	}

	step("Removendo os arquivos do aplicativo")
	if err := removeInstallDir(installDir); err != nil {
		return fmt.Errorf("nao consegui remover %s: %w", installDir, err)
	}

	fmt.Println()
	fmt.Printf("  %s removido. Os perfis continuam em %s\n", appName, appConfigDir())
	return nil
}

// waitAndRemove apaga a pasta assim que o processo que a segurava termina.
// O desinstalador nao consegue se autoexcluir no Windows, entao esta copia
// roda de fora e insiste ate o arquivo liberar.
func waitAndRemove(dir string) {
	deadline := time.Now().Add(30 * time.Second)
	for {
		if err := os.RemoveAll(dir); err == nil {
			return
		}
		if time.Now().After(deadline) {
			return
		}
		time.Sleep(400 * time.Millisecond)
	}
}

// resolveInstallDir normaliza a pasta de destino. Um caminho vindo da linha de
// comando pode chegar com as barras trocadas, e no Windows isso quebra desde os
// atalhos ate o comando que apaga a pasta na desinstalacao.
func resolveInstallDir(opts options) (string, error) {
	dir := opts.dir
	if dir == "" {
		var err error
		dir, err = defaultInstallDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Abs(filepath.FromSlash(dir))
}

// installApplication grava o executavel embutido no pacote.
func installApplication(installDir string) (string, error) {
	if len(appPayload) == 0 {
		return "", fmt.Errorf("este instalador foi montado sem o executavel do %s", appName)
	}
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return "", err
	}

	binaryPath := filepath.Join(installDir, executableName())
	if err := stopRunningApplication(binaryPath); err != nil {
		return "", err
	}
	if err := writeGzipFile(appPayload, binaryPath, 0755); err != nil {
		return "", fmt.Errorf("nao consegui gravar %s: %w", binaryPath, err)
	}

	if len(iconPayload) > 0 {
		_ = os.WriteFile(filepath.Join(installDir, appSlug+".png"), iconPayload, 0644)
	}

	// Uma copia do proprio instalador vira o desinstalador.
	if self, err := os.Executable(); err == nil {
		if data, err := os.ReadFile(self); err == nil {
			_ = os.WriteFile(filepath.Join(installDir, uninstallerName()), data, 0755)
		}
	}
	return binaryPath, nil
}

// createEmptyDatabase garante a pasta de dados com a lista de perfis vazia.
// Uma instalacao existente nunca e sobrescrita.
func createEmptyDatabase() (string, error) {
	configDir := appConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}
	projects := filepath.Join(configDir, "projects.json")
	if _, err := os.Stat(projects); err == nil {
		detail("base ja existente preservada")
		return configDir, nil
	}
	if err := os.WriteFile(projects, []byte("[]\n"), 0644); err != nil {
		return "", err
	}
	return configDir, nil
}

func appConfigDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.TempDir(), appSlug)
	}
	return filepath.Join(base, appSlug)
}

func confirm(prompt string) bool {
	fmt.Print(prompt)
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "s" || answer == "sim" || answer == "y" || answer == "yes"
}

// waitForEnter evita que a janela feche antes de o usuario ler o resultado
// quando o instalador e aberto com dois cliques.
func waitForEnter(opts options) {
	if opts.silent {
		return
	}
	fmt.Print("\n  Pressione Enter para sair. ")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}
