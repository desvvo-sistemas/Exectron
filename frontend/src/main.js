import './style.css';
import logoMark from './assets/images/logo-mark.png';
import {
  AddAppSettingsNode,
  ChooseAppSettingsFile,
  ChooseComposeFile,
  ChooseDirectory,
  ChooseDockerfile,
  ChooseDotnetProject,
  DeleteAppSettingsNode,
  DeleteConfig,
  DeleteNodeVersion,
  DockerContainerLogs,
  FindAppSettings,
  FindDockerFiles,
  FindDotnetProjects,
  GetCommandPresets,
  GetDockerInfo,
  GetNodeInfo,
  GetStatuses,
  HideToTray,
  InstallNodeVersion,
  ListComposeServices,
  ListConfigs,
  ListDockerContainers,
  ListDockerImages,
  LoadAppSettingsTree,
  OpenSettings,
  PreviewDockerCommand,
  PullDockerImage,
  RefreshNodeVersionList,
  RemoveDockerContainer,
  RemoveDockerImage,
  RestartDockerContainer,
  SaveAppSettingsNode,
  SaveConfig,
  Start,
  StartDockerContainer,
  Stop,
  StopAll,
  StopDockerContainer,
  UseNodeVersion,
} from '../wailsjs/go/main/App';
import { BrowserOpenURL, EventsOn, Quit } from '../wailsjs/runtime/runtime';

const RUNTIME_LABELS = {
  node: 'Node',
  dotnet: '.NET',
  go: 'Go',
  python: 'Python',
  docker: 'Docker',
  custom: 'Custom',
};

const KIND_LABELS = {
  object: 'secao',
  array: 'lista',
  string: 'texto',
  number: 'numero',
  boolean: 'bool',
  null: 'null',
};

const state = {
  view: 'runner',
  projectTab: 'project',
  configs: [],
  presets: [],
  selectedId: '',
  node: null,
  statuses: [],
  runningKey: '',
  progress: [],
  solutions: [],
  appSettingsFiles: [],
  activeConsoleId: '',
  settings: {
    tree: null,
    file: '',
    expanded: new Set(),
    editing: '',
    addParent: '',
    pendingDelete: '',
    filter: '',
  },
  docker: {
    info: null,
    tab: 'containers',
    containers: [],
    images: [],
    showAll: true,
    selected: '',
    logs: '',
    pendingDelete: '',
    composeFiles: [],
    dockerfiles: [],
    services: [],
    preview: '',
  },
};

