export type EgressType = 'direct' | 'egress_awg';

export interface Peer {
  id: number;
  name: string;
  public_key: string;
  preshared_key?: string | null;
  allowed_ips: string;
  enabled: boolean;
  egress_type: EgressType;
  egress_tunnel_id?: number | null;
  created_at: string;
  updated_at: string;
  status?: PeerStatus | null;
}

export interface PeerStatus {
  public_key: string;
  endpoint?: string;
  latest_handshake_unix: number;
  transfer_rx_bytes: number;
  transfer_tx_bytes: number;
  persistent_keepalive?: number;
}

export interface EgressTunnel {
  id: number;
  name: string;
  interface_name: string;
  config_text: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
  status?: EgressTunnelStatus | null;
}

export interface EgressTunnelStatus {
  interface_up: boolean;
  latest_handshake_unix: number;
  transfer_rx_bytes: number;
  transfer_tx_bytes: number;
  last_error?: string;
}

export interface IngressSettings {
  listen_port: number;
  host_endpoint: string;
  tunnel_subnet: string;
  dns_servers: string;
  mtu: number;
  interface_name: string;
  server_tunnel_ip: string;
  server_public_key: string;
  jc: number;
  jmin: number;
  jmax: number;
  s1: string;
  s2: string;
  s3: string;
  s4: string;
  h1: number;
  h2: number;
  h3: number;
  h4: number;
}

export interface SystemSettings {
  external_interface: string;
  tunnel_offline_policy: 'block' | 'ignore';
  client_allowed_ipv4: string;
  client_allowed_ipv6: string;
  updated_at: string;
}

export interface SetupStatus {
  setup_complete: boolean;
}

export interface SystemStatus {
  time: string;
  policy_routing?: { os: string; backend: string; last_error?: string };
  wireguard?: { quick_bin: string; awg_bin: string; config_dir: string; note?: string };
  ingress?: {
    interface_name: string;
    listen_port: number;
    peers?: Record<string, PeerStatus>;
    peers_error?: string;
  };
  egress_tunnels?: Array<{
    id: number;
    name: string;
    interface_name: string;
    enabled: boolean;
    status?: EgressTunnelStatus;
  }>;
}
