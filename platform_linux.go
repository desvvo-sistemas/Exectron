//go:build linux

package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

/*
Parte especifica do Linux: shell, arvore de processos, pacote do Node e a troca
de versao, que aqui escreve nos perfis de shell e nos atalhos de ~/.local/bin.
O contrato implementado aqui esta espelhado em platform_windows.go.
*/

func shellName() string {
	return "sh"
}

func shellFlag() string {
	return "-c"
}

func shellQuote(value string) string {
	value = strings.TrimSpace(value)
	return `'` + strings.ReplaceAll(value, `'`, `'\''`) + `'`
}

// processGroupAttr coloca o processo em um grupo proprio para que o Stop
// alcance tambem os filhos (o npm, por exemplo, sobe outro processo).
func processGroupAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

func hiddenProcessAttr() *syscall.SysProcAttr {
	return nil
}

func stopProcessTree(cmd *exec.Cmd) error {
	return signalProcessTree(cmd, syscall.SIGTERM)
}

func killProcessTree(cmd *exec.Cmd) error {
	return signalProcessTree(cmd, syscall.SIGKILL)
}

func signalProcessTree(cmd *exec.Cmd, signal syscall.Signal) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return cmd.Process.Signal(signal)
	}
	return syscall.Kill(-pgid, signal)
}

// ---------------------------------------------------------------- pacote node

func nodeExecutableName() string {
	return "node"
}

// No Linux o pacote oficial traz os binarios em <raiz>/bin.
func managedNodeBin(version string) string {
	return filepath.Join(managedNodeRoot(version), "bin")
}

func nodeArchiveURL(version string) string {
	version = normalizeNodeVersion(version)
	return fmt.Sprintf("https://nodejs.org/dist/v%s/node-v%s-linux-x64.tar.gz", version, version)
}

// O tar.gz e preferido ao tar.xz oficial porque o gzip vem na biblioteca padrao.
func nodeArchiveExtension() string {
	return ".tar.gz"
}

func nodeArchiveRootName(version string) string {
	return "node-v" + normalizeNodeVersion(version) + "-linux-x64"
}

func extractNodeArchive(source string, destination string) error {
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

		target := filepath.Join(cleanDestination, header.Name)
		cleanTarget, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(cleanTarget, cleanDestination+string(os.PathSeparator)) && cleanTarget != cleanDestination {
			return fmt.Errorf("arquivo tar invalido: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(cleanTarget, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := copyTarEntry(reader, cleanTarget, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(cleanTarget), 0755); err != nil {
				return err
			}
			_ = os.Remove(cleanTarget)
			if err := os.Symlink(header.Linkname, cleanTarget); err != nil {
				return err
			}
		}
	}
}

func copyTarEntry(reader io.Reader, target string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	destination, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, reader)
	return err
}

// ------------------------------------------------------------- troca de versao

func (a *App) activateNodeVersionPlatform(version string, nodeBin string) error {
	a.emitProgress("node", "atualizando os perfis de shell do usuario")
	if err := writeShellNodeProfiles(nodeBin); err != nil {
		a.emitProgress("node", "falha ao atualizar os perfis de shell: "+err.Error())
		return err
	}

	a.emitProgress("node", "atualizando os atalhos em ~/.local/bin")
	if err := linkManagedNodeBinaries(nodeBin); err != nil {
		a.emitProgress("node", "falha ao criar os atalhos: "+err.Error())
		return err
	}

	a.emitProgress("node", "Node "+version+" ativo. Abra um novo terminal para ver node --version atualizado.")
	return nil
}

// writeShellNodeProfiles grava o mesmo bloco marcado em cada perfil de shell,
// para que a troca valha em terminais novos independente do shell usado.
func writeShellNodeProfiles(nodeBin string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	profiles := []string{
		filepath.Join(home, ".profile"),
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".zshrc"),
	}
	for _, profile := range profiles {
		// Perfis que ainda nao existem so sao criados para o .profile, que e
		// o unico lido por todos os shells de login.
		if _, err := os.Stat(profile); errors.Is(err, os.ErrNotExist) && !strings.HasSuffix(profile, ".profile") {
			continue
		}
		if err := writeShellNodeProfile(profile, nodeBin); err != nil {
			return err
		}
	}
	return nil
}

func writeShellNodeProfile(profilePath string, nodeBin string) error {
	const start = "# >>> Starter Project Node >>>"
	const end = "# <<< Starter Project Node <<<"
	block := strings.Join([]string{
		start,
		`STARTER_PROJECT_NODE="` + nodeBin + `"`,
		`if [ -d "$STARTER_PROJECT_NODE" ]; then`,
		`    PATH="$STARTER_PROJECT_NODE:$(printf '%s' "$PATH" | tr ':' '\n' | grep -v '/starter-project/node/' | paste -sd ':' -)"`,
		`    export PATH`,
		`fi`,
		end,
	}, "\n")

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

// linkManagedNodeBinaries aponta ~/.local/bin para a versao ativa, o que faz a
// troca valer tambem para atalhos e aplicativos que nao leem o perfil do shell.
func linkManagedNodeBinaries(nodeBin string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	localBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(localBin, 0755); err != nil {
		return err
	}

	for _, name := range []string{"node", "npm", "npx", "corepack"} {
		source := filepath.Join(nodeBin, name)
		if _, err := os.Lstat(source); err != nil {
			continue
		}
		link := filepath.Join(localBin, name)
		// So substitui o que ja for um link nosso: um binario instalado pela
		// distribuicao no mesmo caminho fica intacto.
		if info, err := os.Lstat(link); err == nil {
			if info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			if target, err := os.Readlink(link); err == nil && !strings.Contains(target, "starter-project") {
				continue
			}
			if err := os.Remove(link); err != nil {
				return err
			}
		}
		if err := os.Symlink(source, link); err != nil {
			return err
		}
	}
	return nil
}