document.querySelector('#app').innerHTML = `
  <main class="shell">
    <aside class="sidebar">
      <div class="brand">
        <img class="brand-mark" src="${logoMark}" alt="Exectron" />
        <div>
          <strong>Exectron</strong>
          <span>Desktop runner</span>
        </div>
      </div>

      <div class="tabs tabs-nav" role="tablist">
        <button class="tab-item active" role="tab" data-view="runner"><span class="tab-icon">&#9654;</span>Runner</button>
        <button class="tab-item" role="tab" data-view="docker"><span class="tab-icon">&#9673;</span>Docker<span class="tab-badge hidden" id="dockerNavBadge">0</span></button>
        <button class="tab-item" role="tab" data-view="settings"><span class="tab-icon">&#9881;</span>Config</button>
      </div>

      <button class="secondary full" id="newProfile">Novo perfil</button>
      <div class="profile-list" id="profileList"></div>
    </aside>

    <section class="workspace">
      <header class="topbar">
        <div class="topbar-title">
          <h1 id="viewTitle">Runner local</h1>
          <p id="viewSubtitle">Configure projetos, rode varios processos e acompanhe os consoles por abas.</p>
        </div>
        <div class="top-actions">
          <div class="status-pill" id="statusPill">Parado</div>
          <button class="secondary" id="hideToTray">Segundo plano</button>
          <div class="dropdown">
            <button class="icon-button" id="optionsToggle" title="Opcoes">...</button>
            <div class="dropdown-menu" id="optionsMenu">
              <button id="openSettings">Configuracoes</button>
              <button id="stopAll">Parar todos</button>
              <button id="quitApp">Sair</button>
            </div>
          </div>
        </div>
      </header>
      <div class="notice" id="notice"></div>

      <section class="view runner-view active" id="runnerView">
        <section class="panel config-panel">
          <div class="tabs tabs-panel" id="projectTabs" role="tablist">
            <button class="tab-item active" role="tab" data-project-tab="project">Projeto</button>
            <button class="tab-item hidden" role="tab" data-project-tab="docker" id="dockerProjectTab">Docker</button>
            <button class="tab-item" role="tab" data-project-tab="appsettings">AppSettings<span class="tab-badge hidden" id="appsettingsBadge">0</span></button>
          </div>

          <div class="project-pane active" id="projectPane">
            <div class="pane-scroll">
              <div class="form-grid">
                <label>
                  Nome
                  <input id="name" placeholder="Minha API" />
                </label>
                <label>
                  Tipo
                  <select id="runtime">
                    <option value="node">Node</option>
                    <option value="dotnet">.NET</option>
                    <option value="docker">Docker</option>
                    <option value="go">Go</option>
                    <option value="python">Python</option>
                    <option value="custom">Custom</option>
                  </select>
                </label>
                <label class="wide">
                  Caminho
                  <div class="path-row">
                    <input id="path" placeholder="C:\\\\meu-projeto" />
                    <button class="icon-button" id="pickPath" title="Selecionar pasta">...</button>
                  </div>
                </label>
                <label class="command-config">
                  Preset
                  <select id="preset"></select>
                </label>
                <label class="command-config">
                  Comando
                  <input id="command" placeholder="npm run dev" />
                </label>
                <label class="node-config">
                  Versao Node
                  <select id="profileNodeVersion"></select>
                </label>
                <label class="dotnet-config wide">
                  Projeto .NET (.csproj)
                  <div class="path-row">
                    <select id="projectFile"></select>
                    <button class="secondary" id="pickProjectFile">Escolher</button>
                    <button class="secondary" id="scanProjects">Pesquisar</button>
                  </div>
                </label>
                <label class="wide">
                  AppSettings
                  <div class="path-row">
                    <select id="appSettings"></select>
                    <button class="secondary" id="pickAppSettings">Escolher</button>
                    <button class="secondary" id="scanAppSettings">Buscar</button>
                  </div>
                </label>
              </div>
            </div>
            <div class="actions">
              <button class="primary" id="save">Salvar</button>
              <button class="danger ghost" id="delete">Excluir</button>
            </div>
          </div>

          <div class="project-pane" id="dockerPane">
            <div class="pane-scroll">
              <div class="form-grid">
                <div class="wide">
                  <span class="field-label">Modo do container</span>
                  <div class="tabs tabs-inline" id="dockerModeTabs" role="tablist">
                    <button class="tab-item active" role="tab" data-docker-mode="compose">docker compose</button>
                    <button class="tab-item" role="tab" data-docker-mode="dockerfile">Dockerfile</button>
                    <button class="tab-item" role="tab" data-docker-mode="image">Imagem pronta</button>
                  </div>
                  <select id="dockerMode" class="hidden">
                    <option value="compose">compose</option>
                    <option value="dockerfile">dockerfile</option>
                    <option value="image">image</option>
                  </select>
                </div>

                <label class="mode-compose wide">
                  Arquivo compose
                  <div class="path-row">
                    <select id="dockerCompose"></select>
                    <button class="secondary" id="pickCompose">Escolher</button>
                    <button class="secondary" id="scanCompose">Buscar</button>
                  </div>
                </label>
                <label class="mode-compose">
                  Servico (vazio = todos)
                  <div class="path-row">
                    <select id="dockerService"></select>
                    <button class="secondary" id="scanServices">Ler</button>
                  </div>
                </label>
                <label class="mode-compose">
                  Nome do projeto compose
                  <input id="dockerProject" placeholder="minha-stack" />
                </label>

                <label class="mode-dockerfile wide">
                  Dockerfile
                  <div class="path-row">
                    <select id="dockerfile"></select>
                    <button class="secondary" id="pickDockerfile">Escolher</button>
                    <button class="secondary" id="scanDockerfile">Buscar</button>
                  </div>
                </label>
                <label class="mode-dockerfile wide">
                  Contexto do build (vazio = caminho do projeto)
                  <input id="dockerContext" placeholder="C:\\\\meu-projeto" />
                </label>

                <label class="mode-image mode-dockerfile wide">
                  Imagem
                  <input id="dockerImage" placeholder="postgres:16 ou minha-api:local" />
                </label>
                <label class="mode-image mode-dockerfile">
                  Nome do container
                  <input id="dockerContainer" placeholder="minha-api" />
                </label>
                <label class="mode-image mode-dockerfile">
                  Portas (host:container)
                  <input id="dockerPorts" placeholder="8080:80, 5432:5432" />
                </label>
                <label class="mode-image mode-dockerfile wide">
                  Variaveis de ambiente
                  <input id="dockerEnv" placeholder="POSTGRES_PASSWORD=123, TZ=America/Sao_Paulo" />
                </label>
                <label class="mode-image mode-dockerfile wide">
                  Volumes
                  <input id="dockerVolumes" placeholder="dados:/var/lib/postgresql/data" />
                </label>
                <label class="mode-image mode-dockerfile wide">
                  Comando dentro do container (opcional)
                  <input id="dockerCommand" placeholder="npm run start" />
                </label>

                <label class="wide">
                  Argumentos extras
                  <input id="dockerArgs" placeholder="--pull always" />
                </label>

                <div class="wide check-row">
                  <label class="check mode-compose"><input type="checkbox" id="dockerBuild" /> Rebuild das imagens (--build)</label>
                  <label class="check mode-compose"><input type="checkbox" id="dockerRecreate" /> Recriar containers (--force-recreate)</label>
                  <label class="check mode-image mode-dockerfile"><input type="checkbox" id="dockerRemove" /> Remover container ao parar (--rm)</label>
                </div>

                <div class="wide command-preview">
                  <span class="field-label">Comando gerado</span>
                  <code id="dockerPreview">Preencha o perfil para ver o comando.</code>
                </div>
              </div>
            </div>
            <div class="actions">
              <button class="primary" id="saveDocker">Salvar</button>
              <button class="secondary" id="refreshPreview">Atualizar comando</button>
            </div>
          </div>

          <div class="project-pane" id="appsettingsPane">
            <div class="tree-toolbar">
              <strong id="appsettingsPath" title="">Nenhum appsettings selecionado</strong>
              <input id="treeFilter" class="tree-filter" placeholder="Filtrar chave ou valor" />
              <div class="tree-toolbar-actions">
                <button class="icon-button" id="expandAll" title="Expandir tudo">&#9660;</button>
                <button class="icon-button" id="collapseAll" title="Recolher tudo">&#9650;</button>
                <button class="secondary" id="reloadAppSettings">Recarregar</button>
              </div>
            </div>
            <div class="settings-tree" id="settingsTree"></div>
            <div class="tree-add">
              <div class="tree-add-target">
                Nova chave em <strong id="addTarget">raiz</strong>
                <button class="link-button hidden" id="clearAddTarget">usar a raiz</button>
              </div>
              <div class="tree-add-form">
                <input id="newKey" placeholder="Chave. Ex: ConnectionStrings:Default" />
                <select id="newKind">
                  <option value="value">valor</option>
                  <option value="object">secao</option>
                  <option value="array">lista</option>
                </select>
                <input id="newValue" placeholder="Valor" />
                <button class="primary" id="addSetting">Adicionar</button>
              </div>
            </div>
          </div>
        </section>

        <section class="panel control-panel">
          <div class="panel-head">
            <h2>Execucao</h2>
            <div class="run-actions">
              <button class="start" id="start">Start</button>
              <button class="stop" id="stop">Stop</button>
            </div>
          </div>
          <div class="runner-hint" id="runnerHint"></div>
          <div class="process-progress" id="processProgressLog"></div>
          <div class="tabs tabs-console" id="consoleTabs" role="tablist"></div>
          <pre id="output"></pre>
        </section>
      </section>

      <section class="view docker-view" id="dockerView">
        <section class="panel docker-panel">
          <div class="docker-head">
            <div class="docker-status" id="dockerStatus">Consultando Docker...</div>
            <div class="docker-head-actions">
              <label class="check"><input type="checkbox" id="dockerShowAll" checked /> Incluir parados</label>
              <button class="secondary" id="refreshDocker">Atualizar</button>
            </div>
          </div>

          <div class="tabs tabs-panel" id="dockerTabs" role="tablist">
            <button class="tab-item active" role="tab" data-docker-tab="containers">Containers<span class="tab-badge" id="containersBadge">0</span></button>
            <button class="tab-item" role="tab" data-docker-tab="images">Imagens<span class="tab-badge" id="imagesBadge">0</span></button>
          </div>

          <div class="docker-pane active" id="containersPane">
            <div class="docker-list" id="containerList"></div>
          </div>

          <div class="docker-pane" id="imagesPane">
            <div class="docker-pull">
              <input id="pullImage" placeholder="postgres:16" />
              <button class="primary" id="pullImageButton">Baixar imagem</button>
            </div>
            <div class="docker-list" id="imageList"></div>
          </div>

          <div class="docker-logs">
            <div class="docker-logs-head">
              <strong id="logsTitle">Logs do container</strong>
              <button class="secondary" id="refreshLogs">Recarregar logs</button>
            </div>
            <pre id="dockerLogs">Selecione um container para ver os logs.</pre>
          </div>
        </section>
      </section>

      <section class="view settings-view" id="settingsView">
        <section class="panel settings-panel">
          <h2>Gerenciador Node</h2>
          <div class="settings-grid">
            <div class="node-status" id="nodeStatus">Carregando...</div>
            <div class="node-controls">
              <label>
                Versao Node
                <select id="nodeVersion"></select>
              </label>
              <div class="node-action-grid">
                <button class="secondary" id="selectNode">Selecionar</button>
                <button class="primary" id="activateNode">Trocar versao</button>
                <button class="secondary" id="installSelectedNode">Baixar selecionada</button>
                <button class="danger ghost" id="deleteNode">Excluir</button>
              </div>
              <div class="node-install">
                <input id="installVersion" placeholder="20.17.0" />
                <button class="secondary" id="installNode">Instalar manual</button>
              </div>
              <button class="secondary full" id="refreshNode">Buscar e salvar lista</button>
            </div>
          </div>
          <div class="progress-log" id="progressLog"></div>
        </section>
      </section>
    </section>

    <div class="modal-backdrop" id="modal">
      <div class="modal-card">
        <div class="spinner"></div>
        <h2 id="modalTitle">Processando</h2>
        <p id="modalMessage">Aguarde...</p>
        <div class="progress-log modal-log" id="modalLog"></div>
      </div>
    </div>
  </main>
`;

const el = (id) => document.getElementById(id);

async function boot() {
  // Um slice vazio do Go pode chegar como null: normalizamos na entrada para
  // nenhuma tela precisar se defender disso depois.
  state.presets = (await GetCommandPresets()) || [];
  state.configs = (await ListConfigs()) || [];
  state.node = (await GetNodeInfo()) || null;
  renderPresets();
  renderNode();
  renderProgress();
  renderProfiles();
  renderSettingsTree();
  if (state.configs.length) selectConfig(state.configs[0].id);
  switchView('runner');
  await refreshStatus();
  setInterval(refreshStatus, 1800);
  setInterval(autoRefreshDocker, 6000);
}

/* --------------------------------------------------------------- navegacao */

function switchView(view) {
  state.view = view;
  document.querySelectorAll('.view').forEach((item) => item.classList.toggle('active', item.id === `${view}View`));
  document.querySelectorAll('.tabs-nav .tab-item').forEach((item) => {
    const active = item.dataset.view === view;
    item.classList.toggle('active', active);
    item.setAttribute('aria-selected', String(active));
  });

  const titles = {
    runner: ['Runner local', 'Configure projetos, rode varios processos e acompanhe os consoles por abas.'],
    docker: ['Docker', 'Suba stacks, acompanhe containers e imagens sem sair do app.'],
    settings: ['Configuracoes', 'Gerencie a versao Node usada pelos projetos iniciados pelo app.'],
  };
  const [title, subtitle] = titles[view] || titles.runner;
  el('viewTitle').textContent = title;
  el('viewSubtitle').textContent = subtitle;

  if (view === 'docker' && !state.docker.info) {
    runAction(loadDocker, 'Docker carregado.');
  }
}

