package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, name string, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBuildComposeCommand(t *testing.T) {
	if composeCommand() == "" {
		t.Skip("docker compose indisponivel neste ambiente")
	}
	app := NewApp()
	compose := writeFile(t, "docker-compose.yml", "services:\n  api:\n    image: nginx\n")

	command, err := app.buildComposeCommand(DockerConfig{
		Mode:        "compose",
		ComposeFile: compose,
		Service:     "api",
		ProjectName: "minha-stack",
		Build:       true,
		Recreate:    true,
	})
	if err != nil {
		t.Fatalf("buildComposeCommand: %v", err)
	}
	for _, fragment := range []string{"-p " + shellQuote("minha-stack"), "up", "--build", "--force-recreate"} {
		if !strings.Contains(command, fragment) {
			t.Fatalf("comando %q nao contem %q", command, fragment)
		}
	}
	if !strings.HasSuffix(command, " api") {
		t.Fatalf("servico deveria ser o ultimo argumento: %q", command)
	}
}

func TestBuildComposeCommandExigeArquivo(t *testing.T) {
	if composeCommand() == "" {
		t.Skip("docker compose indisponivel neste ambiente")
	}
	app := NewApp()
	if _, err := app.buildComposeCommand(DockerConfig{Mode: "compose"}); err == nil {
		t.Fatal("compose sem arquivo deveria falhar")
	}
	if _, err := app.buildComposeCommand(DockerConfig{Mode: "compose", ComposeFile: "C:/nao/existe.yml"}); err == nil {
		t.Fatal("compose inexistente deveria falhar")
	}
}

func TestBuildImageCommand(t *testing.T) {
	app := NewApp()
	command, err := app.buildImageCommand(DockerConfig{
		Mode:          "image",
		Image:         "postgres:16",
		ContainerName: "pg-local",
		Ports:         "5432:5432, 8080:80",
		EnvVars:       "POSTGRES_PASSWORD=123\nTZ=America/Sao_Paulo",
		Volumes:       "dados:/var/lib/postgresql/data",
		ExtraArgs:     "--pull always",
		RemoveOnStop:  true,
	})
	if err != nil {
		t.Fatalf("buildImageCommand: %v", err)
	}
	for _, fragment := range []string{
		"docker run",
		"--rm",
		"--name " + shellQuote("pg-local"),
		"-p " + shellQuote("5432:5432"),
		"-p " + shellQuote("8080:80"),
		"-e " + shellQuote("POSTGRES_PASSWORD=123"),
		"-e " + shellQuote("TZ=America/Sao_Paulo"),
		"-v " + shellQuote("dados:/var/lib/postgresql/data"),
		"--pull always",
	} {
		if !strings.Contains(command, fragment) {
			t.Fatalf("comando %q nao contem %q", command, fragment)
		}
	}
	if !strings.HasSuffix(command, shellQuote("postgres:16")) {
		t.Fatalf("a imagem deveria fechar o comando: %q", command)
	}

	if _, err := app.buildImageCommand(DockerConfig{Mode: "image"}); err == nil {
		t.Fatal("modo imagem sem imagem deveria falhar")
	}
}

func TestBuildDockerfileCommand(t *testing.T) {
	app := NewApp()
	dockerfile := writeFile(t, "Dockerfile", "FROM nginx\n")
	config := ProjectConfig{Name: "Minha API", Path: filepath.Dir(dockerfile)}

	command, err := app.buildDockerfileCommand(config, DockerConfig{
		Mode:          "dockerfile",
		Dockerfile:    dockerfile,
		ContainerName: "minha-api",
		Ports:         "8080:80",
	})
	if err != nil {
		t.Fatalf("buildDockerfileCommand: %v", err)
	}
	if !strings.HasPrefix(command, "docker build ") {
		t.Fatalf("o build deveria vir primeiro: %q", command)
	}
	if !strings.Contains(command, " && docker run") {
		t.Fatalf("o run deveria vir encadeado ao build: %q", command)
	}
	// Sem imagem informada, a tag sai do nome do perfil ja normalizado.
	if !strings.Contains(command, shellQuote("minha-api:local")) {
		t.Fatalf("tag gerada ausente: %q", command)
	}
}

