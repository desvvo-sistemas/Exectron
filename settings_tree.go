package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// SettingsNode e um no da arvore do appsettings enviada para a interface.
// Path usa o separador ":" da configuracao .NET (ex: Logging:LogLevel:Default).
type SettingsNode struct {
	Key      string         `json:"key"`
	Path     string         `json:"path"`
	Kind     string         `json:"kind"`
	Value    string         `json:"value"`
	Children []SettingsNode `json:"children"`
}

// orderedMap preserva a ordem original das chaves do arquivo.
// Sem isso cada gravacao reembaralharia o appsettings em ordem alfabetica.
type orderedMap struct {
	keys   []string
	values map[string]any
}

func newOrderedMap() *orderedMap {
	return &orderedMap{values: map[string]any{}}
}

func (m *orderedMap) Set(key string, value any) {
	if _, exists := m.values[key]; !exists {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

func (m *orderedMap) Get(key string) (any, bool) {
	value, exists := m.values[key]
	return value, exists
}

func (m *orderedMap) Delete(key string) {
	if _, exists := m.values[key]; !exists {
		return
	}
	delete(m.values, key)
	for index, current := range m.keys {
		if current == key {
			m.keys = append(m.keys[:index], m.keys[index+1:]...)
			break
		}
	}
}

func (m *orderedMap) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString("{")
	for index, key := range m.keys {
		if index > 0 {
			buffer.WriteString(",")
		}
		encodedKey, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		buffer.Write(encodedKey)
		buffer.WriteString(":")
		encodedValue, err := json.Marshal(m.values[key])
		if err != nil {
			return nil, err
		}
		buffer.Write(encodedValue)
	}
	buffer.WriteString("}")
	return buffer.Bytes(), nil
}

func decodeOrderedJSON(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return token, nil
	}

	switch delimiter {
	case '{':
		object := newOrderedMap()
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, errors.New("chave JSON invalida")
			}
			value, err := decodeOrderedJSON(decoder)
			if err != nil {
				return nil, err
			}
			object.Set(key, value)
		}
		if _, err := decoder.Token(); err != nil {
			return nil, err
		}
		return object, nil
	case '[':
		items := []any{}
		for decoder.More() {
			value, err := decodeOrderedJSON(decoder)
			if err != nil {
				return nil, err
			}
			items = append(items, value)
		}
		if _, err := decoder.Token(); err != nil {
			return nil, err
		}
		return items, nil
	}
	return nil, fmt.Errorf("JSON inesperado: %v", delimiter)
}

func readOrderedJSONFile(path string) (*orderedMap, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("selecione um appsettings")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(stripJSONComments(trimUTF8BOM(data))))
	decoder.UseNumber()
	root, err := decodeOrderedJSON(decoder)
	if err != nil {
		return nil, fmt.Errorf("nao consegui ler %s: %w", path, err)
	}
	object, ok := root.(*orderedMap)
	if !ok {
		return nil, errors.New("o arquivo selecionado nao e um objeto JSON")
	}
	return object, nil
}