function switchProjectTab(tab) {
  state.projectTab = tab;
  document.querySelectorAll('#projectTabs .tab-item').forEach((item) => {
    const active = item.dataset.projectTab === tab;
    item.classList.toggle('active', active);
    item.setAttribute('aria-selected', String(active));
  });
  document.querySelectorAll('.project-pane').forEach((item) => item.classList.toggle('active', item.id === `${tab}Pane`));
}

/* ------------------------------------------------------------------ perfis */

function currentForm() {
  return {
    id: state.selectedId,
    name: el('name').value.trim(),
    path: el('path').value.trim(),
    runtime: el('runtime').value,
    command: el('command').value.trim(),
    nodeVersion: el('profileNodeVersion').value,
    solution: '',
    projectFile: el('projectFile').value,
    appSettings: el('appSettings').value,
    docker: currentDockerForm(),
  };
}

function currentDockerForm() {
  return {
    mode: el('dockerMode').value || 'compose',
    composeFile: el('dockerCompose').value,
    service: el('dockerService').value,
    projectName: el('dockerProject').value.trim(),
    dockerfile: el('dockerfile').value,
    context: el('dockerContext').value.trim(),
    image: el('dockerImage').value.trim(),
    containerName: el('dockerContainer').value.trim(),
    ports: el('dockerPorts').value.trim(),
    envVars: el('dockerEnv').value.trim(),
    volumes: el('dockerVolumes').value.trim(),
    extraArgs: el('dockerArgs').value.trim(),
    command: el('dockerCommand').value.trim(),
    build: el('dockerBuild').checked,
    recreate: el('dockerRecreate').checked,
    removeOnStop: el('dockerRemove').checked,
  };
}

function renderPresets() {
  el('preset').innerHTML = [
    '<option value="">Comando personalizado</option>',
    ...state.presets.map((preset) => `<option value="${escapeHtml(preset.command)}" data-runtime="${escapeHtml(preset.runtime)}">${escapeHtml(preset.label)}</option>`),
  ].join('');
}

function renderProfiles() {
  const running = new Set(state.statuses.map((status) => status.projectId));
  el('profileList').innerHTML = state.configs.length ? state.configs.map((config) => `
    <button class="profile ${config.id === state.selectedId ? 'active' : ''}" data-id="${escapeHtml(config.id)}">
      <span class="profile-head">
        <strong>${escapeHtml(config.name || 'Sem nome')}</strong>
        <span class="dot ${running.has(config.id) ? 'on' : ''}"></span>
      </span>
      <span class="profile-meta">
        <em class="chip chip-${escapeHtml(config.runtime || 'custom')}">${escapeHtml(RUNTIME_LABELS[config.runtime] || 'Custom')}</em>
        <span>${escapeHtml(profileSummary(config))}</span>
      </span>
    </button>
  `).join('') : '<div class="empty dark">Nenhum perfil salvo ainda.</div>';
}

function profileSummary(config) {
  if (config.runtime !== 'docker') return config.command || '-';
  const docker = config.docker || {};
  if (docker.mode === 'image') return docker.image || 'imagem nao definida';
  if (docker.mode === 'dockerfile') return baseName(docker.dockerfile) || 'Dockerfile nao definido';
  return baseName(docker.composeFile) || 'compose nao definido';
}

function selectConfig(id) {
  const config = state.configs.find((item) => item.id === id);
  state.selectedId = config?.id || '';
  el('name').value = config?.name || '';
  el('path').value = config?.path || '';
  el('runtime').value = config?.runtime || detectRuntime(config?.command || '');
  el('command').value = config?.command || '';
  el('profileNodeVersion').value = config?.nodeVersion || '';

  const projectFile = config?.projectFile || (config?.solution?.toLowerCase?.().endsWith('.csproj') ? config.solution : '');
  state.solutions = projectFile ? [projectFile] : [];
  state.appSettingsFiles = config?.appSettings ? [config.appSettings] : [];
  state.settings.tree = null;
  state.settings.file = '';
  state.settings.editing = '';
  state.settings.addParent = '';
  state.settings.pendingDelete = '';

  fillDockerForm(config?.docker || {});
  renderSolutions(projectFile);
  renderAppSettingsFiles(config?.appSettings || '');
  renderSettingsTree();
  renderRuntimeFields();
  renderProfiles();
  switchProjectTab(el('runtime').value === 'docker' ? 'docker' : 'project');
}

function fillDockerForm(docker) {
  state.docker.composeFiles = docker.composeFile ? [docker.composeFile] : [];
  state.docker.dockerfiles = docker.dockerfile ? [docker.dockerfile] : [];
  state.docker.services = docker.service ? [docker.service] : [];

  el('dockerMode').value = docker.mode || 'compose';
  renderOptions('dockerCompose', state.docker.composeFiles, docker.composeFile || '', 'Selecione o compose');
  renderOptions('dockerService', state.docker.services, docker.service || '', 'Todos os servicos');
  renderOptions('dockerfile', state.docker.dockerfiles, docker.dockerfile || '', 'Selecione o Dockerfile');
  el('dockerProject').value = docker.projectName || '';
  el('dockerContext').value = docker.context || '';
  el('dockerImage').value = docker.image || '';
  el('dockerContainer').value = docker.containerName || '';
  el('dockerPorts').value = docker.ports || '';
  el('dockerEnv').value = docker.envVars || '';
  el('dockerVolumes').value = docker.volumes || '';
  el('dockerArgs').value = docker.extraArgs || '';
  el('dockerCommand').value = docker.command || '';
  el('dockerBuild').checked = !!docker.build;
  el('dockerRecreate').checked = !!docker.recreate;
  el('dockerRemove').checked = docker.removeOnStop !== false;
  renderDockerMode();
}

function renderOptions(id, values, selected, emptyLabel) {
  el(id).innerHTML = [
    `<option value="">${escapeHtml(emptyLabel)}</option>`,
    ...values.map((value) => `<option value="${escapeHtml(value)}">${escapeHtml(value)}</option>`),
  ].join('');
  el(id).value = selected;
}

function renderDockerMode() {
  const mode = el('dockerMode').value || 'compose';
  document.querySelectorAll('#dockerModeTabs .tab-item').forEach((item) => {
    const active = item.dataset.dockerMode === mode;
    item.classList.toggle('active', active);
    item.setAttribute('aria-selected', String(active));
  });
  ['compose', 'dockerfile', 'image'].forEach((option) => {
    document.querySelectorAll(`.mode-${option}`).forEach((item) => {
      item.classList.toggle('hidden', !item.classList.contains(`mode-${mode}`));
    });
  });
}

function renderSolutions(selected = '') {
  renderOptions('projectFile', state.solutions, selected, 'Selecione o .csproj');
}

function renderAppSettingsFiles(selected = '') {
  renderOptions('appSettings', state.appSettingsFiles, selected, 'Selecione um appsettings');
  el('appsettingsPath').textContent = selected ? baseName(selected) : 'Nenhum appsettings selecionado';
  el('appsettingsPath').title = selected || '';
}

function renderRuntimeFields() {
  const runtime = el('runtime').value;
  document.querySelectorAll('.node-config').forEach((item) => item.classList.toggle('hidden', runtime !== 'node'));
  document.querySelectorAll('.dotnet-config').forEach((item) => item.classList.toggle('hidden', runtime !== 'dotnet'));
  document.querySelectorAll('.command-config').forEach((item) => item.classList.toggle('hidden', runtime === 'docker'));
  el('dockerProjectTab').classList.toggle('hidden', runtime !== 'docker');
  if (runtime !== 'docker' && state.projectTab === 'docker') switchProjectTab('project');
}

