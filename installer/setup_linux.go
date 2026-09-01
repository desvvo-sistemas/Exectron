//go:build linux

package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

func defaultInstallDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", appSlug), nil
}

func executableName() string {
	return appSlug
}

func uninstallerName() string {
	return appSlug + "-uninstall"
}

func hiddenProcessAttr() *syscall.SysProcAttr {
	return nil
}

func stopRunningApplication(binaryPath string) error {
	if _, err := os.Stat(binaryPath); err != nil {
		return nil
	}
	if output, _ := runShort("pgrep", "-x", executableName()); strings.TrimSpace(output) != "" {
		detail("encerrando a versao em execucao")
		_, _ = runShort("pkill", "-x", executableName())
	}
	return nil
}

// ---------------------------------------------------------------- pacotes

func archiveExtension() string {
	return ".tar.gz"
}

func nodeArchiveRootName(version string) string {
	return "node-v" + version + "-linux-" + nodeArch()
}

func nodeArch() string {
	if runtime.GOARCH == "arm64" {
		return "arm64"
	}
	return "x64"
}

// No Linux o pacote do Node traz os binarios em <raiz>/bin.
func managedNodeBinDir(versionRoot string) string {
	return filepath.Join(versionRoot, "bin")
}

func extractArchive(source string, destination string) error {
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()

	decompressor, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer decompressor.Close()

	cleanDestination, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cleanDestination, 0755); err != nil {
		return err
	}

	reader := tar.NewReader(decompressor)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		target, err := filepath.Abs(filepath.Join(cleanDestination, header.Name))
		if err != nil {
			return err
		}
		if !strings.HasPrefix(target, cleanDestination+string(os.PathSeparator)) && target != cleanDestination {
			return fmt.Errorf("arquivo invalido no pacote: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			destination, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			_, err = io.Copy(destination, reader)
			closeErr := destination.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		}
	}
}

// -------------------------------------------------------- atalhos do sistema

func registerApplication(installDir string, binaryPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	iconPath := filepath.Join(installDir, appSlug+".png")
	if _, err := os.Stat(iconPath); err != nil {
		iconPath = appSlug
	}

	entry := strings.Join([]string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=" + appName,
		"Comment=Runner local de projetos, Node e containers",
		"Exec=" + binaryPath,
		"Icon=" + iconPath,
		"Terminal=false",
		"Categories=Development;Utility;",
		"StartupNotify=true",
		"",
	}, "\n")

	applications := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(applications, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(applications, appSlug+".desktop"), []byte(entry), 0644); err != nil {
		return err
	}
	_, _ = runShort("update-desktop-database", applications)

	// Um link em ~/.local/bin deixa o comando disponivel no terminal.
	localBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(localBin, 0755); err == nil {
		link := filepath.Join(localBin, appSlug)
		_ = os.Remove(link)
		_ = os.Symlink(binaryPath, link)
	}
	return addToUserPath(localBin)
}

func unregisterApplication(installDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(home, ".local", "share", "applications", appSlug+".desktop"))
	_ = os.Remove(filepath.Join(home, ".local", "bin", appSlug))
	return nil
}

// ------------------------------------------------------------------- PATH

// addToUserPath grava um bloco marcado no ~/.profile. O marcador permite
// reescrever o bloco nas proximas execucoes em vez de acumular linhas.
func addToUserPath(dir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	profile := filepath.Join(home, ".profile")

	content := ""
	if data, err := os.ReadFile(profile); err == nil {
		content = string(data)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	const start = "# >>> Exectron >>>"
	const end = "# <<< Exectron <<<"

	// Preserva os diretorios ja adicionados por execucoes anteriores.
	dirs := []string{dir}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, `PATH="`) || !strings.Contains(line, "$PATH") {
			continue
		}
		if existing := strings.TrimSuffix(strings.TrimPrefix(line, `PATH="`), `:$PATH"`); existing != "" && existing != dir {
			if !contains(dirs, existing) {
				dirs = append(dirs, existing)
			}
		}
	}

	lines := []string{start}
	for _, item := range dirs {
		lines = append(lines, `PATH="`+item+`:$PATH"`)
	}
	lines = append(lines, "export PATH", end)

	if err := os.WriteFile(profile, []byte(replaceMarkedBlock(content, start, end, strings.Join(lines, "\n"))), 0644); err != nil {
		return err
	}
	detail("PATH atualizado em %s", profile)
	return nil
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func replaceMarkedBlock(content string, start string, end string, block string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	startIndex := strings.Index(content, start)
	endIndex := -1
	if startIndex >= 0 {
		searchFrom := startIndex + len(start)
		if relative := strings.Index(content[searchFrom:], end); relative >= 0 {
			endIndex = searchFrom + relative + len(end)
		}
	}
	if startIndex >= 0 && endIndex >= startIndex {
		prefix := strings.TrimRight(content[:startIndex], "\n")
		suffix := strings.TrimLeft(content[endIndex:], "\n")
		parts := []string{}
		if prefix != "" {
			parts = append(parts, prefix)
		}
		parts = append(parts, block)
		if suffix != "" {
			parts = append(parts, suffix)
		}
		return strings.Join(parts, "\n\n") + "\n"
	}
	if strings.TrimSpace(content) == "" {
		return block + "\n"
	}
	return strings.TrimRight(content, "\n") + "\n\n" + block + "\n"
}

// removeInstallDir no Linux e direto: o sistema permite apagar um binario que
// ainda esta em execucao.
func removeInstallDir(dir string) error {
	return os.RemoveAll(dir)
}
