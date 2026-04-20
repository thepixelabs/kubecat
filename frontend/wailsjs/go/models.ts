export namespace ai {
	
	export class ApprovalDecision {
	    Approved: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ApprovalDecision(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Approved = source["Approved"];
	    }
	}

}

export namespace client {
	
	export class ClusterInfo {
	    Name: string;
	    Context: string;
	    Server: string;
	    Version: string;
	    Status: number;
	    // Go type: time
	    LastCheck: any;
	    Error: string;
	    NodeCount: number;
	    PodCount: number;
	    NamespaceCount: number;
	
	    static createFrom(source: any = {}) {
	        return new ClusterInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Context = source["Context"];
	        this.Server = source["Server"];
	        this.Version = source["Version"];
	        this.Status = source["Status"];
	        this.LastCheck = this.convertValues(source["LastCheck"], null);
	        this.Error = source["Error"];
	        this.NodeCount = source["NodeCount"];
	        this.PodCount = source["PodCount"];
	        this.NamespaceCount = source["NamespaceCount"];
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

export namespace config {
	
	export class CostConfig {
	    CPUCostPerCoreHour: number;
	    MemCostPerGBHour: number;
	    Currency: string;
	    OpenCostEndpoint: string;
	
	    static createFrom(source: any = {}) {
	        return new CostConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.CPUCostPerCoreHour = source["CPUCostPerCoreHour"];
	        this.MemCostPerGBHour = source["MemCostPerGBHour"];
	        this.Currency = source["Currency"];
	        this.OpenCostEndpoint = source["OpenCostEndpoint"];
	    }
	}

}

export namespace cost {
	
	export class CostEstimate {
	    workloadName: string;
	    namespace: string;
	    cpuCost: number;
	    memoryCost: number;
	    totalCost: number;
	    monthlyTotal: number;
	    currency: string;
	    period: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new CostEstimate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workloadName = source["workloadName"];
	        this.namespace = source["namespace"];
	        this.cpuCost = source["cpuCost"];
	        this.memoryCost = source["memoryCost"];
	        this.totalCost = source["totalCost"];
	        this.monthlyTotal = source["monthlyTotal"];
	        this.currency = source["currency"];
	        this.period = source["period"];
	        this.source = source["source"];
	    }
	}
	export class NamespaceCostSummary {
	    namespace: string;
	    totalPerHour: number;
	    totalPerMonth: number;
	    currency: string;
	    source: string;
	    workloads: CostEstimate[];
	
	    static createFrom(source: any = {}) {
	        return new NamespaceCostSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.namespace = source["namespace"];
	        this.totalPerHour = source["totalPerHour"];
	        this.totalPerMonth = source["totalPerMonth"];
	        this.currency = source["currency"];
	        this.source = source["source"];
	        this.workloads = this.convertValues(source["workloads"], CostEstimate);
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

export namespace main {
	
	export class AIContextItem {
	    id: string;
	    type: string;
	    namespace?: string;
	    name: string;
	    cluster: string;
	
	    static createFrom(source: any = {}) {
	        return new AIContextItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.namespace = source["namespace"];
	        this.name = source["name"];
	        this.cluster = source["cluster"];
	    }
	}
	export class ProviderConfig {
	    enabled: boolean;
	    apiKey: string;
	    endpoint: string;
	    models: string[];
	
	    static createFrom(source: any = {}) {
	        return new ProviderConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.apiKey = source["apiKey"];
	        this.endpoint = source["endpoint"];
	        this.models = source["models"];
	    }
	}
	export class AISettings {
	    enabled: boolean;
	    selectedProvider: string;
	    selectedModel: string;
	    providers: Record<string, ProviderConfig>;
	
	    static createFrom(source: any = {}) {
	        return new AISettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.selectedProvider = source["selectedProvider"];
	        this.selectedModel = source["selectedModel"];
	        this.providers = this.convertValues(source["providers"], ProviderConfig, true);
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
	export class AlertSettings {
	    enabled: boolean;
	    scanIntervalSeconds: number;
	    cooldownMinutes: number;
	    ignoredNamespaces: string[];
	
	    static createFrom(source: any = {}) {
	        return new AlertSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.scanIntervalSeconds = source["scanIntervalSeconds"];
	        this.cooldownMinutes = source["cooldownMinutes"];
	        this.ignoredNamespaces = source["ignoredNamespaces"];
	    }
	}
	export class AnalyzerFix {
	    description: string;
	    yaml?: string;
	    command?: string;
	
	    static createFrom(source: any = {}) {
	        return new AnalyzerFix(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.description = source["description"];
	        this.yaml = source["yaml"];
	        this.command = source["command"];
	    }
	}
	export class AnalyzerIssue {
	    id: string;
	    category: string;
	    severity: string;
	    title: string;
	    message: string;
	    resource: string;
	    namespace: string;
	    kind: string;
	    details?: Record<string, any>;
	    fixes?: AnalyzerFix[];
	    detectedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new AnalyzerIssue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.category = source["category"];
	        this.severity = source["severity"];
	        this.title = source["title"];
	        this.message = source["message"];
	        this.resource = source["resource"];
	        this.namespace = source["namespace"];
	        this.kind = source["kind"];
	        this.details = source["details"];
	        this.fixes = this.convertValues(source["fixes"], AnalyzerFix);
	        this.detectedAt = source["detectedAt"];
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
	export class AnalyzerSummary {
	    critical: number;
	    warning: number;
	    info: number;
	    issuesByCategory: Record<string, Array<AnalyzerIssue>>;
	    scannedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new AnalyzerSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.critical = source["critical"];
	        this.warning = source["warning"];
	        this.info = source["info"];
	        this.issuesByCategory = this.convertValues(source["issuesByCategory"], Array<AnalyzerIssue>, true);
	        this.scannedAt = source["scannedAt"];
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
	export class ApplyResult {
	    success: boolean;
	    dryRun: boolean;
	    message: string;
	    changes: string[];
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new ApplyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.dryRun = source["dryRun"];
	        this.message = source["message"];
	        this.changes = source["changes"];
	        this.warnings = source["warnings"];
	    }
	}
	export class ClusterEdge {
	    id: string;
	    source: string;
	    target: string;
	    edgeType: string;
	    label?: string;
	
	    static createFrom(source: any = {}) {
	        return new ClusterEdge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.source = source["source"];
	        this.target = source["target"];
	        this.edgeType = source["edgeType"];
	        this.label = source["label"];
	    }
	}
	export class ClusterHealthInfo {
	    context: string;
	    status: string;
	    nodeCount: number;
	    podCount: number;
	    cpuPercent: number;
	    memPercent: number;
	    issues: number;
	    lastChecked: string;
	
	    static createFrom(source: any = {}) {
	        return new ClusterHealthInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = source["context"];
	        this.status = source["status"];
	        this.nodeCount = source["nodeCount"];
	        this.podCount = source["podCount"];
	        this.cpuPercent = source["cpuPercent"];
	        this.memPercent = source["memPercent"];
	        this.issues = source["issues"];
	        this.lastChecked = source["lastChecked"];
	    }
	}
	export class ConversationMessage {
	    role: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new ConversationMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.content = source["content"];
	    }
	}
	export class ResourceInfo {
	    kind: string;
	    name: string;
	    namespace: string;
	    status: string;
	    age: string;
	    labels: Record<string, string>;
	    apiVersion: string;
	    replicas?: string;
	    restarts?: number;
	    node?: string;
	    qosClass?: string;
	    readyContainers?: string;
	    images?: string[];
	    cpuRequest?: string;
	    cpuLimit?: string;
	    memRequest?: string;
	    memLimit?: string;
	    serviceType?: string;
	    clusterIP?: string;
	    externalIP?: string;
	    ports?: string;
	    storageClass?: string;
	    capacity?: string;
	    accessModes?: string;
	    ingressClass?: string;
	    hosts?: string;
	    paths?: string;
	    tlsHosts?: string;
	    backends?: string;
	    selectors?: string;
	    dataKeys?: string[];
	    dataCount?: number;
	    ownerKind?: string;
	    ownerName?: string;
	    hasLiveness?: boolean;
	    hasReadiness?: boolean;
	    securityIssues?: string[];
	    volumes?: string[];
	    cpuAllocatable?: string;
	    memAllocatable?: string;
	    cpuCapacity?: string;
	    memCapacity?: string;
	    podCount?: number;
	    podCapacity?: number;
	    nodeConditions?: string[];
	    kubeletVersion?: string;
	    containerRuntime?: string;
	    osImage?: string;
	    architecture?: string;
	    taints?: string[];
	    unschedulable?: boolean;
	    internalIP?: string;
	    externalIPNode?: string;
	    roles?: string;
	
	    static createFrom(source: any = {}) {
	        return new ResourceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.status = source["status"];
	        this.age = source["age"];
	        this.labels = source["labels"];
	        this.apiVersion = source["apiVersion"];
	        this.replicas = source["replicas"];
	        this.restarts = source["restarts"];
	        this.node = source["node"];
	        this.qosClass = source["qosClass"];
	        this.readyContainers = source["readyContainers"];
	        this.images = source["images"];
	        this.cpuRequest = source["cpuRequest"];
	        this.cpuLimit = source["cpuLimit"];
	        this.memRequest = source["memRequest"];
	        this.memLimit = source["memLimit"];
	        this.serviceType = source["serviceType"];
	        this.clusterIP = source["clusterIP"];
	        this.externalIP = source["externalIP"];
	        this.ports = source["ports"];
	        this.storageClass = source["storageClass"];
	        this.capacity = source["capacity"];
	        this.accessModes = source["accessModes"];
	        this.ingressClass = source["ingressClass"];
	        this.hosts = source["hosts"];
	        this.paths = source["paths"];
	        this.tlsHosts = source["tlsHosts"];
	        this.backends = source["backends"];
	        this.selectors = source["selectors"];
	        this.dataKeys = source["dataKeys"];
	        this.dataCount = source["dataCount"];
	        this.ownerKind = source["ownerKind"];
	        this.ownerName = source["ownerName"];
	        this.hasLiveness = source["hasLiveness"];
	        this.hasReadiness = source["hasReadiness"];
	        this.securityIssues = source["securityIssues"];
	        this.volumes = source["volumes"];
	        this.cpuAllocatable = source["cpuAllocatable"];
	        this.memAllocatable = source["memAllocatable"];
	        this.cpuCapacity = source["cpuCapacity"];
	        this.memCapacity = source["memCapacity"];
	        this.podCount = source["podCount"];
	        this.podCapacity = source["podCapacity"];
	        this.nodeConditions = source["nodeConditions"];
	        this.kubeletVersion = source["kubeletVersion"];
	        this.containerRuntime = source["containerRuntime"];
	        this.osImage = source["osImage"];
	        this.architecture = source["architecture"];
	        this.taints = source["taints"];
	        this.unschedulable = source["unschedulable"];
	        this.internalIP = source["internalIP"];
	        this.externalIPNode = source["externalIPNode"];
	        this.roles = source["roles"];
	    }
	}
	export class CrossClusterSearchResult {
	    cluster: string;
	    resources: ResourceInfo[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new CrossClusterSearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cluster = source["cluster"];
	        this.resources = this.convertValues(source["resources"], ResourceInfo);
	        this.error = source["error"];
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
	export class RBACSubject {
	    kind: string;
	    name: string;
	    namespace?: string;
	
	    static createFrom(source: any = {}) {
	        return new RBACSubject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	    }
	}
	export class DangerousAccessInfo {
	    subject: RBACSubject;
	    reason: string;
	    binding: string;
	    namespace?: string;
	    permissions: string[];
	
	    static createFrom(source: any = {}) {
	        return new DangerousAccessInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.subject = this.convertValues(source["subject"], RBACSubject);
	        this.reason = source["reason"];
	        this.binding = source["binding"];
	        this.namespace = source["namespace"];
	        this.permissions = source["permissions"];
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
	export class DiffReport {
	    format: string;
	    content: string;
	    filename: string;
	
	    static createFrom(source: any = {}) {
	        return new DiffReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.content = source["content"];
	        this.filename = source["filename"];
	    }
	}
	export class DiffSource {
	    context: string;
	    snapshot?: string;
	    isLive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DiffSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = source["context"];
	        this.snapshot = source["snapshot"];
	        this.isLive = source["isLive"];
	    }
	}
	export class DiffRequest {
	    kind: string;
	    namespace: string;
	    name: string;
	    left: DiffSource;
	    right: DiffSource;
	
	    static createFrom(source: any = {}) {
	        return new DiffRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.namespace = source["namespace"];
	        this.name = source["name"];
	        this.left = this.convertValues(source["left"], DiffSource);
	        this.right = this.convertValues(source["right"], DiffSource);
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
	export class FieldDifference {
	    path: string;
	    leftValue: string;
	    rightValue: string;
	    category: string;
	    severity: string;
	    changeType: string;
	
	    static createFrom(source: any = {}) {
	        return new FieldDifference(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.leftValue = source["leftValue"];
	        this.rightValue = source["rightValue"];
	        this.category = source["category"];
	        this.severity = source["severity"];
	        this.changeType = source["changeType"];
	    }
	}
	export class DiffResult {
	    request: DiffRequest;
	    leftYaml: string;
	    rightYaml: string;
	    leftExists: boolean;
	    rightExists: boolean;
	    differences: FieldDifference[];
	    filteredPaths: string[];
	    computedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new DiffResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.request = this.convertValues(source["request"], DiffRequest);
	        this.leftYaml = source["leftYaml"];
	        this.rightYaml = source["rightYaml"];
	        this.leftExists = source["leftExists"];
	        this.rightExists = source["rightExists"];
	        this.differences = this.convertValues(source["differences"], FieldDifference);
	        this.filteredPaths = source["filteredPaths"];
	        this.computedAt = source["computedAt"];
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
	
	
	export class GitOpsSource {
	    type: string;
	    url?: string;
	    path?: string;
	    branch?: string;
	    revision?: string;
	    chart?: string;
	    version?: string;
	    repository?: string;
	
	    static createFrom(source: any = {}) {
	        return new GitOpsSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.url = source["url"];
	        this.path = source["path"];
	        this.branch = source["branch"];
	        this.revision = source["revision"];
	        this.chart = source["chart"];
	        this.version = source["version"];
	        this.repository = source["repository"];
	    }
	}
	export class GitOpsApplicationInfo {
	    name: string;
	    namespace: string;
	    provider: string;
	    kind: string;
	    source: GitOpsSource;
	    syncStatus: string;
	    healthStatus: string;
	    message?: string;
	    lastSyncTime?: string;
	    revision?: string;
	    labels?: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new GitOpsApplicationInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.provider = source["provider"];
	        this.kind = source["kind"];
	        this.source = this.convertValues(source["source"], GitOpsSource);
	        this.syncStatus = source["syncStatus"];
	        this.healthStatus = source["healthStatus"];
	        this.message = source["message"];
	        this.lastSyncTime = source["lastSyncTime"];
	        this.revision = source["revision"];
	        this.labels = source["labels"];
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
	
	export class GitOpsSummary {
	    total: number;
	    synced: number;
	    outOfSync: number;
	    healthy: number;
	    degraded: number;
	    progressing: number;
	
	    static createFrom(source: any = {}) {
	        return new GitOpsSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.synced = source["synced"];
	        this.outOfSync = source["outOfSync"];
	        this.healthy = source["healthy"];
	        this.degraded = source["degraded"];
	        this.progressing = source["progressing"];
	    }
	}
	export class GitOpsStatusInfo {
	    provider: string;
	    detected: boolean;
	    applications: GitOpsApplicationInfo[];
	    summary: GitOpsSummary;
	
	    static createFrom(source: any = {}) {
	        return new GitOpsStatusInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.detected = source["detected"];
	        this.applications = this.convertValues(source["applications"], GitOpsApplicationInfo);
	        this.summary = this.convertValues(source["summary"], GitOpsSummary);
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
	
	export class LogLine {
	    pod: string;
	    container: string;
	    line: string;
	    colorIdx: number;
	
	    static createFrom(source: any = {}) {
	        return new LogLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pod = source["pod"];
	        this.container = source["container"];
	        this.line = source["line"];
	        this.colorIdx = source["colorIdx"];
	    }
	}
	export class LogMatchInfo {
	    lineNumber: number;
	    line: string;
	
	    static createFrom(source: any = {}) {
	        return new LogMatchInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lineNumber = source["lineNumber"];
	        this.line = source["line"];
	    }
	}
	export class NodeAllocationInfo {
	    nodeName: string;
	    podCount: number;
	    cpuRequests: string;
	    cpuLimits: string;
	    memRequests: string;
	    memLimits: string;
	    cpuAllocatable: string;
	    memAllocatable: string;
	    cpuRequestPct: number;
	    memRequestPct: number;
	    cpuLimitPct: number;
	    memLimitPct: number;
	
	    static createFrom(source: any = {}) {
	        return new NodeAllocationInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodeName = source["nodeName"];
	        this.podCount = source["podCount"];
	        this.cpuRequests = source["cpuRequests"];
	        this.cpuLimits = source["cpuLimits"];
	        this.memRequests = source["memRequests"];
	        this.memLimits = source["memLimits"];
	        this.cpuAllocatable = source["cpuAllocatable"];
	        this.memAllocatable = source["memAllocatable"];
	        this.cpuRequestPct = source["cpuRequestPct"];
	        this.memRequestPct = source["memRequestPct"];
	        this.cpuLimitPct = source["cpuLimitPct"];
	        this.memLimitPct = source["memLimitPct"];
	    }
	}
	export class PortForwardInfo {
	    id: string;
	    namespace: string;
	    pod: string;
	    localPort: number;
	    remotePort: number;
	    status: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new PortForwardInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.namespace = source["namespace"];
	        this.pod = source["pod"];
	        this.localPort = source["localPort"];
	        this.remotePort = source["remotePort"];
	        this.status = source["status"];
	        this.error = source["error"];
	    }
	}
	
	export class ProviderInfo {
	    id: string;
	    name: string;
	    requiresApiKey: boolean;
	    defaultEndpoint: string;
	    defaultModel: string;
	    models: string[];
	
	    static createFrom(source: any = {}) {
	        return new ProviderInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.requiresApiKey = source["requiresApiKey"];
	        this.defaultEndpoint = source["defaultEndpoint"];
	        this.defaultModel = source["defaultModel"];
	        this.models = source["models"];
	    }
	}
	export class RBACPermission {
	    verbs: string[];
	    resources: string[];
	    resourceNames?: string[];
	    apiGroups: string[];
	
	    static createFrom(source: any = {}) {
	        return new RBACPermission(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.verbs = source["verbs"];
	        this.resources = source["resources"];
	        this.resourceNames = source["resourceNames"];
	        this.apiGroups = source["apiGroups"];
	    }
	}
	export class RBACBinding {
	    name: string;
	    namespace?: string;
	    roleName: string;
	    roleKind: string;
	    subjects: RBACSubject[];
	    permissions: RBACPermission[];
	    isCluster: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RBACBinding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.roleName = source["roleName"];
	        this.roleKind = source["roleKind"];
	        this.subjects = this.convertValues(source["subjects"], RBACSubject);
	        this.permissions = this.convertValues(source["permissions"], RBACPermission);
	        this.isCluster = source["isCluster"];
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
	
	
	export class RBACSummary {
	    bindings: RBACBinding[];
	    subjectSummary: Record<string, Array<string>>;
	    dangerousAccess: DangerousAccessInfo[];
	
	    static createFrom(source: any = {}) {
	        return new RBACSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bindings = this.convertValues(source["bindings"], RBACBinding);
	        this.subjectSummary = source["subjectSummary"];
	        this.dangerousAccess = this.convertValues(source["dangerousAccess"], DangerousAccessInfo);
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
	export class ResourceChange {
	    kind: string;
	    name: string;
	    namespace: string;
	    oldStatus?: string;
	    newStatus?: string;
	
	    static createFrom(source: any = {}) {
	        return new ResourceChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.oldStatus = source["oldStatus"];
	        this.newStatus = source["newStatus"];
	    }
	}
	
	export class SecurityIssueInfo {
	    id: string;
	    category: string;
	    severity: string;
	    title: string;
	    description: string;
	    resource: string;
	    namespace: string;
	    kind: string;
	    remediation: string;
	    details?: Record<string, any>;
	    detectedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new SecurityIssueInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.category = source["category"];
	        this.severity = source["severity"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.resource = source["resource"];
	        this.namespace = source["namespace"];
	        this.kind = source["kind"];
	        this.remediation = source["remediation"];
	        this.details = source["details"];
	        this.detectedAt = source["detectedAt"];
	    }
	}
	export class SecurityScoreInfo {
	    overall: number;
	    grade: string;
	    categories: Record<string, number>;
	    scannedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new SecurityScoreInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.overall = source["overall"];
	        this.grade = source["grade"];
	        this.categories = source["categories"];
	        this.scannedAt = source["scannedAt"];
	    }
	}
	export class SecuritySummaryInfo {
	    score: SecurityScoreInfo;
	    totalIssues: number;
	    criticalCount: number;
	    highCount: number;
	    mediumCount: number;
	    lowCount: number;
	    issuesByCategory: Record<string, number>;
	    topIssues: SecurityIssueInfo[];
	
	    static createFrom(source: any = {}) {
	        return new SecuritySummaryInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.score = this.convertValues(source["score"], SecurityScoreInfo);
	        this.totalIssues = source["totalIssues"];
	        this.criticalCount = source["criticalCount"];
	        this.highCount = source["highCount"];
	        this.mediumCount = source["mediumCount"];
	        this.lowCount = source["lowCount"];
	        this.issuesByCategory = source["issuesByCategory"];
	        this.topIssues = this.convertValues(source["topIssues"], SecurityIssueInfo);
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
	export class SnapshotDiffResult {
	    before: string;
	    after: string;
	    added: ResourceChange[];
	    removed: ResourceChange[];
	    modified: ResourceChange[];
	
	    static createFrom(source: any = {}) {
	        return new SnapshotDiffResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.before = source["before"];
	        this.after = source["after"];
	        this.added = this.convertValues(source["added"], ResourceChange);
	        this.removed = this.convertValues(source["removed"], ResourceChange);
	        this.modified = this.convertValues(source["modified"], ResourceChange);
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
	export class SnapshotInfo {
	    timestamp: string;
	
	    static createFrom(source: any = {}) {
	        return new SnapshotInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	    }
	}
	export class TimelineEvent {
	    id: number;
	    cluster: string;
	    namespace: string;
	    kind: string;
	    name: string;
	    type: string;
	    reason: string;
	    message: string;
	    firstSeen: string;
	    lastSeen: string;
	    count: number;
	    sourceComponent: string;
	
	    static createFrom(source: any = {}) {
	        return new TimelineEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.cluster = source["cluster"];
	        this.namespace = source["namespace"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.reason = source["reason"];
	        this.message = source["message"];
	        this.firstSeen = source["firstSeen"];
	        this.lastSeen = source["lastSeen"];
	        this.count = source["count"];
	        this.sourceComponent = source["sourceComponent"];
	    }
	}
	export class TimelineFilter {
	    namespace: string;
	    kind: string;
	    type: string;
	    sinceMinutes: number;
	    limit: number;
	
	    static createFrom(source: any = {}) {
	        return new TimelineFilter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.namespace = source["namespace"];
	        this.kind = source["kind"];
	        this.type = source["type"];
	        this.sinceMinutes = source["sinceMinutes"];
	        this.limit = source["limit"];
	    }
	}

}

export namespace network {
	
	export class NetworkEdge {
	    id: string;
	    source: string;
	    target: string;
	    allowed: boolean;
	    direction: string;
	    policyName: string;
	    ports: string;
	
	    static createFrom(source: any = {}) {
	        return new NetworkEdge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.source = source["source"];
	        this.target = source["target"];
	        this.allowed = source["allowed"];
	        this.direction = source["direction"];
	        this.policyName = source["policyName"];
	        this.ports = source["ports"];
	    }
	}
	export class NetworkNode {
	    id: string;
	    name: string;
	    namespace: string;
	    kind: string;
	    labels: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new NetworkNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.kind = source["kind"];
	        this.labels = source["labels"];
	    }
	}
	export class NetworkGraph {
	    nodes: NetworkNode[];
	    edges: NetworkEdge[];
	    hasPolicies: boolean;
	    warning?: string;
	
	    static createFrom(source: any = {}) {
	        return new NetworkGraph(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodes = this.convertValues(source["nodes"], NetworkNode);
	        this.edges = this.convertValues(source["edges"], NetworkEdge);
	        this.hasPolicies = source["hasPolicies"];
	        this.warning = source["warning"];
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

export namespace rbac {
	
	export class PolicyRule {
	    verbs: string[];
	    resources: string[];
	    apiGroups: string[];
	    isWildcard: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PolicyRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.verbs = source["verbs"];
	        this.resources = source["resources"];
	        this.apiGroups = source["apiGroups"];
	        this.isWildcard = source["isWildcard"];
	    }
	}
	export class RBACBinding {
	    name: string;
	    kind: string;
	    roleName: string;
	    roleKind: string;
	    namespace: string;
	    clusterWide: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RBACBinding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.roleName = source["roleName"];
	        this.roleKind = source["roleKind"];
	        this.namespace = source["namespace"];
	        this.clusterWide = source["clusterWide"];
	    }
	}
	export class SubjectPermissions {
	    subject: string;
	    kind: string;
	    bindings: RBACBinding[];
	    rules: PolicyRule[];
	
	    static createFrom(source: any = {}) {
	        return new SubjectPermissions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.subject = source["subject"];
	        this.kind = source["kind"];
	        this.bindings = this.convertValues(source["bindings"], RBACBinding);
	        this.rules = this.convertValues(source["rules"], PolicyRule);
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
	export class RBACMatrix {
	    namespace: string;
	    subjects: SubjectPermissions[];
	    warning?: string;
	
	    static createFrom(source: any = {}) {
	        return new RBACMatrix(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.namespace = source["namespace"];
	        this.subjects = this.convertValues(source["subjects"], SubjectPermissions);
	        this.warning = source["warning"];
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

