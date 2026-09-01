//go:build windows

package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

/*
Parte especifica do Windows: shell, arvore de processos, pacote do Node e a
troca de versao, que aqui mexe no PATH do registro e nos profiles do PowerShell.
O contrato implementado aqui esta espelhado em platform_linux.go.
*/

func shellName() string {
	return "cmd"
}

func shellFlag() string {
	return "/C"
}

func shellQuote(value string) string {
	value = strings.TrimSpace(value)
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func processGroupAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP, HideWindow: true}
}

// hiddenProcessAttr evita o flash de console nas chamadas curtas a CLIs.
func hiddenProcessAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}

func stopProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(cmd.Process.Pid)).Run()
}

func killProcessTree(cmd *exec.Cmd) error {
	return stopProcessTree(cmd)
}

// ---------------------------------------------------------------- pacote node

func nodeExecutableName() string {
	return "node.exe"
}

// No Windows o zip oficial ja traz node.exe na raiz da versao.
func managedNodeBin(version string) string {
	return managedNodeRoot(version)
}

func nodeArchiveURL(version string) string {
	version = normalizeNodeVersion(version)
	return fmt.Sprintf("https://nodejs.org/dist/v%s/node-v%s-win-x64.zip", version, version)
}

func nodeArchiveExtension() string {
	return ".zip"
}

func nodeArchiveRootName(version string) string {
	return "node-v" + normalizeNodeVersion(version) + "-win-x64"
}

func extractNodeArchive(source string, destination string) error {
	reader, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer reader.Close()

	cleanDestination, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	for _, file := range reader.File {
		target := filepath.Join(cleanDestination, file.Name)
		cleanTarget, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(cleanTarget, cleanDestination+string(os.PathSeparator)) && cleanTarget != cleanDestination {
			return fmt.Errorf("arquivo zip invalido: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(cleanTarget, file.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(cleanTarget), 0755); err != nil {
			return err
		}
		if err := copyZipEntry(file, cleanTarget); err != nil {
			return err
		}
	}
	return nil
}

func copyZipEntry(file *zip.File, target string) error {
	source, err := file.Open()
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

// ------------------------------------------------------------- troca de versao

func (a *App) activateNodeVersionPlatform(version string, nodeBin string) error {
	a.emitProgress("node", "atualizando PATH global da maquina no registro do Windows")
	machinePath := readMachinePath()
	if err := writeMachinePath(withManagedNodeFirst(machinePath, nodeBin)); err != nil {
		a.emitProgress("node", "falha ao atualizar PATH global. Abra o aplicativo como administrador: "+err.Error())
		return errors.New("nao foi possivel trocar o Node globalmente sem permissao de administrador")
	}

	a.emitProgress("node", "atualizando PATH do usuario no registro do Windows")
	if err := writeUserPath(withManagedNodeFirst(readUserPath(), nodeBin)); err != nil {
		a.emitProgress("node", "falha ao atualizar PATH do usuario: "+err.Error())
		return err
	}

	a.emitProgress("node", "atualizando profile do PowerShell para priorizar o Node gerenciado")
	if err := writePowerShellNodeProfiles(nodeBin); err != nil {
		a.emitProgress("node", "falha ao atualizar profile do PowerShell: "+err.Error())
		return err
	}

	broadcastEnvironmentChange()
	a.emitProgress("node", "Node "+version+" ativo globalmente. Feche e abra um novo terminal para ver node --version atualizado.")
	return nil
}

func writePowerShellNodeProfiles(nodeBin string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	profiles := []string{
		filepath.Join(home, "Documents", "PowerShell", "profile.ps1"),
		filepath.Join(home, "Documents", "WindowsPowerShell", "profile.ps1"),
	}
	for _, profile := range profiles {
		if err := writePowerShellNodeProfile(profile, nodeBin); err != nil {
			return err
		}
	}
	return nil
}

func writePowerShellNodeProfile(profilePath string, nodeBin string) error {
	const start = "# >>> Starter Project Node >>>"
	const end = "# <<< Starter Project Node <<<"
	escapedNodeBin := strings.ReplaceAll(nodeBin, "'", "''")
	block := strings.Join([]string{
		start,
		"$starterProjectNode = '" + escapedNodeBin + "'",
		"if (Test-Path $starterProjectNode) {",
		"    $env:PATH = $starterProjectNode + ';' + (($env:PATH -split ';' | Where-Object { $_ -and ($_ -notlike '*\\starter-project\\node\\*') }) -join ';')",
		"}",
		end,
	}, "\r\n")

	if err := os.MkdirAll(filepath.Dir(profilePath), 0755); err != nil {
		return err
	}

	content := ""
	if data, err := os.ReadFile(profilePath); err == nil {
		content = string(data)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return os.WriteFile(profilePath, []byte(replaceMarkedBlock(content, start, end, block)), 0644)
}

// ------------------------------------------------------------------ registro

func readUserPath() string {
	return readRegistryPath(`HKCU\Environment`, os.Getenv("PATH"))
}

func readMachinePath() string {
	return readRegistryPath(`HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, "")
}

func readRegistryPath(key string, fallback string) string {
	output, err := exec.Command("reg", "query", key, "/v", "Path").CombinedOutput()
	if err != nil {
		return fallback
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.EqualFold(fields[0], "Path") {
			return strings.Join(fields[2:], " ")
		}
	}
	return fallback
}

func writeUserPath(value string) error {
	return exec.Command("reg", "add", `HKCU\Environment`, "/v", "Path", "/t", "REG_EXPAND_SZ", "/d", value, "/f").Run()
}

func writeMachinePath(value string) error {
	return exec.Command("reg", "add", `HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, "/v", "Path", "/t", "REG_EXPAND_SZ", "/d", value, "/f").Run()
}

// broadcastEnvironmentChange avisa o Explorer que o PATH mudou, para que
// terminais novos ja abram com a versao recem ativada.
func broadcastEnvironmentChange() {
	user32 := syscall.NewLazyDLL("user32.dll")
	sendMessageTimeout := user32.NewProc("SendMessageTimeoutW")
	environment, err := syscall.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	const (
		hwndBroadcast     = 0xffff
		wmSettingChange   = 0x001a
		smtoAbortIfHung   = 0x0002
		environmentWaitMS = 5000
	)
	sendMessageTimeout.Call(
		uintptr(hwndBroadcast),
		uintptr(wmSettingChange),
		0,
		uintptr(unsafe.Pointer(environment)),
		uintptr(smtoAbortIfHung),
		uintptr(environmentWaitMS),
		0,
	)
}

// withManagedNodeFirst poe a versao escolhida na frente e tira do PATH as
// outras versoes gerenciadas, para nao acumular entradas a cada troca.
func withManagedNodeFirst(currentPath string, nodeBin string) string {
	root := strings.ToLower(filepath.Clean(nodeVersionsDir()))
	selected := strings.ToLower(filepath.Clean(nodeBin))
	seen := map[string]bool{selected: true}
	parts := []string{nodeBin}
	for _, item := range filepath.SplitList(currentPath) {
		clean := strings.ToLower(filepath.Clean(strings.Trim(item, `"`)))
		if clean == "." || clean == selected || strings.HasPrefix(clean, root+string(os.PathSeparator)) {
			continue
		}
		if seen[clean] {
			continue
		}
		parts = append(parts, item)
		seen[clean] = true
	}
	return strings.Join(parts, string(os.PathListSeparator))
}
