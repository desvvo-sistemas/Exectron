//go:build windows

package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

const uninstallKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\Exectron`

func defaultInstallDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(base, "Programs", appName), nil
}

func executableName() string {
	return appSlug + ".exe"
}

func uninstallerName() string {
	return appSlug + "-uninstall.exe"
}

func hiddenProcessAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}

// stopRunningApplication libera o executavel: o Windows nao deixa sobrescrever
// um binario que esta em execucao, e o app vive na bandeja.
func stopRunningApplication(binaryPath string) error {
	removeStaleCleanupHelpers()
	if _, err := os.Stat(binaryPath); err != nil {
		return nil
	}
	output, err := runShort("tasklist", "/FI", "IMAGENAME eq "+executableName(), "/NH")
	if err != nil || !strings.Contains(strings.ToLower(output), strings.ToLower(executableName())) {
		return nil
	}
	detail("encerrando a versao em execucao")
	_, _ = runShort("taskkill", "/IM", executableName(), "/F")
	return nil
}

// removeStaleCleanupHelpers apaga as copias que desinstalacoes anteriores
// deixaram no TEMP. Elas nao conseguem se autoexcluir, entao a limpeza fica
// para a proxima instalacao.
func removeStaleCleanupHelpers() {
	matches, err := filepath.Glob(filepath.Join(os.TempDir(), appSlug+"-cleanup-*.exe"))
	if err != nil {
		return
	}
	for _, path := range matches {
		_ = os.Remove(path)
	}
}

// ---------------------------------------------------------------- pacotes

func archiveExtension() string {
	return ".zip"
}

func nodeArchiveRootName(version string) string {
	return "node-v" + version + "-win-" + nodeArch()
}

func nodeArch() string {
	if runtime.GOARCH == "arm64" {
		return "arm64"
	}
	return "x64"
}

// No Windows o pacote do Node ja traz node.exe na raiz da versao.
func managedNodeBinDir(versionRoot string) string {
	return versionRoot
}

func extractArchive(source string, destination string) error {
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
		target, err := filepath.Abs(filepath.Join(cleanDestination, file.Name))
		if err != nil {
			return err
		}
		if !strings.HasPrefix(target, cleanDestination+string(os.PathSeparator)) && target != cleanDestination {
			return fmt.Errorf("arquivo invalido no pacote: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := copyZipEntry(file, target); err != nil {
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

// ------------------------------------------------------- atalhos e registro

func registerApplication(installDir string, binaryPath string) error {
	// O atalho e a entrada de desinstalacao apontam para o executavel: o icone
	// esta embutido nele como recurso, entao nao ha arquivo de imagem a indicar.
	startMenu := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs")
	if err := createShortcut(filepath.Join(startMenu, appName+".lnk"), binaryPath, installDir); err != nil {
		return err
	}
	if desktop, err := os.UserHomeDir(); err == nil {
		_ = createShortcut(filepath.Join(desktop, "Desktop", appName+".lnk"), binaryPath, installDir)
	}

	uninstaller := filepath.Join(installDir, uninstallerName())
	entries := [][]string{
		{"DisplayName", "REG_SZ", appName},
		{"DisplayVersion", "REG_SZ", appVersion},
		{"Publisher", "REG_SZ", "desvvo-sistemas"},
		{"InstallLocation", "REG_SZ", installDir},
		{"DisplayIcon", "REG_SZ", binaryPath},
		{"UninstallString", "REG_SZ", `"` + uninstaller + `" --uninstall`},
		{"QuietUninstallString", "REG_SZ", `"` + uninstaller + `" --uninstall --silent`},
	}
	for _, entry := range entries {
		if _, err := runShort("reg", "add", uninstallKey, "/v", entry[0], "/t", entry[1], "/d", entry[2], "/f"); err != nil {
			return fmt.Errorf("nao consegui registrar o desinstalador: %w", err)
		}
	}
	_, _ = runShort("reg", "add", uninstallKey, "/v", "NoModify", "/t", "REG_DWORD", "/d", "1", "/f")
	_, _ = runShort("reg", "add", uninstallKey, "/v", "NoRepair", "/t", "REG_DWORD", "/d", "1", "/f")
	return nil
}

func unregisterApplication(installDir string) error {
	_, _ = runShort("reg", "delete", uninstallKey, "/f")

	startMenu := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", appName+".lnk")
	_ = os.Remove(startMenu)
	if home, err := os.UserHomeDir(); err == nil {
		_ = os.Remove(filepath.Join(home, "Desktop", appName+".lnk"))
	}
	removeFromUserPath(installDir)
	return nil
}

// removeInstallDir apaga a instalacao. O desinstalador roda de dentro da
// propria pasta e o Windows nao deixa apagar um executavel em uso, entao ele
// limpa o resto e agenda a remocao da pasta para depois de sair.
func removeInstallDir(dir string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	self, _ = filepath.Abs(self)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	pending := false
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if strings.EqualFold(path, self) {
			pending = true
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}

	if !pending {
		return os.RemoveAll(dir)
	}
	if err := scheduleDirRemoval(dir); err != nil {
		return err
	}
	detail("a pasta some assim que esta janela fechar")
	return nil
}

// scheduleDirRemoval deixa uma copia do proprio instalador rodando fora da
// pasta para apaga-la assim que este processo sair. Delegar para o "rd" do cmd
// nao funciona de forma confiavel por causa das regras de citacao do shell.
func scheduleDirRemoval(dir string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(self)
	if err != nil {
		return err
	}

	helper, err := os.CreateTemp("", appSlug+"-cleanup-*.exe")
	if err != nil {
		return err
	}
	helperPath := helper.Name()
	if _, err := helper.Write(data); err != nil {
		helper.Close()
		return err
	}
	if err := helper.Close(); err != nil {
		return err
	}

	const detachedProcess = 0x00000008
	cmd := exec.Command(helperPath, "--cleanup", dir)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: detachedProcess}
	return cmd.Start()
}

// createShortcut usa o WScript.Shell porque o formato .lnk e binario e
// proprietario; o COM do proprio Windows evita reimplementa-lo aqui.
func createShortcut(linkPath string, target string, workingDir string) error {
	if err := os.MkdirAll(filepath.Dir(linkPath), 0755); err != nil {
		return err
	}
	script := strings.Join([]string{
		"$shell = New-Object -ComObject WScript.Shell;",
		"$link = $shell.CreateShortcut('" + escapePowerShell(linkPath) + "');",
		"$link.TargetPath = '" + escapePowerShell(target) + "';",
		"$link.WorkingDirectory = '" + escapePowerShell(workingDir) + "';",
		"$link.Description = '" + appName + "';",
		"$link.Save()",
	}, " ")
	if output, err := runShort("powershell", "-NoProfile", "-NonInteractive", "-Command", script); err != nil {
		return fmt.Errorf("atalho %s: %s", filepath.Base(linkPath), strings.TrimSpace(output))
	}
	return nil
}

func escapePowerShell(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

// ------------------------------------------------------------------- PATH

func addToUserPath(dir string) error {
	current := readUserPath()
	for _, item := range filepath.SplitList(current) {
		if strings.EqualFold(filepath.Clean(strings.Trim(item, `"`)), filepath.Clean(dir)) {
			return nil
		}
	}
	next := dir
	if strings.TrimSpace(current) != "" {
		next = dir + string(os.PathListSeparator) + current
	}
	if _, err := runShort("reg", "add", `HKCU\Environment`, "/v", "Path", "/t", "REG_EXPAND_SZ", "/d", next, "/f"); err != nil {
		return fmt.Errorf("nao consegui atualizar o PATH do usuario: %w", err)
	}
	broadcastEnvironmentChange()
	detail("PATH do usuario atualizado com %s", dir)
	return nil
}

