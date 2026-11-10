import { describe, expect, test } from "bun:test";
import { readdirSync, readFileSync } from "fs";
import { join } from "path";
import {
  allChannels,
  FACILE_EVENT_VERSION,
  type FacileAction,
  type FacileAgentSession,
  type FacileApp,
  type FacileCalendar,
  type FacileCalendarEvent,
  type FacileDocument,
  type FacileEvent,
  type FacileInvoice,
  type FacileMonitor,
  type FacileObjectType,
  type FacileProject,
  type FacileTask,
  type FacileTimeEntry,
  type FacileUser,
} from "../src/index.js";

const goRoot = join(import.meta.dir, "..", "..", "go");
const fixtures = join(goRoot, "envelope", "testdata");

function event<T>(app: FacileApp, object: FacileObjectType, action: FacileAction, id: string, payload: T): FacileEvent<T> {
  return {
    version: FACILE_EVENT_VERSION,
    app,
    object,
    action,
    facile_id: id,
    payload,
    timestamp: "2026-11-10T09:30:00Z",
    idempotency_key: `${id}:1`,
  };
}

const golden: Record<string, FacileEvent> = {
  "project.json": event<FacileProject>("Opus", "project", "created", "fac_prj_1", {
    facile_id: "fac_prj_1",
    name: "Launch",
    description: "Ship v1",
    icon: null,
  }),
  "task.json": event<FacileTask>("Opus", "task", "updated", "fac_tsk_1", {
    facile_id: "fac_tsk_1",
    project_facile_id: "fac_prj_1",
    name: "Write docs",
    status: "doing",
  }),
  "user.json": event<FacileUser>("Sablier", "user", "created", "fac_usr_1", {
    facile_id: "fac_usr_1",
    email: "ada@example.com",
    name: "Ada",
  }),
  "time_entry.json": event<FacileTimeEntry>("Sablier", "time_entry", "created", "fac_te_1", {
    facile_id: "fac_te_1",
    task_facile_id: "fac_tsk_1",
    user_facile_id: "fac_usr_1",
    user_email: "ada@example.com",
    started_at: "2026-11-10T08:00:00Z",
    stopped_at: null,
  }),
  "agent_session.json": event<FacileAgentSession>("Mycelium", "agent_session", "created", "fac_as_1", {
    facile_id: "fac_as_1",
    project: "Sonde",
    machine: "ruche",
    agent: "claude",
    user_email: "ada@example.com",
    started_at: "2026-11-10T08:00:00Z",
    stopped_at: "2026-11-10T09:00:00Z",
    tokens_in: 1200,
    tokens_out: 340,
  }),
  "invoice.json": event<FacileInvoice>("Ardoise", "invoice", "created", "fac_inv_1", {
    facile_id: "fac_inv_1",
    client_name: "Acme",
    amount: 1250.5,
    currency: "EUR",
    status: "sent",
  }),
  "document.json": event<FacileDocument>("Plume", "document", "updated", "fac_doc_1", {
    facile_id: "fac_doc_1",
    title: "Contract",
    status: "signed",
    signer_email: "ada@example.com",
  }),
  "monitor.json": event<FacileMonitor>("Sonde", "monitor", "updated", "fac_mon_1", {
    monitor_id: "42",
    facile_id: "fac_mon_1",
    slug: "api",
    name: "API",
    status: "down",
    latency_ms: 812,
    error: "connection refused",
  }),
  "calendar.json": event<FacileCalendar>("Agenda", "calendar", "created", "fac_cal_1", {
    facile_id: "fac_cal_1",
    name: "Work",
    owner_email: "ada@example.com",
    actor_email: "ada@example.com",
  }),
  "calendar_event.json": event<FacileCalendarEvent>("Agenda", "calendar_event", "updated", "fac_evt_1", {
    facile_id: "fac_evt_1",
    calendar_facile_id: "fac_cal_1",
    title: "Standup",
    starts_at: "2026-11-12T09:00:00Z",
    ends_at: "2026-11-12T09:15:00Z",
    all_day: false,
  }),
};

describe("envelope parity with go/envelope/testdata", () => {
  test("every Go fixture has a TypeScript case", () => {
    const files = readdirSync(fixtures).filter((f) => f.endsWith(".json")).sort();
    expect(files).toEqual(Object.keys(golden).sort());
  });

  for (const [name, value] of Object.entries(golden)) {
    test(name, () => {
      const want = readFileSync(join(fixtures, name), "utf8").trimEnd();
      expect(JSON.stringify(value)).toBe(want);
    });
  }
});

describe("catalog parity with go/catalog", () => {
  test("same channels in the same order", () => {
    const source = readFileSync(join(goRoot, "catalog", "catalog.go"), "utf8");
    const goChannels = [...source.matchAll(/^\s+\w+ Channel = "([^"]+)"$/gm)].map((m) => m[1]);
    expect(goChannels.length).toBeGreaterThan(0);
    expect(allChannels() as string[]).toEqual(goChannels);
  });
});
