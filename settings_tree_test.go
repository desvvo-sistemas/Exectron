package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleAppSettings = `{
  // comentario que o leitor precisa ignorar
  "Logging": {
    "LogLevel": {
      "Default": "Information",
      "Microsoft.AspNetCore": "Warning"
    }
  },
  "ConnectionStrings": {
    "Default": "Server=localhost;Database=app"
  },
  "AllowedHosts": "*",
  "Porta": 5001,
  "Detalhado": false,
  "Origens": ["http://localhost:3000", "http://localhost:5173"]
}`

func writeSample(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "appsettings.json")
	if err := os.WriteFile(path, []byte(sampleAppSettings), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func findNode(node SettingsNode, path string) (SettingsNode, bool) {
	if node.Path == path {
		return node, true
	}
	for _, child := range node.Children {
		if found, ok := findNode(child, path); ok {
			return found, true
		}
	}
	return SettingsNode{}, false
}

func TestLoadAppSettingsTree(t *testing.T) {
	app := NewApp()
	tree, err := app.LoadAppSettingsTree(writeSample(t))
	if err != nil {
		t.Fatalf("LoadAppSettingsTree: %v", err)
	}

	// A arvore precisa sair na ordem do arquivo, nao em ordem alfabetica.
	var topLevel []string
	for _, child := range tree.Children {
		topLevel = append(topLevel, child.Key)
	}
	expected := []string{"Logging", "ConnectionStrings", "AllowedHosts", "Porta", "Detalhado", "Origens"}
	if strings.Join(topLevel, ",") != strings.Join(expected, ",") {
		t.Fatalf("ordem das chaves = %v, esperado %v", topLevel, expected)
	}

	nested, ok := findNode(tree, "Logging:LogLevel:Default")
	if !ok {
		t.Fatal("no aninhado Logging:LogLevel:Default nao encontrado")
	}
	if nested.Kind != "string" || nested.Value != "Information" {
		t.Fatalf("no aninhado = %+v", nested)
	}

	item, ok := findNode(tree, "Origens:1")
	if !ok || item.Value != "http://localhost:5173" {
		t.Fatalf("item de lista = %+v", item)
	}
	if porta, _ := findNode(tree, "Porta"); porta.Kind != "number" {
		t.Fatalf("Porta deveria ser numero, veio %q", porta.Kind)
	}
	if flag, _ := findNode(tree, "Detalhado"); flag.Kind != "boolean" {
		t.Fatalf("Detalhado deveria ser bool, veio %q", flag.Kind)
	}
}

func TestSaveAppSettingsNodePreservaTipoEOrdem(t *testing.T) {
	app := NewApp()
	path := writeSample(t)

	if _, err := app.SaveAppSettingsNode(path, "Logging:LogLevel:Default", "Debug"); err != nil {
		t.Fatalf("gravar texto: %v", err)
	}
	if _, err := app.SaveAppSettingsNode(path, "Porta", "5005"); err != nil {
		t.Fatalf("gravar numero: %v", err)
	}
	if _, err := app.SaveAppSettingsNode(path, "Detalhado", "true"); err != nil {
		t.Fatalf("gravar bool: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, `"Porta": 5005`) {
		t.Fatalf("numero virou texto no arquivo:\n%s", content)
	}
	if !strings.Contains(content, `"Detalhado": true`) {
		t.Fatalf("bool virou texto no arquivo:\n%s", content)
	}
	if strings.Index(content, `"Logging"`) > strings.Index(content, `"AllowedHosts"`) {
		t.Fatalf("ordem original das chaves foi perdida:\n%s", content)
	}

	tree, err := app.LoadAppSettingsTree(path)
	if err != nil {
		t.Fatal(err)
	}
	if node, _ := findNode(tree, "Logging:LogLevel:Default"); node.Value != "Debug" {
		t.Fatalf("valor gravado = %q", node.Value)
	}
}

func TestSaveAppSettingsNodeRecusaSecao(t *testing.T) {
	app := NewApp()
	if _, err := app.SaveAppSettingsNode(writeSample(t), "Logging", "x"); err == nil {
		t.Fatal("gravar sobre uma secao deveria falhar")
	}
}

func TestAddAppSettingsNode(t *testing.T) {
	app := NewApp()
	path := writeSample(t)

	tree, err := app.AddAppSettingsNode(path, "ConnectionStrings", "Relatorios", "value", "Server=reports")
	if err != nil {
		t.Fatalf("adicionar chave: %v", err)
	}
	if node, ok := findNode(tree, "ConnectionStrings:Relatorios"); !ok || node.Value != "Server=reports" {
		t.Fatalf("chave nova = %+v", node)
	}

	// Caminho com ":" cria as secoes intermediarias que ainda nao existem.
	tree, err = app.AddAppSettingsNode(path, "", "Jwt:Issuer", "value", "starter")
	if err != nil {
		t.Fatalf("adicionar caminho aninhado: %v", err)
	}
	if node, ok := findNode(tree, "Jwt:Issuer"); !ok || node.Value != "starter" {
		t.Fatalf("chave aninhada = %+v", node)
	}

	if _, err := app.AddAppSettingsNode(path, "ConnectionStrings", "Relatorios", "value", "x"); err == nil {
		t.Fatal("chave duplicada deveria falhar")
	}
}

func TestDeleteAppSettingsNode(t *testing.T) {
	app := NewApp()
	path := writeSample(t)

	tree, err := app.DeleteAppSettingsNode(path, "Origens:0")
	if err != nil {
		t.Fatalf("remover item da lista: %v", err)
	}
	lista, _ := findNode(tree, "Origens")
	if len(lista.Children) != 1 || lista.Children[0].Value != "http://localhost:5173" {
		t.Fatalf("lista apos remocao = %+v", lista.Children)
	}

	tree, err = app.DeleteAppSettingsNode(path, "Logging:LogLevel")
	if err != nil {
		t.Fatalf("remover secao: %v", err)
	}
	if _, ok := findNode(tree, "Logging:LogLevel"); ok {
		t.Fatal("secao continua na arvore apos remocao")
	}

	if _, err := app.DeleteAppSettingsNode(path, "NaoExiste"); err == nil {
		t.Fatal("remover chave inexistente deveria falhar")
	}
}
