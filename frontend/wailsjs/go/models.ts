export namespace main {
	
	export class AppSettingEntry {
	    section: string;
	    key: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new AppSettingEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.section = source["section"];
	        this.key = source["key"];
	        this.value = source["value"];
	    }
	}
	export class CommandPreset {
	    label: string;
	    command: string;
	    runtime: string;
	
	    static createFrom(source: any = {}) {
	        return new CommandPreset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.command = source["command"];
	        this.runtime = source["runtime"];
	    }
	}
	export class DockerConfig {
	    mode: string;
	    composeFile: string;
	    service: string;
	    projectName: string;
	    dockerfile: string;
	    context: string;
	    image: string;
	    containerName: string;
	    ports: string;
	    envVars: string;
	    volumes: string;
	    extraArgs: string;
	    command: string;
	    build: boolean;
	    recreate: boolean;
	    removeOnStop: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DockerConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.composeFile = source["composeFile"];
	        this.service = source["service"];
	        this.projectName = source["projectName"];
	        this.dockerfile = source["dockerfile"];
	        this.context = source["context"];
	        this.image = source["image"];
	        this.containerName = source["containerName"];
	        this.ports = source["ports"];
	        this.envVars = source["envVars"];
	        this.volumes = source["volumes"];
	        this.extraArgs = source["extraArgs"];
	        this.command = source["command"];
	        this.build = source["build"];
	        this.recreate = source["recreate"];
	        this.removeOnStop = source["removeOnStop"];
	    }
	}
	export class DockerContainer {
	    id: string;
	    name: string;
	    image: string;
	    state: string;
	    status: string;
	    ports: string;
	    compose: string;
	    service: string;
	    createdAt: string;
	    running: boolean;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new DockerContainer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.image = source["image"];
	        this.state = source["state"];
	        this.status = source["status"];
	        this.ports = source["ports"];
	        this.compose = source["compose"];
	        this.service = source["service"];
	        this.createdAt = source["createdAt"];
	        this.running = source["running"];
	        this.url = source["url"];
	    }
	}
	export class DockerFiles {
	    composeFiles: string[];
	    dockerfiles: string[];
	
	    static createFrom(source: any = {}) {
	        return new DockerFiles(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.composeFiles = source["composeFiles"];
	        this.dockerfiles = source["dockerfiles"];
	    }
	}
	export class DockerImage {
	    id: string;
	    repository: string;
	    tag: string;
	    reference: string;
	    size: string;
	    createdSince: string;
	    dangling: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DockerImage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.repository = source["repository"];
	        this.tag = source["tag"];
	        this.reference = source["reference"];
	        this.size = source["size"];
	        this.createdSince = source["createdSince"];
	        this.dangling = source["dangling"];
	    }
	}
	export class DockerInfo {
	    available: boolean;
	    engineRunning: boolean;
	    version: string;
	    composeAvailable: boolean;
	    composeCommand: string;
	    composeVersion: string;
	    containers: number;
	    runningContainers: number;
	    images: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new DockerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.engineRunning = source["engineRunning"];
	        this.version = source["version"];
	        this.composeAvailable = source["composeAvailable"];
	        this.composeCommand = source["composeCommand"];
	        this.composeVersion = source["composeVersion"];
	        this.containers = source["containers"];
	        this.runningContainers = source["runningContainers"];
	        this.images = source["images"];
	        this.message = source["message"];
	    }
	}
	export class NodeVersionOption {
	    version: string;
	    channel: string;
	    installed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NodeVersionOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.channel = source["channel"];
	        this.installed = source["installed"];
	    }
	}
	export class NodeInfo {
	    currentVersion: string;
	    installedVersions: string[];
	    availableVersions: NodeVersionOption[];
	    managedDirectory: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new NodeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.installedVersions = source["installedVersions"];
	        this.availableVersions = this.convertValues(source["availableVersions"], NodeVersionOption);
	        this.managedDirectory = source["managedDirectory"];
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ProcessStatus {
	    running: boolean;
	    message: string;
	    projectId: string;
	    projectName: string;
	    port: number;
	    url: string;
	    docsActive: boolean;
	    docsUrl: string;
	    detectedRuntime: string;
	    startedAt: string;
	    output: string;
	
	    static createFrom(source: any = {}) {
	        return new ProcessStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.message = source["message"];
	        this.projectId = source["projectId"];
	        this.projectName = source["projectName"];
	        this.port = source["port"];
	        this.url = source["url"];
	        this.docsActive = source["docsActive"];
	        this.docsUrl = source["docsUrl"];
	        this.detectedRuntime = source["detectedRuntime"];
	        this.startedAt = source["startedAt"];
	        this.output = source["output"];
	    }
	}
	export class ProjectConfig {
	    id: string;
	    name: string;
	    path: string;
	    runtime: string;
	    command: string;
	    nodeVersion: string;
	    solution: string;
	    projectFile: string;
	    appSettings: string;
	    docker: DockerConfig;
	
	    static createFrom(source: any = {}) {
	        return new ProjectConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.runtime = source["runtime"];
	        this.command = source["command"];
	        this.nodeVersion = source["nodeVersion"];
	        this.solution = source["solution"];
	        this.projectFile = source["projectFile"];
	        this.appSettings = source["appSettings"];
	        this.docker = this.convertValues(source["docker"], DockerConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SettingsNode {
	    key: string;
	    path: string;
	    kind: string;
	    value: string;
	    children: SettingsNode[];
	
	    static createFrom(source: any = {}) {
	        return new SettingsNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.path = source["path"];
	        this.kind = source["kind"];
	        this.value = source["value"];
	        this.children = this.convertValues(source["children"], SettingsNode);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

