export type FacileApp = "Sablier" | "Opus" | "Ardoise" | "Plume" | "Glouton" | "Vision" | "Mycelium" | "Sonde" | "Agenda" | "Journal" | "Courrier" | "Nuage" | "Bulle" | "Antenne";
export type FacileAction = "created" | "updated" | "deleted";
export type FacileObjectType = "project" | "task" | "user" | "time_entry" | "invoice" | "document" | "agent_session" | "monitor" | "calendar" | "calendar_event";
export interface FacileEvent<T = unknown> {
    version: number;
    app: FacileApp;
    object: FacileObjectType;
    action: FacileAction;
    facile_id: string;
    payload: T;
    timestamp: string;
    idempotency_key: string;
}
export declare const FACILE_EVENT_VERSION = 1;
export interface FacileProject {
    facile_id: string;
    name: string;
    description: string | null;
    icon: string | null;
}
export interface FacileTask {
    facile_id: string;
    project_facile_id: string;
    name: string;
    status: string;
    actor_email?: string;
}
export interface FacileUser {
    facile_id: string;
    email: string;
    name: string;
}
export interface FacileTimeEntry {
    facile_id: string;
    task_facile_id: string;
    user_facile_id: string;
    user_email?: string;
    started_at: string;
    stopped_at: string | null;
}
export interface FacileAgentSession {
    facile_id: string;
    project: string;
    machine: string;
    agent: string;
    branch?: string;
    user_email: string;
    started_at: string;
    stopped_at: string;
    tokens_in: number;
    tokens_out: number;
}
export interface FacileInvoice {
    facile_id: string;
    client_name: string;
    amount: number;
    currency: string;
    status: string;
}
export interface FacileDocument {
    facile_id: string;
    title: string;
    status: string;
    signer_email: string;
}
export interface FacileMonitor {
    monitor_id: string;
    facile_id: string;
    slug: string;
    name: string;
    status: "up" | "down";
    latency_ms: number;
    error: string | null;
}
export interface FacileCalendar {
    facile_id: string;
    name: string;
    owner_email?: string;
    actor_email?: string;
}
export interface FacileCalendarEvent {
    facile_id: string;
    calendar_facile_id: string;
    title: string;
    starts_at: string;
    ends_at: string;
    all_day: boolean;
    actor_email?: string;
}