async function saveConfig() {
  const form = currentForm();
  state.configs = await SaveConfig(form);
  const saved = state.configs.find((item) => item.id === form.id)
    || state.configs.find((item) => item.name === form.name)
    || state.configs[state.configs.length - 1];
  if (saved) state.selectedId = saved.id;
  renderProfiles();
}

/* -------------------------------------------------------- arvore appsettings */

async function loadSettingsTree(file) {
  const target = file || el('appSettings').value;
  if (!target) throw new Error('selecione um appsettings');
  if (!state.appSettingsFiles.includes(target)) state.appSettingsFiles = [target, ...state.appSettingsFiles];
  state.settings.tree = await LoadAppSettingsTree(target);
  state.settings.file = target;
  state.settings.editing = '';
  state.settings.pendingDelete = '';
  if (!state.settings.expanded.size) expandTopLevel();
  renderAppSettingsFiles(target);
  renderSettingsTree();
}

function expandTopLevel() {
  (state.settings.tree?.children || []).forEach((child) => {
    if (isContainer(child)) state.settings.expanded.add(child.path);
  });
}

function isContainer(node) {
  return node.kind === 'object' || node.kind === 'array';
}

function walkTree(node, visit) {
  (node.children || []).forEach((child) => {
    visit(child);
    walkTree(child, visit);
  });
}

function nodeMatches(node, filter) {
  if (!filter) return true;
  if (`${node.key} ${node.path} ${node.value}`.toLowerCase().includes(filter)) return true;
  return (node.children || []).some((child) => nodeMatches(child, filter));
}

function renderSettingsTree() {
  const tree = state.settings.tree;
  const badge = el('appsettingsBadge');
  if (!tree) {
    el('settingsTree').innerHTML = '<div class="empty">Selecione um arquivo e clique em Recarregar para abrir a arvore.</div>';
    badge.classList.add('hidden');
    renderAddTarget();
    return;
  }

  let leaves = 0;
  walkTree(tree, (node) => {
    if (!isContainer(node)) leaves += 1;
  });
  badge.textContent = String(leaves);
  badge.classList.remove('hidden');

  const filter = state.settings.filter.trim().toLowerCase();
  const html = (tree.children || []).map((child) => renderTreeNode(child, 0, filter)).join('');
  el('settingsTree').innerHTML = html || '<div class="empty">Nenhuma chave encontrada para esse filtro.</div>';
  renderAddTarget();

  const editor = el('treeEditor');
  if (editor) {
    editor.focus();
    editor.select();
  }
}

function renderTreeNode(node, depth, filter) {
  if (filter && !nodeMatches(node, filter)) return '';
  const kind = `<em class="kind kind-${escapeHtml(node.kind)}">${escapeHtml(KIND_LABELS[node.kind] || node.kind)}</em>`;
  const pending = state.settings.pendingDelete === node.path;
  const removeLabel = pending ? 'confirmar' : '&#10005;';
  const removeClass = pending ? 'tree-action danger-action pending' : 'tree-action danger-action';

  if (isContainer(node)) {
    const expanded = Boolean(filter) || state.settings.expanded.has(node.path);
    const children = (node.children || []).map((child) => renderTreeNode(child, depth + 1, filter)).join('');
    const selected = state.settings.addParent === node.path ? ' selected' : '';
    return `
      <div class="tree-node">
        <div class="tree-row container${selected}" style="--depth:${depth}" title="${escapeHtml(node.path)}">
          <button class="tree-caret" data-toggle="${escapeHtml(node.path)}">${expanded ? '&#9662;' : '&#9656;'}</button>
          <span class="tree-key">${escapeHtml(node.key)}</span>
          ${kind}
          <span class="tree-value muted">${escapeHtml(node.value)}</span>
          <span class="tree-actions">
            <button class="tree-action" data-add="${escapeHtml(node.path)}" title="Adicionar chave nesta secao">+</button>
            <button class="${removeClass}" data-remove="${escapeHtml(node.path)}" title="Remover secao">${removeLabel}</button>
          </span>
        </div>
        <div class="tree-children ${expanded ? '' : 'hidden'}">${children}</div>
      </div>
    `;
  }

  if (state.settings.editing === node.path) {
    return `
      <div class="tree-row leaf editing" style="--depth:${depth}">
        <span class="tree-key">${escapeHtml(node.key)}</span>
        ${kind}
        <input class="tree-input" id="treeEditor" value="${escapeHtml(node.value)}" data-path="${escapeHtml(node.path)}" />
        <span class="tree-actions">
          <button class="tree-action ok" data-save="${escapeHtml(node.path)}" title="Salvar">&#10003;</button>
          <button class="tree-action" data-cancel="1" title="Cancelar">&#10005;</button>
        </span>
      </div>
    `;
  }

  return `
    <div class="tree-row leaf" style="--depth:${depth}" data-edit="${escapeHtml(node.path)}" title="${escapeHtml(node.path)}">
      <span class="tree-key">${escapeHtml(node.key)}</span>
      ${kind}
      <span class="tree-value">${escapeHtml(node.value || '(vazio)')}</span>
      <span class="tree-actions">
        <button class="${removeClass}" data-remove="${escapeHtml(node.path)}" title="Remover chave">${removeLabel}</button>
      </span>
    </div>
  `;
}

function renderAddTarget() {
  const target = state.settings.addParent;
  el('addTarget').textContent = target || 'raiz';
  el('clearAddTarget').classList.toggle('hidden', !target);
}

async function saveTreeValue(path, value) {
  state.settings.tree = await SaveAppSettingsNode(state.settings.file, path, value);
  state.settings.editing = '';
  renderSettingsTree();
}

/* ---------------------------------------------------------------- execucao */

async function refreshStatus() {
  state.statuses = (await GetStatuses()) || [];
  renderStatus();
}

function renderStatus() {
  const statuses = state.statuses || [];
  if (state.activeConsoleId && !statuses.some((status) => status.projectId === state.activeConsoleId)) {
    state.activeConsoleId = statuses[0]?.projectId || '';
  }
  if (!state.activeConsoleId && statuses.length) state.activeConsoleId = statuses[0].projectId;

  el('statusPill').textContent = statuses.length ? `${statuses.length} rodando` : 'Parado';
  el('statusPill').classList.toggle('running', statuses.length > 0);

  el('consoleTabs').innerHTML = statuses.length ? statuses.map((status) => `
    <button class="tab-item console ${status.projectId === state.activeConsoleId ? 'active' : ''}" role="tab" data-id="${escapeHtml(status.projectId)}">
      <span class="dot on"></span>
      <span class="console-name">${escapeHtml(status.projectName || status.projectId)}</span>
      ${status.port ? `<span class="tab-badge">:${status.port}</span>` : ''}
      <span class="tab-close" data-close="${escapeHtml(status.projectId)}" title="Parar processo">&#10005;</span>
    </button>
  `).join('') : '<div class="tabs-empty">Nenhum processo ativo. Salve um perfil e clique em Start.</div>';

  const active = statuses.find((status) => status.projectId === state.activeConsoleId);
  const text = active ? [
    `Status: ${active.message || '-'}`,
    `Porta: ${active.port || '-'}`,
    `URL: ${active.url || '-'}`,
    `Docs: ${active.docsActive ? active.docsUrl : 'nao detectada'}`,
    '',
    active.output || 'Sem saida ainda.',
  ].join('\n') : 'Sem processo selecionado.';

  // O status se atualiza sozinho a cada segundo e meio: so mexe no console
  // quando o texto mudou, e mantem a rolagem de quem subiu para ler o log.
  const output = el('output');
  if (output.textContent !== text) {
    const atBottom = output.scrollHeight - output.scrollTop - output.clientHeight < 40;
    output.textContent = text;
    if (atBottom) output.scrollTop = output.scrollHeight;
  }

  // Pela mesma razao a lista de perfis so e redesenhada quando o
  // conjunto de processos ativos muda, para nao perder a rolagem lateral.
  const runningKey = statuses.map((status) => status.projectId).sort().join('|');
  if (runningKey !== state.runningKey) {
    state.runningKey = runningKey;
    renderProfiles();
  }
  renderRunnerHint();
}

