package main

import (
	"encoding/json"
	"testing"
)

// Um slice nil vira `null` no JSON, e o frontend chama `.length` direto no
// retorno destes metodos. Numa base recem criada, sem nenhum perfil salvo,
// isso derrubava a tela inicial inteira com "Cannot read properties of null".
func TestListConfigsVazioSerializaComoArray(t *testing.T) {
	app := NewApp()

	configs, err := app.ListConfigs()
	if err != nil {
		t.Fatalf("ListConfigs devolveu erro: %v", err)
	}
	if configs == nil {
		t.Fatal("ListConfigs devolveu nil; o frontend espera um slice vazio")
	}

	data, err := json.Marshal(configs)
	if err != nil {
		t.Fatalf("nao consegui serializar: %v", err)
	}
	if got := string(data); got != "[]" {
		t.Fatalf("JSON = %s, esperado []", got)
	}
}

func TestJSONSliceNuncaDevolveNil(t *testing.T) {
	if got := jsonSlice[string](nil); got == nil {
		t.Fatal("jsonSlice(nil) devolveu nil")
	}

	original := []string{"a", "b"}
	got := jsonSlice(original)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("jsonSlice alterou o conteudo: %v", got)
	}
}

// GetNodeInfo alimenta a tela de configuracoes, que percorre as duas listas.
func TestGetNodeInfoTemListasNaoNulas(t *testing.T) {
	info := NewApp().GetNodeInfo()
	if info.InstalledVersions == nil {
		t.Error("InstalledVersions e nil")
	}
	if info.AvailableVersions == nil {
		t.Error("AvailableVersions e nil")
	}
}
