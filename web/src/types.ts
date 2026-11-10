export interface PortMapping {
  target: number;
  published?: number;
  host_ip?: string;
  protocol?: string;
  raw: string;
}

export interface ServiceNodeData {
  [key: string]: unknown;
  id: string;
  name: string;
  image?: string;
  container_name?: string;
  ports?: PortMapping[];
  networks?: string[];
  volumes?: string[];
  profiles?: string[];
  has_healthcheck: boolean;
  user?: string;
  privileged: boolean;
  in_degree: number;
  out_degree: number;
  metrics?: {
    cpu_percentage: number;
    memory_usage_mb: number;
    memory_limit_mb: number;
    memory_percent: number;
    state?: string;
    health?: string;
  };
  hasPortCollision?: boolean;
}

export interface GraphEdge {
  from: string;
  to: string;
  condition: 'service_started' | 'service_healthy' | 'service_completed_successfully';
  type: 'depends_on' | 'network_mode' | 'volumes_from';
}

export interface GraphData {
  nodes: Record<string, ServiceNodeData>;
  edges: GraphEdge[];
}

export interface StartupPlan {
  stages: string[][];
  order: string[];
  shutdown_order: string[];
}

export interface LinterFinding {
  rule_id: string;
  severity: 'CRITICAL' | 'WARNING' | 'SECURITY';
  service?: string;
  message: string;
  detail?: string;
  recommendation: string;
  suggested_override?: string;
}

export interface LinterResult {
  findings: LinterFinding[];
  critical_count: number;
  warning_count: number;
  security_count: number;
  passed: boolean;
}

export interface SystemStatus {
  docker_connected: boolean;
  docker_host: string;
  docker_error?: string;
  compose_files: string[];
  all_profiles: string[];
  active_profiles: string[];
  project_name?: string;
  service_count: number;
}

export interface LogLine {
  service: string;
  stream: 'stdout' | 'stderr';
  timestamp: string;
  message: string;
}