function renderRunnerHint() {
  const form = currentForm();
  const running = state.statuses.some((status) => status.projectId === form.id);
  let message = 'Pronto para iniciar.';
  if (form.runtime === 'docker') {
    const docker = form.docker;
    if (docker.mode === 'compose' && !docker.composeFile) message = 'Docker: selecione o arquivo compose na aba Docker.';
    else if (docker.mode === 'dockerfile' && !docker.dockerfile) message = 'Docker: selecione o Dockerfile na aba Docker.';
    else if (docker.mode === 'image' && !docker.image) message = 'Docker: informe a imagem na aba Docker.';
    else if (running) message = 'Stack em execucao. Stop derruba os containers deste perfil.';
    else message = 'Pronto para subir o container.';
  } else if (!form.id) message = 'Novo perfil: salve antes de iniciar para manter historico e console.';
  else if (!form.path) message = 'Selecione o caminho do projeto.';
  else if (!form.command) message = 'Informe o comando de inicializacao.';
  else if (form.runtime === 'dotnet' && !form.projectFile) message = 'Projeto .NET: pesquise e selecione o arquivo .csproj que deve iniciar.';
  else if (running) message = 'Este projeto ja esta rodando. Use Stop para encerrar a aba ativa.';
  el('runnerHint').textContent = message;
}

/* ------------------------------------------------------------------ docker */

async function loadDocker() {
  state.docker.info = await GetDockerInfo();
  renderDockerStatus();
  if (state.docker.info.engineRunning) {
    await refreshDockerLists();
    return;
  }
  renderContainers();
  renderImages();
}

async function refreshDockerLists() {
  const [containers, images] = await Promise.all([
    ListDockerContainers(state.docker.showAll),
    ListDockerImages(),
  ]);
  state.docker.containers = containers || [];
  state.docker.images = images || [];
  renderContainers();
  renderImages();
}

async function autoRefreshDocker() {
  if (state.view !== 'docker' || !state.docker.info?.engineRunning) return;
  try {
    state.docker.containers = (await ListDockerContainers(state.docker.showAll)) || [];
    renderContainers();
  } catch (error) {
    // atualizacao de fundo nao deve interromper o usuario
  }
}

function renderDockerStatus() {
  const info = state.docker.info || {};
  const badge = el('dockerNavBadge');
  badge.textContent = String(info.runningContainers || 0);
  badge.classList.toggle('hidden', !info.runningContainers);

  const engine = info.engineRunning ? 'ativo' : (info.available ? 'parado' : 'ausente');
  const compose = info.composeAvailable ? `${info.composeCommand} ${info.composeVersion || ''}`.trim() : 'nao encontrado';
  el('dockerStatus').innerHTML = `
    <div class="docker-stat"><span>Engine</span><strong class="state state-${escapeHtml(engine)}">${escapeHtml(engine)}</strong></div>
    <div class="docker-stat"><span>Versao</span><strong>${escapeHtml(info.version || '-')}</strong></div>
    <div class="docker-stat"><span>Compose</span><strong>${escapeHtml(compose)}</strong></div>
    <div class="docker-stat"><span>Containers</span><strong>${info.runningContainers || 0} de ${info.containers || 0} ativos</strong></div>
    <div class="docker-stat"><span>Imagens</span><strong>${info.images || 0}</strong></div>
    ${info.message ? `<small>${escapeHtml(info.message)}</small>` : ''}
  `;
}

function renderContainers() {
  const containers = state.docker.containers || [];
  el('containersBadge').textContent = String(containers.length);
  if (!containers.length) {
    const message = state.docker.info?.engineRunning
      ? 'Nenhum container encontrado.'
      : 'Docker parado. Inicie o Docker Desktop e clique em Atualizar.';
    el('containerList').innerHTML = `<div class="empty">${message}</div>`;
    return;
  }

  el('containerList').innerHTML = containers.map((container) => {
    const pending = state.docker.pendingDelete === container.id;
    const controls = container.running
      ? `<button class="secondary" data-stop="${escapeHtml(container.id)}">Parar</button>
         <button class="secondary" data-restart="${escapeHtml(container.id)}">Reiniciar</button>`
      : `<button class="start" data-start="${escapeHtml(container.id)}">Iniciar</button>`;
    return `
      <div class="docker-row ${container.id === state.docker.selected ? 'selected' : ''}" data-container="${escapeHtml(container.id)}">
        <div class="docker-row-main">
          <span class="dot ${container.running ? 'on' : ''}"></span>
          <div class="docker-row-text">
            <strong>${escapeHtml(container.name || container.id)}</strong>
            <span>${escapeHtml(container.image)}${container.compose ? ` &middot; compose: ${escapeHtml(container.compose)}` : ''}</span>
          </div>
        </div>
        <div class="docker-row-meta">
          <span class="state state-${container.running ? 'ativo' : 'parado'}">${escapeHtml(container.status || container.state)}</span>
          <span class="ports">${escapeHtml(container.ports || 'sem portas publicadas')}</span>
        </div>
        <div class="docker-row-actions">
          ${container.url ? `<button class="secondary" data-open="${escapeHtml(container.url)}">Abrir</button>` : ''}
          ${controls}
          <button class="secondary" data-logs="${escapeHtml(container.id)}">Logs</button>
          <button class="danger ghost ${pending ? 'pending' : ''}" data-remove-container="${escapeHtml(container.id)}">${pending ? 'Confirmar' : 'Remover'}</button>
        </div>
      </div>
    `;
  }).join('');
}

function renderImages() {
  const images = state.docker.images || [];
  el('imagesBadge').textContent = String(images.length);
  if (!images.length) {
    el('imageList').innerHTML = '<div class="empty">Nenhuma imagem local.</div>';
    return;
  }

  el('imageList').innerHTML = images.map((image) => {
    const pending = state.docker.pendingDelete === image.id;
    return `
      <div class="docker-row">
        <div class="docker-row-main">
          <div class="docker-row-text">
            <strong>${escapeHtml(image.reference)}</strong>
            <span>${escapeHtml(image.id)} &middot; criada ${escapeHtml(image.createdSince || '-')}</span>
          </div>
        </div>
        <div class="docker-row-meta"><span class="ports">${escapeHtml(image.size || '-')}</span></div>
        <div class="docker-row-actions">
          <button class="secondary" data-profile="${escapeHtml(image.reference)}">Criar perfil</button>
          <button class="danger ghost ${pending ? 'pending' : ''}" data-remove-image="${escapeHtml(image.id)}">${pending ? 'Confirmar' : 'Remover'}</button>
        </div>
      </div>
    `;
  }).join('');
}

function switchDockerTab(tab) {
  state.docker.tab = tab;
  document.querySelectorAll('#dockerTabs .tab-item').forEach((item) => {
    const active = item.dataset.dockerTab === tab;
    item.classList.toggle('active', active);
    item.setAttribute('aria-selected', String(active));
  });
  document.querySelectorAll('.docker-pane').forEach((item) => item.classList.toggle('active', item.id === `${tab}Pane`));
}

async function showContainerLogs(id) {
  state.docker.selected = id;
  const container = state.docker.containers.find((item) => item.id === id);
  el('logsTitle').textContent = `Logs de ${container?.name || id}`;
  el('dockerLogs').textContent = 'Carregando logs...';
  renderContainers();
  state.docker.logs = await DockerContainerLogs(id, 300);
  el('dockerLogs').textContent = state.docker.logs;
}

// createDockerProfile leva a imagem escolhida direto para um perfil novo do runner.
function createDockerProfile(reference) {
  selectConfig('');
  const shortName = reference.split(':')[0].split('/').pop();
  el('runtime').value = 'docker';
  el('name').value = shortName;
  el('dockerMode').value = 'image';
  el('dockerImage').value = reference;
  el('dockerContainer').value = `${shortName}-local`;
  renderDockerMode();
  renderRuntimeFields();
  switchView('runner');
  switchProjectTab('docker');
  refreshDockerPreview();
  showNotice(`Perfil docker preparado para ${reference}. Ajuste as portas e salve.`, false);
}