func writeOrderedJSONFile(path string, root *orderedMap) error {
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// LoadAppSettingsTree devolve o appsettings inteiro em forma de arvore.
func (a *App) LoadAppSettingsTree(path string) (SettingsNode, error) {
	root, err := readOrderedJSONFile(path)
	if err != nil {
		return SettingsNode{}, err
	}
	return buildSettingsTree(path, root), nil
}

func buildSettingsTree(path string, root *orderedMap) SettingsNode {
	name := path
	if index := strings.LastIndexAny(path, `\/`); index >= 0 {
		name = path[index+1:]
	}
	node := settingsNodeFrom("", "", root)
	node.Key = name
	return node
}

func settingsNodeFrom(key string, path string, value any) SettingsNode {
	node := SettingsNode{Key: key, Path: path, Children: []SettingsNode{}}
	switch typed := value.(type) {
	case *orderedMap:
		node.Kind = "object"
		node.Value = fmt.Sprintf("%d chaves", len(typed.keys))
		for _, childKey := range typed.keys {
			child, _ := typed.Get(childKey)
			node.Children = append(node.Children, settingsNodeFrom(childKey, joinSettingsPath(path, childKey), child))
		}
	case []any:
		node.Kind = "array"
		node.Value = fmt.Sprintf("%d itens", len(typed))
		for index, item := range typed {
			childKey := strconv.Itoa(index)
			node.Children = append(node.Children, settingsNodeFrom(childKey, joinSettingsPath(path, childKey), item))
		}
	case json.Number:
		node.Kind = "number"
		node.Value = typed.String()
	case bool:
		node.Kind = "boolean"
		node.Value = strconv.FormatBool(typed)
	case nil:
		node.Kind = "null"
		node.Value = ""
	default:
		node.Kind = "string"
		node.Value = fmt.Sprint(typed)
	}
	return node
}

func joinSettingsPath(parent string, key string) string {
	if parent == "" {
		return key
	}
	return parent + ":" + key
}

func splitSettingsPath(path string) []string {
	segments := []string{}
	for _, segment := range strings.Split(path, ":") {
		segment = strings.TrimSpace(segment)
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	return segments
}

// applySettingsAt caminha ate o container que guarda o ultimo segmento e
// entrega esse container para mutate. Devolver o container permite que
// remocoes em arrays substituam o slice no pai.
func applySettingsAt(node any, segments []string, createMissing bool, mutate func(container any, key string) (any, error)) (any, error) {
	if len(segments) == 0 {
		return nil, errors.New("informe a chave")
	}
	if len(segments) == 1 {
		return mutate(node, segments[0])
	}

	head := segments[0]
	switch typed := node.(type) {
	case *orderedMap:
		child, exists := typed.Get(head)
		if !exists {
			if !createMissing {
				return nil, fmt.Errorf("secao nao encontrada: %s", head)
			}
			child = newOrderedMap()
			typed.Set(head, child)
		}
		updated, err := applySettingsAt(child, segments[1:], createMissing, mutate)
		if err != nil {
			return nil, err
		}
		typed.Set(head, updated)
		return typed, nil
	case []any:
		index, err := strconv.Atoi(head)
		if err != nil || index < 0 || index >= len(typed) {
			return nil, fmt.Errorf("indice invalido: %s", head)
		}
		updated, err := applySettingsAt(typed[index], segments[1:], createMissing, mutate)
		if err != nil {
			return nil, err
		}
		typed[index] = updated
		return typed, nil
	default:
		return nil, fmt.Errorf("nao e possivel navegar dentro de %s", head)
	}
}

func setSettingsChild(container any, key string, value any) (any, error) {
	switch typed := container.(type) {
	case *orderedMap:
		typed.Set(key, value)
		return typed, nil
	case []any:
		index, err := strconv.Atoi(key)
		if err != nil || index < 0 || index >= len(typed) {
			return nil, fmt.Errorf("indice invalido: %s", key)
		}
		typed[index] = value
		return typed, nil
	default:
		return nil, fmt.Errorf("nao e possivel gravar em %s", key)
	}
}

// SaveAppSettingsNode grava um novo valor na folha indicada por nodePath.
// O tipo original e preservado sempre que o texto informado for compativel.
func (a *App) SaveAppSettingsNode(path string, nodePath string, value string) (SettingsNode, error) {
	root, err := readOrderedJSONFile(path)
	if err != nil {
		return SettingsNode{}, err
	}
	segments := splitSettingsPath(nodePath)
	if len(segments) == 0 {
		return SettingsNode{}, errors.New("selecione uma chave na arvore")
	}

	if _, err := applySettingsAt(root, segments, false, func(container any, key string) (any, error) {
		current, err := settingsChild(container, key)
		if err != nil {
			return nil, err
		}
		if _, isObject := current.(*orderedMap); isObject {
			return nil, errors.New("essa chave e uma secao; edite as chaves de dentro dela")
		}
		if _, isArray := current.([]any); isArray {
			return nil, errors.New("essa chave e uma lista; edite os itens de dentro dela")
		}
		return setSettingsChild(container, key, coerceLikeCurrent(current, value))
	}); err != nil {
		return SettingsNode{}, err
	}

	if err := writeOrderedJSONFile(path, root); err != nil {
		return SettingsNode{}, err
	}
	a.emitProgress("appsettings", "chave "+nodePath+" atualizada")
	return buildSettingsTree(path, root), nil
}

// AddAppSettingsNode cria uma chave nova dentro da secao informada.
// kind aceita value, object ou array; parentPath vazio grava na raiz.
func (a *App) AddAppSettingsNode(path string, parentPath string, key string, kind string, value string) (SettingsNode, error) {
	root, err := readOrderedJSONFile(path)
	if err != nil {
		return SettingsNode{}, err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return SettingsNode{}, errors.New("informe o nome da chave")
	}

	var newValue any
	switch kind {
	case "object":
		newValue = newOrderedMap()
	case "array":
		newValue = []any{}
	default:
		newValue = coerceSettingsValue(value)
	}

	// A chave nova entra como ultimo segmento, entao o caminho completo
	// leva applySettingsAt direto ate o container correto.
	segments := append(splitSettingsPath(parentPath), splitSettingsPath(key)...)
	if len(segments) == 0 {
		return SettingsNode{}, errors.New("informe o nome da chave")
	}
	if _, err := applySettingsAt(root, segments, true, func(container any, last string) (any, error) {
		if object, ok := container.(*orderedMap); ok {
			if _, exists := object.Get(last); exists {
				return nil, fmt.Errorf("a chave %s ja existe nessa secao", last)
			}
			object.Set(last, newValue)
			return object, nil
		}
		if items, ok := container.([]any); ok {
			return append(items, newValue), nil
		}
		return nil, errors.New("selecione uma secao para receber a chave")
	}); err != nil {
		return SettingsNode{}, err
	}

	if err := writeOrderedJSONFile(path, root); err != nil {
		return SettingsNode{}, err
	}
	a.emitProgress("appsettings", "chave "+joinSettingsPath(parentPath, key)+" criada")
	return buildSettingsTree(path, root), nil
}

// DeleteAppSettingsNode remove uma chave ou uma secao inteira da arvore.
func (a *App) DeleteAppSettingsNode(path string, nodePath string) (SettingsNode, error) {
	root, err := readOrderedJSONFile(path)
	if err != nil {
		return SettingsNode{}, err
	}
	segments := splitSettingsPath(nodePath)
	if len(segments) == 0 {
		return SettingsNode{}, errors.New("selecione uma chave na arvore")
	}

	if _, err := applySettingsAt(root, segments, false, func(container any, key string) (any, error) {
		switch typed := container.(type) {
		case *orderedMap:
			if _, exists := typed.Get(key); !exists {
				return nil, fmt.Errorf("chave nao encontrada: %s", key)
			}
			typed.Delete(key)
			return typed, nil
		case []any:
			index, err := strconv.Atoi(key)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, fmt.Errorf("indice invalido: %s", key)
			}
			return append(typed[:index], typed[index+1:]...), nil
		default:
			return nil, fmt.Errorf("nao e possivel remover %s", key)
		}
	}); err != nil {
		return SettingsNode{}, err
	}

	if err := writeOrderedJSONFile(path, root); err != nil {
		return SettingsNode{}, err
	}
	a.emitProgress("appsettings", "chave "+nodePath+" removida")
	return buildSettingsTree(path, root), nil
}

func settingsChild(container any, key string) (any, error) {
	switch typed := container.(type) {
	case *orderedMap:
		value, exists := typed.Get(key)
		if !exists {
			return nil, fmt.Errorf("chave nao encontrada: %s", key)
		}
		return value, nil
	case []any:
		index, err := strconv.Atoi(key)
		if err != nil || index < 0 || index >= len(typed) {
			return nil, fmt.Errorf("indice invalido: %s", key)
		}
		return typed[index], nil
	default:
		return nil, fmt.Errorf("chave nao encontrada: %s", key)
	}
}

// coerceLikeCurrent mantem o tipo que ja estava no arquivo quando o texto
// novo continua valido para ele, e so entao cai na deteccao automatica.
func coerceLikeCurrent(current any, value string) any {
	trimmed := strings.TrimSpace(value)
	switch current.(type) {
	case bool:
		if parsed, err := strconv.ParseBool(trimmed); err == nil {
			return parsed
		}
	case json.Number:
		if _, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return json.Number(trimmed)
		}
	case string:
		return value
	}
	return coerceSettingsValue(value)
}

func coerceSettingsValue(value string) any {
	trimmed := strings.TrimSpace(value)
	switch {
	case trimmed == "":
		return value
	case strings.EqualFold(trimmed, "true"):
		return true
	case strings.EqualFold(trimmed, "false"):
		return false
	case strings.EqualFold(trimmed, "null"):
		return nil
	}
	if _, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return json.Number(trimmed)
	}
	return value
}
