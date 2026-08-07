export interface AntenneConfig {
    app: string;
    instance: string;
    secret: string;
    instance_id?: string;
    events?: {
        emit?: string[];
        listen?: string[];
    };
}
export interface AntenneManifest {
    app: string;
    instance?: string;
    secret?: string;
    instance_id?: string;
    events?: {
        emit?: string[];
        listen?: string[];
    };
}
export interface AntenneClientOptions {
    config?: AntenneConfig;
    configPath?: string;
    onConnect?: () => void;
    onDisconnect?: () => void;
    onError?: (error: Error) => void;
    reconnect?: boolean;
    maxReconnectAttempts?: number;
}
export interface EventMeta {
    id: string;
    channel: string;
    offset: number;
    timestamp: number;
    sender: string;
}
type EventHandler<T = unknown> = (payload: T, meta: EventMeta) => void | Promise<void>;
type Unsubscribe = () => void;
export declare function loadConfig(path?: string): AntenneConfig;
export declare function loadManifest(path?: string): AntenneManifest;
export declare class AntenneClient {
    private config;
    private ws;
    private handlers;
    private offsets;
    private epoch;
    private appId;
    private token;
    private connected;
    private reconnecting;
    private reconnectAttempt;
    private shouldReconnect;
    private maxReconnectAttempts;
    private _onConnect?;
    private _onDisconnect?;
    private _onError?;
    private pendingMessages;
    private reconnectTimer;
    private pingTimer;
    constructor(options: AntenneClientOptions);
    connect(): Promise<void>;
    disconnect(): Promise<void>;
    emit<T = unknown>(channel: string, payload: T): void;
    emitNow<T = unknown>(channel: string, payload: T): boolean;
    listen<T = unknown>(channel: string, handler: EventHandler<T>): Unsubscribe;
    get isConnected(): boolean;
    identity(): string;
    get currentEpoch(): string;
    private register;
    private openWebSocket;
    private handleMessage;
    private startPinging;
    private stopPinging;
    private scheduleReconnect;
}
export {};