async function refreshDockerPreview() {
  if (el('runtime').value !== 'docker') return;
  try {
    state.docker.preview = await PreviewDockerCommand(currentForm());
    el('dockerPreview').textContent = state.docker.preview;
    el('dockerPreview').classList.remove('preview-error');
  } catch (error) {
    el('dockerPreview').textContent = error?.message || String(error);
    el('dockerPreview').classList.add('preview-error');
  }
}

/* -------------------------------------------------------------------- node */

function renderNode() {
  const node = state.node || {};
  const installed = node.installedVersions || [];
  const available = node.availableVersions || [];
  const allVersions = available.length ? available : installed.map((version) => ({ version, channel: 'Installed', installed: true }));
  const active = normalizeVersion(node.currentVersion || '');
  const options = allVersions.map((item) => {
    const version = normalizeVersion(item.version);
    const label = `${version} - ${item.channel || 'Stable'}${item.installed ? ' - instalado' : ' - baixar'}`;
    return `<option value="${escapeHtml(version)}">${escapeHtml(label)}</option>`;
  });

  el('nodeVersion').innerHTML = options.join('');
  el('nodeVersion').value = active && allVersions.some((item) => normalizeVersion(item.version) === active)
    ? active
    : normalizeVersion(allVersions[0]?.version || '');
  el('profileNodeVersion').innerHTML = [
    '<option value="">Versao ativa global</option>',
    ...installed.map((version) => `<option value="${escapeHtml(version)}">${escapeHtml(version)}</option>`),
  ].join('');

  el('nodeStatus').innerHTML = `
    <div><span>Ativa</span><strong>${escapeHtml(node.currentVersion || 'nao selecionada')}</strong></div>
    <div><span>Instaladas</span><strong class="badges">${installed.length ? installed.map((version) => nodeBadge(version, version === active)).join('') : '-'}</strong></div>
    <div><span>Disponiveis</span><strong class="badges">${available.length ? available.slice(0, 12).map((item) => badge(`${item.version} ${item.channel}`, item.installed ? 'installed' : '')).join('') : '-'}</strong></div>
    <div><span>Pasta</span><strong>${escapeHtml(node.managedDirectory || '-')}</strong></div>
    ${node.message ? `<small>${escapeHtml(node.message)}</small>` : ''}
  `;
}

function renderProgress() {
  const items = state.progress.slice(-8).reverse();
  const html = items.length ? items.map((item) => `
    <div><span>${escapeHtml(item.time || '')}</span><strong>${escapeHtml(item.scope || 'app')}</strong><p>${escapeHtml(item.message || '')}</p></div>
  `).join('') : '<div class="empty">Sem eventos recentes.</div>';
  el('progressLog').innerHTML = html;
  el('processProgressLog').innerHTML = html;
  el('modalLog').innerHTML = html;
}

/* ----------------------------------------------------------------- helpers */

async function runAction(action, successMessage = '') {
  try {
    showNotice('Executando...', false);
    await action();
    showNotice(successMessage || 'Acao concluida.', false);
  } catch (error) {
    const message = error?.message || String(error);
    showNotice(message, true);
    showActionError(message);
  }
}

async function runWithSplash(title, action, successMessage = '') {
  showModal(title, 'Preparando...');
  try {
    showNotice('Executando...', false);
    await action();
    showNotice(successMessage || 'Acao concluida.', false);
  } catch (error) {
    const message = error?.message || String(error);
    showNotice(message, true);
    showActionError(message);
  } finally {
    hideModal();
  }
}

function showActionError(message) {
  const output = el('output');
  if (!output) return;
  output.textContent = [
    'Falha na acao executada:',
    '',
    message,
    '',
    'Verifique o caminho selecionado, o comando e a saida do processo acima.',
  ].join('\n');
}

function showModal(title, message) {
  el('modalTitle').textContent = title;
  el('modalMessage').textContent = message;
  el('modal').classList.add('open');
  renderProgress();
}

function hideModal() {
  el('modal').classList.remove('open');
}

function showNotice(message, isError) {
  el('notice').textContent = message;
  el('notice').classList.toggle('error', !!isError);
  el('notice').classList.toggle('open', !!message);
}

function detectRuntime(command) {
  const lower = command.toLowerCase();
  if (lower.includes('docker')) return 'docker';
  if (lower.includes('npm') || lower.includes('pnpm') || lower.includes('yarn') || lower.includes('node')) return 'node';
  if (lower.includes('dotnet')) return 'dotnet';
  if (lower.includes('go ')) return 'go';
  if (lower.includes('python') || lower.includes('uvicorn') || lower.includes('flask')) return 'python';
  return 'custom';
}

function baseName(value) {
  return String(value || '').split(/[\\/]/).pop();
}

function normalizeVersion(version) {
  return String(version).trim().replace(/^v/, '');
}

function badge(text, variant = '') {
  return `<span class="badge ${variant}">${escapeHtml(text)}</span>`;
}

function nodeBadge(version, active) {
  return `
    <span class="badge ${active ? 'active' : 'installed'}">
      ${escapeHtml(version)}
      <button class="trash-version" data-version="${escapeHtml(version)}" title="Excluir versao">&#128465;</button>
    </span>
  `;
}