func TestDockerImageTag(t *testing.T) {
	cases := map[string]string{
		"Minha API":    "minha-api:local",
		"API_Financas": "api_financas:local",
		"  ###  ":      "exectron:local",
		"Checkout.Web": "checkout.web:local",
	}
	for name, expected := range cases {
		if tag := dockerImageTag(ProjectConfig{Name: name}, DockerConfig{}); tag != expected {
			t.Fatalf("dockerImageTag(%q) = %q, esperado %q", name, tag, expected)
		}
	}
	if tag := dockerImageTag(ProjectConfig{Name: "x"}, DockerConfig{Image: "custom:1"}); tag != "custom:1" {
		t.Fatalf("imagem informada deveria vencer, veio %q", tag)
	}
}

func TestSplitDockerList(t *testing.T) {
	items := splitDockerList(" 8080:80 , 5432:5432 \n 9000:9000 ; 7000:70 ")
	expected := []string{"8080:80", "5432:5432", "9000:9000", "7000:70"}
	if strings.Join(items, "|") != strings.Join(expected, "|") {
		t.Fatalf("splitDockerList = %v", items)
	}
	if len(splitDockerList("   ")) != 0 {
		t.Fatal("lista vazia deveria devolver nada")
	}
}

func TestFirstDockerHostPort(t *testing.T) {
	cases := map[string]int{
		"8080:80":             8080,
		"127.0.0.1:9000:9000": 9000,
		"5432:5432, 8080:80":  5432,
		"80":                  0,
		"":                    0,
	}
	for ports, expected := range cases {
		if port := firstDockerHostPort(DockerConfig{Ports: ports}); port != expected {
			t.Fatalf("firstDockerHostPort(%q) = %d, esperado %d", ports, port, expected)
		}
	}
}

func TestFirstPublishedPort(t *testing.T) {
	cases := map[string]int{
		"0.0.0.0:8080->80/tcp, :::8080->80/tcp": 8080,
		"0.0.0.0:5432->5432/tcp":                5432,
		"80/tcp":                                0,
		"":                                      0,
	}
	for ports, expected := range cases {
		if port := firstPublishedPort(ports); port != expected {
			t.Fatalf("firstPublishedPort(%q) = %d, esperado %d", ports, port, expected)
		}
	}
}

func TestParseDockerLabels(t *testing.T) {
	labels := parseDockerLabels("com.docker.compose.project=minha-stack,com.docker.compose.service=api")
	if labels["com.docker.compose.project"] != "minha-stack" || labels["com.docker.compose.service"] != "api" {
		t.Fatalf("labels = %v", labels)
	}
}

func TestValidateDockerConfig(t *testing.T) {
	if err := validateDockerConfig(DockerConfig{Mode: "image", Image: "nginx"}); err != nil {
		t.Fatalf("config valida recusada: %v", err)
	}
	for _, invalid := range []DockerConfig{
		{Mode: "compose"},
		{Mode: "dockerfile"},
		{Mode: "image"},
	} {
		if err := validateDockerConfig(invalid); err == nil {
			t.Fatalf("config incompleta aceita: %+v", invalid)
		}
	}
}

func TestDockerWorkingDirectory(t *testing.T) {
	compose := writeFile(t, "docker-compose.yml", "services: {}\n")
	config := ProjectConfig{Runtime: "docker", Docker: DockerConfig{Mode: "compose", ComposeFile: compose}}
	if dir := dockerWorkingDirectory(config); dir != filepath.Dir(compose) {
		t.Fatalf("diretorio = %q, esperado %q", dir, filepath.Dir(compose))
	}

	config.Path = "C:/projetos/api"
	if dir := dockerWorkingDirectory(config); dir != "C:/projetos/api" {
		t.Fatalf("o caminho do perfil deveria vencer, veio %q", dir)
	}
}

func TestDetectRuntimeDocker(t *testing.T) {
	if runtime := detectRuntime("docker compose up"); runtime != "docker" {
		t.Fatalf("detectRuntime = %q", runtime)
	}
	if runtime := detectRuntime("npm run dev"); runtime != "node" {
		t.Fatalf("detectRuntime = %q", runtime)
	}
}