func removeFromUserPath(prefix string) {
	current := readUserPath()
	kept := []string{}
	for _, item := range filepath.SplitList(current) {
		clean := filepath.Clean(strings.Trim(item, `"`))
		if strings.HasPrefix(strings.ToLower(clean), strings.ToLower(filepath.Clean(prefix))) {
			continue
		}
		kept = append(kept, item)
	}
	_, _ = runShort("reg", "add", `HKCU\Environment`, "/v", "Path", "/t", "REG_EXPAND_SZ",
		"/d", strings.Join(kept, string(os.PathListSeparator)), "/f")
	broadcastEnvironmentChange()
}

func readUserPath() string {
	output, err := runShort("reg", "query", `HKCU\Environment`, "/v", "Path")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.EqualFold(fields[0], "Path") {
			return strings.Join(fields[2:], " ")
		}
	}
	return ""
}

func broadcastEnvironmentChange() {
	user32 := syscall.NewLazyDLL("user32.dll")
	sendMessageTimeout := user32.NewProc("SendMessageTimeoutW")
	environment, err := syscall.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	const (
		hwndBroadcast   = 0xffff
		wmSettingChange = 0x001a
		smtoAbortIfHung = 0x0002
		waitMS          = 5000
	)
	sendMessageTimeout.Call(uintptr(hwndBroadcast), uintptr(wmSettingChange), 0,
		uintptr(unsafe.Pointer(environment)), uintptr(smtoAbortIfHung), uintptr(waitMS), 0)
}