function escapeHtml(value) {
  return String(value ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;');
}

/* ----------------------------------------------------------------- eventos */

document.querySelector('.tabs-nav').addEventListener('click', (event) => {
  const button = event.target.closest('.tab-item');
  if (button) switchView(button.dataset.view);
});

el('profileList').addEventListener('click', (event) => {
  const button = event.target.closest('.profile');
  if (button) selectConfig(button.dataset.id);
});

el('newProfile').addEventListener('click', () => {
  selectConfig('');
  switchView('runner');
});

el('runtime').addEventListener('change', () => {
  renderRuntimeFields();
  if (el('runtime').value === 'docker') {
    switchProjectTab('docker');
    refreshDockerPreview();
  }
});

['name', 'path', 'command', 'runtime', 'projectFile'].forEach((id) => {
  el(id).addEventListener('input', renderRunnerHint);
  el(id).addEventListener('change', renderRunnerHint);
});

el('projectTabs').addEventListener('click', (event) => {
  const button = event.target.closest('.tab-item');
  if (!button || button.classList.contains('hidden')) return;
  switchProjectTab(button.dataset.projectTab);
  if (button.dataset.projectTab === 'appsettings' && el('appSettings').value && !state.settings.tree) {
    runAction(() => loadSettingsTree(), 'AppSettings carregado.');
  }
  if (button.dataset.projectTab === 'docker') refreshDockerPreview();
});

el('preset').addEventListener('change', (event) => {
  const option = event.target.selectedOptions[0];
  if (!event.target.value) return;
  el('command').value = event.target.value;
  if (option?.dataset.runtime) el('runtime').value = option.dataset.runtime;
  renderRuntimeFields();
  renderRunnerHint();
});

el('pickPath').addEventListener('click', async () => {
  const path = await ChooseDirectory();
  if (!path) return;
  el('path').value = path;
  renderRunnerHint();
  if (el('runtime').value === 'dotnet') {
    await runAction(async () => {
      state.solutions = (await FindDotnetProjects(path)) || [];
      if (!state.solutions.length) throw new Error('nenhum .csproj encontrado nessa pasta');
      renderSolutions(state.solutions[0] || '');
    }, 'Projetos .NET pesquisados.');
  }
  if (el('runtime').value === 'docker') {
    await runAction(scanDockerFiles, 'Arquivos docker pesquisados.');
  }
});

el('scanProjects').addEventListener('click', () => runAction(async () => {
  state.solutions = (await FindDotnetProjects(el('path').value.trim())) || [];
  if (!state.solutions.length) throw new Error('nenhum .csproj encontrado nessa pasta');
  renderSolutions(state.solutions[0] || '');
}, 'Projetos .NET pesquisados.'));

el('pickProjectFile').addEventListener('click', () => runAction(async () => {
  const file = await ChooseDotnetProject(el('path').value.trim());
  if (!file) return;
  state.solutions = [file, ...state.solutions.filter((item) => item !== file)];
  renderSolutions(file);
  el('path').value = file.replace(/[\\/][^\\/]+\.csproj$/i, '');
  renderRunnerHint();
}, 'Projeto .NET selecionado.'));

el('scanAppSettings').addEventListener('click', () => runAction(async () => {
  state.appSettingsFiles = (await FindAppSettings(el('path').value.trim())) || [];
  if (!state.appSettingsFiles.length) throw new Error('nenhum appsettings encontrado nessa pasta');
  await loadSettingsTree(state.appSettingsFiles[0]);
  switchProjectTab('appsettings');
}, 'AppSettings pesquisado.'));

el('pickAppSettings').addEventListener('click', () => runAction(async () => {
  const file = await ChooseAppSettingsFile(el('path').value.trim());
  if (!file) return;
  await loadSettingsTree(file);
  switchProjectTab('appsettings');
}, 'AppSettings selecionado.'));

el('appSettings').addEventListener('change', () => runAction(() => loadSettingsTree(), 'AppSettings carregado.'));
el('reloadAppSettings').addEventListener('click', () => runAction(() => loadSettingsTree(), 'AppSettings recarregado.'));

el('treeFilter').addEventListener('input', (event) => {
  state.settings.filter = event.target.value;
  renderSettingsTree();
});

el('expandAll').addEventListener('click', () => {
  if (!state.settings.tree) return;
  walkTree(state.settings.tree, (node) => {
    if (isContainer(node)) state.settings.expanded.add(node.path);
  });
  renderSettingsTree();
});

el('collapseAll').addEventListener('click', () => {
  state.settings.expanded.clear();
  renderSettingsTree();
});

el('settingsTree').addEventListener('click', (event) => {
  const toggle = event.target.closest('[data-toggle]');
  if (toggle) {
    const path = toggle.dataset.toggle;
    if (state.settings.expanded.has(path)) state.settings.expanded.delete(path);
    else state.settings.expanded.add(path);
    renderSettingsTree();
    return;
  }

  const add = event.target.closest('[data-add]');
  if (add) {
    state.settings.addParent = state.settings.addParent === add.dataset.add ? '' : add.dataset.add;
    renderSettingsTree();
    el('newKey').focus();
    return;
  }

  const remove = event.target.closest('[data-remove]');
  if (remove) {
    const path = remove.dataset.remove;
    if (state.settings.pendingDelete !== path) {
      state.settings.pendingDelete = path;
      renderSettingsTree();
      showNotice(`Clique em confirmar para remover ${path}.`, false);
      return;
    }
    state.settings.pendingDelete = '';
    runAction(async () => {
      state.settings.tree = await DeleteAppSettingsNode(state.settings.file, path);
      if (state.settings.addParent === path) state.settings.addParent = '';
      renderSettingsTree();
    }, 'Chave removida do arquivo.');
    return;
  }

  const save = event.target.closest('[data-save]');
  if (save) {
    const value = el('treeEditor')?.value ?? '';
    runAction(() => saveTreeValue(save.dataset.save, value), 'Chave atualizada no arquivo.');
    return;
  }

  if (event.target.closest('[data-cancel]')) {
    state.settings.editing = '';
    renderSettingsTree();
    return;
  }

  const edit = event.target.closest('[data-edit]');
  if (edit) {
    state.settings.editing = edit.dataset.edit;
    state.settings.pendingDelete = '';
    renderSettingsTree();
  }
});

el('settingsTree').addEventListener('keydown', (event) => {
  if (event.target.id !== 'treeEditor') return;
  if (event.key === 'Enter') {
    runAction(() => saveTreeValue(event.target.dataset.path, event.target.value), 'Chave atualizada no arquivo.');
  }
  if (event.key === 'Escape') {
    state.settings.editing = '';
    renderSettingsTree();
  }
});

el('clearAddTarget').addEventListener('click', () => {
  state.settings.addParent = '';
  renderSettingsTree();
});

el('addSetting').addEventListener('click', () => runAction(async () => {
  if (!state.settings.file) throw new Error('carregue um appsettings antes');
  const key = el('newKey').value.trim();
  if (!key) throw new Error('informe o nome da chave');
  state.settings.tree = await AddAppSettingsNode(
    state.settings.file,
    state.settings.addParent,
    key,
    el('newKind').value,
    el('newValue').value,
  );
  el('newKey').value = '';
  el('newValue').value = '';
  state.settings.expanded.add(state.settings.addParent ? `${state.settings.addParent}:${key}` : key);
  renderSettingsTree();
}, 'Chave adicionada ao arquivo.'));

/* ----------------------------------------------------------- perfil docker */

el('dockerModeTabs').addEventListener('click', (event) => {
  const button = event.target.closest('.tab-item');
  if (!button) return;
  el('dockerMode').value = button.dataset.dockerMode;
  renderDockerMode();
  renderRunnerHint();
  refreshDockerPreview();
});

async function scanDockerFiles() {
  const found = await FindDockerFiles(el('path').value.trim());
  state.docker.composeFiles = found.composeFiles || [];
  state.docker.dockerfiles = found.dockerfiles || [];
  renderOptions('dockerCompose', state.docker.composeFiles, state.docker.composeFiles[0] || '', 'Selecione o compose');
  renderOptions('dockerfile', state.docker.dockerfiles, state.docker.dockerfiles[0] || '', 'Selecione o Dockerfile');
  if (state.docker.composeFiles[0]) await loadComposeServices();
  await refreshDockerPreview();
}

async function loadComposeServices() {
  const file = el('dockerCompose').value;
  if (!file) throw new Error('selecione o arquivo compose');
  state.docker.services = (await ListComposeServices(file)) || [];
  renderOptions('dockerService', state.docker.services, '', 'Todos os servicos');
}

el('scanCompose').addEventListener('click', () => runAction(scanDockerFiles, 'Arquivos docker pesquisados.'));
el('scanDockerfile').addEventListener('click', () => runAction(scanDockerFiles, 'Arquivos docker pesquisados.'));
el('scanServices').addEventListener('click', () => runAction(loadComposeServices, 'Servicos do compose carregados.'));

el('pickCompose').addEventListener('click', () => runAction(async () => {
  const file = await ChooseComposeFile(el('path').value.trim());
  if (!file) return;
  state.docker.composeFiles = [file, ...state.docker.composeFiles.filter((item) => item !== file)];
  renderOptions('dockerCompose', state.docker.composeFiles, file, 'Selecione o compose');
  await refreshDockerPreview();
}, 'Compose selecionado.'));

el('pickDockerfile').addEventListener('click', () => runAction(async () => {
  const file = await ChooseDockerfile(el('path').value.trim());
  if (!file) return;
  state.docker.dockerfiles = [file, ...state.docker.dockerfiles.filter((item) => item !== file)];
  renderOptions('dockerfile', state.docker.dockerfiles, file, 'Selecione o Dockerfile');
  await refreshDockerPreview();
}, 'Dockerfile selecionado.'));

[
  'dockerCompose', 'dockerService', 'dockerProject', 'dockerfile', 'dockerContext',
  'dockerImage', 'dockerContainer', 'dockerPorts', 'dockerEnv', 'dockerVolumes',
  'dockerArgs', 'dockerCommand', 'dockerBuild', 'dockerRecreate', 'dockerRemove',
].forEach((id) => {
  el(id).addEventListener('change', () => {
    renderRunnerHint();
    refreshDockerPreview();
  });
});

el('refreshPreview').addEventListener('click', () => runAction(refreshDockerPreview, 'Comando atualizado.'));

el('saveDocker').addEventListener('click', () => runAction(async () => {
  await saveConfig();
  await refreshDockerPreview();
}, 'Perfil docker salvo.'));

/* ------------------------------------------------------------ acoes runner */

el('save').addEventListener('click', () => runAction(saveConfig, 'Perfil salvo.'));

el('delete').addEventListener('click', () => runAction(async () => {
  if (!state.selectedId) throw new Error('nenhum perfil selecionado');
  state.configs = await DeleteConfig(state.selectedId);
  selectConfig(state.configs[0]?.id || '');
}, 'Perfil excluido.'));

el('start').addEventListener('click', () => runWithSplash('Iniciando', async () => {
  await saveConfig();
  const status = await Start(currentForm());
  state.activeConsoleId = status.projectId;
  await refreshStatus();
}, 'Processo iniciado.'));

el('stop').addEventListener('click', () => runWithSplash('Parando', async () => {
  const id = state.activeConsoleId || state.selectedId;
  if (!id) throw new Error('nenhum processo selecionado');
  await Stop(id);
  await refreshStatus();
}, 'Processo parado.'));

el('consoleTabs').addEventListener('click', (event) => {
  const close = event.target.closest('[data-close]');
  if (close) {
    event.stopPropagation();
    runWithSplash('Parando', async () => {
      await Stop(close.dataset.close);
      await refreshStatus();
    }, 'Processo parado.');
    return;
  }
  const button = event.target.closest('.tab-item');
  if (!button) return;
  state.activeConsoleId = button.dataset.id;
  renderStatus();
});

/* ------------------------------------------------------------ acoes docker */

el('dockerTabs').addEventListener('click', (event) => {
  const button = event.target.closest('.tab-item');
  if (button) switchDockerTab(button.dataset.dockerTab);
});

el('refreshDocker').addEventListener('click', () => runAction(loadDocker, 'Docker atualizado.'));

el('dockerShowAll').addEventListener('change', (event) => {
  state.docker.showAll = event.target.checked;
  runAction(refreshDockerLists, 'Lista atualizada.');
});

el('containerList').addEventListener('click', (event) => {
  const open = event.target.closest('[data-open]');
  if (open) {
    BrowserOpenURL(open.dataset.open);
    return;
  }

  const start = event.target.closest('[data-start]');
  if (start) {
    runWithSplash('Iniciando container', async () => {
      state.docker.containers = await StartDockerContainer(start.dataset.start);
      renderContainers();
      state.docker.info = await GetDockerInfo();
      renderDockerStatus();
    }, 'Container iniciado.');
    return;
  }

  const stop = event.target.closest('[data-stop]');
  if (stop) {
    runWithSplash('Parando container', async () => {
      state.docker.containers = await StopDockerContainer(stop.dataset.stop);
      renderContainers();
      state.docker.info = await GetDockerInfo();
      renderDockerStatus();
    }, 'Container parado.');
    return;
  }

  const restart = event.target.closest('[data-restart]');
  if (restart) {
    runWithSplash('Reiniciando container', async () => {
      state.docker.containers = await RestartDockerContainer(restart.dataset.restart);
      renderContainers();
    }, 'Container reiniciado.');
    return;
  }

  const logs = event.target.closest('[data-logs]');
  if (logs) {
    runAction(() => showContainerLogs(logs.dataset.logs), 'Logs carregados.');
    return;
  }

  const remove = event.target.closest('[data-remove-container]');
  if (remove) {
    const id = remove.dataset.removeContainer;
    if (state.docker.pendingDelete !== id) {
      state.docker.pendingDelete = id;
      renderContainers();
      showNotice('Clique em Confirmar para remover o container.', false);
      return;
    }
    state.docker.pendingDelete = '';
    runWithSplash('Removendo container', async () => {
      state.docker.containers = await RemoveDockerContainer(id);
      if (state.docker.selected === id) {
        state.docker.selected = '';
        el('dockerLogs').textContent = 'Selecione um container para ver os logs.';
      }
      renderContainers();
      state.docker.info = await GetDockerInfo();
      renderDockerStatus();
    }, 'Container removido.');
    return;
  }

  const row = event.target.closest('[data-container]');
  if (row) runAction(() => showContainerLogs(row.dataset.container), 'Logs carregados.');
});

el('imageList').addEventListener('click', (event) => {
  const profile = event.target.closest('[data-profile]');
  if (profile) {
    createDockerProfile(profile.dataset.profile);
    return;
  }

  const remove = event.target.closest('[data-remove-image]');
  if (!remove) return;
  const id = remove.dataset.removeImage;
  if (state.docker.pendingDelete !== id) {
    state.docker.pendingDelete = id;
    renderImages();
    showNotice('Clique em Confirmar para remover a imagem.', false);
    return;
  }
  state.docker.pendingDelete = '';
  runWithSplash('Removendo imagem', async () => {
    state.docker.images = await RemoveDockerImage(id);
    renderImages();
  }, 'Imagem removida.');
});

el('pullImageButton').addEventListener('click', () => runWithSplash('Baixando imagem', async () => {
  state.docker.images = await PullDockerImage(el('pullImage').value.trim());
  renderImages();
  switchDockerTab('images');
}, 'Imagem baixada.'));

el('refreshLogs').addEventListener('click', () => runAction(async () => {
  if (!state.docker.selected) throw new Error('selecione um container');
  await showContainerLogs(state.docker.selected);
}, 'Logs recarregados.'));

/* -------------------------------------------------------------- acoes node */

el('selectNode').addEventListener('click', () => {
  const version = el('nodeVersion').value;
  if (!version) {
    showNotice('Selecione uma versao Node primeiro.', true);
    return;
  }
  showNotice(`Versao ${version} selecionada. Use Trocar versao para ativar ou Baixar selecionada para instalar.`);
});

el('activateNode').addEventListener('click', () => runWithSplash('Trocando Node', async () => {
  const version = el('nodeVersion').value;
  if (!version) throw new Error('selecione uma versao Node');
  state.node = await UseNodeVersion(version);
  renderNode();
}, 'Versao Node ativa. Abra um novo PowerShell para conferir com node --version.'));

el('nodeVersion').addEventListener('change', () => {
  const version = el('nodeVersion').value;
  if (version) showNotice(`Versao ${version} selecionada.`);
});

el('installSelectedNode').addEventListener('click', () => runWithSplash('Instalando Node', async () => {
  const version = el('nodeVersion').value;
  if (!version) throw new Error('selecione uma versao Node');
  state.node = await InstallNodeVersion(version);
  renderNode();
}, 'Versao Node instalada. Use Trocar versao para ativar.'));

el('installNode').addEventListener('click', () => runWithSplash('Instalando Node', async () => {
  const version = el('installVersion').value.trim();
  state.node = await InstallNodeVersion(version);
  renderNode();
}, 'Versao Node instalada. Use Trocar versao para ativar.'));

el('refreshNode').addEventListener('click', () => runAction(async () => {
  state.node = await RefreshNodeVersionList();
  renderNode();
}, 'Lista Node buscada e salva.'));

el('deleteNode').addEventListener('click', () => runWithSplash('Removendo Node', async () => {
  const version = el('nodeVersion').value;
  if (!version) throw new Error('selecione uma versao Node');
  state.node = await DeleteNodeVersion(version);
  renderNode();
}, 'Versao Node excluida.'));

el('nodeStatus').addEventListener('click', (event) => {
  const button = event.target.closest('.trash-version');
  if (!button) return;
  event.preventDefault();
  event.stopPropagation();
  runWithSplash('Removendo Node', async () => {
    state.node = await DeleteNodeVersion(button.dataset.version);
    renderNode();
  }, 'Versao Node excluida.');
});

/* ------------------------------------------------------------------ janela */

el('hideToTray').addEventListener('click', HideToTray);

el('optionsToggle').addEventListener('click', () => {
  el('optionsMenu').classList.toggle('open');
});

el('openSettings').addEventListener('click', () => {
  el('optionsMenu').classList.remove('open');
  OpenSettings();
  switchView('settings');
});

el('stopAll').addEventListener('click', () => runWithSplash('Parando todos', async () => {
  el('optionsMenu').classList.remove('open');
  await StopAll();
  await refreshStatus();
}, 'Todos os processos foram parados.'));

el('quitApp').addEventListener('click', async () => {
  await StopAll();
  Quit();
});

EventsOn('navigate:settings', () => switchView('settings'));
EventsOn('app:progress', (event) => {
  state.progress.push(event);
  renderProgress();
  if (el('modal').classList.contains('open')) {
    el('modalMessage').textContent = event.message || 'Processando...';
  }
  showNotice(`${event.scope}: ${event.message}`, false);
});

boot().catch((error) => {
  document.querySelector('#app').innerHTML = `<main class="fatal">${escapeHtml(error.message || error)}</main>`;
});
