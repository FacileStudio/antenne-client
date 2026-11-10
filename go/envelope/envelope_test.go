package envelope

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func ptr(s string) *string { return &s }

func event[T any](app App, object ObjectType, action Action, id string, payload T) Event[T] {
	return Event[T]{
		Version:        EventVersion,
		App:            app,
		Object:         object,
		Action:         action,
		FacileID:       id,
		Payload:        payload,
		Timestamp:      "2026-11-10T09:30:00Z",
		IdempotencyKey: id + ":1",
	}
}

func golden() map[string]any {
	return map[string]any{
		"project.json": event(AppOpus, ObjectProject, ActionCreated, "fac_prj_1", Project{
			FacileID: "fac_prj_1", Name: "Launch", Description: ptr("Ship v1"), Icon: nil,
		}),
		"task.json": event(AppOpus, ObjectTask, ActionUpdated, "fac_tsk_1", Task{
			FacileID: "fac_tsk_1", ProjectFacileID: "fac_prj_1", Name: "Write docs", Status: "doing",
		}),
		"user.json": event(AppSablier, ObjectUser, ActionCreated, "fac_usr_1", User{
			FacileID: "fac_usr_1", Email: "ada@example.com", Name: "Ada",
		}),
		"time_entry.json": event(AppSablier, ObjectTimeEntry, ActionCreated, "fac_te_1", TimeEntry{
			FacileID: "fac_te_1", TaskFacileID: "fac_tsk_1", UserFacileID: "fac_usr_1",
			UserEmail: "ada@example.com", StartedAt: "2026-11-10T08:00:00Z", StoppedAt: nil,
		}),
		"agent_session.json": event(AppMycelium, ObjectAgentSession, ActionCreated, "fac_as_1", AgentSession{
			FacileID: "fac_as_1", Project: "Sonde", Machine: "ruche", Agent: "claude",
			UserEmail: "ada@example.com", StartedAt: "2026-11-10T08:00:00Z", StoppedAt: "2026-11-10T09:00:00Z",
			TokensIn: 1200, TokensOut: 340,
		}),
		"invoice.json": event(AppArdoise, ObjectInvoice, ActionCreated, "fac_inv_1", Invoice{
			FacileID: "fac_inv_1", ClientName: "Acme", Amount: 1250.5, Currency: "EUR", Status: "sent",
		}),
		"document.json": event(AppPlume, ObjectDocument, ActionUpdated, "fac_doc_1", Document{
			FacileID: "fac_doc_1", Title: "Contract", Status: "signed", SignerEmail: "ada@example.com",
		}),
		"monitor.json": event(AppSonde, ObjectMonitor, ActionUpdated, "fac_mon_1", Monitor{
			MonitorID: "42", FacileID: "fac_mon_1", Slug: "api", Name: "API", Status: "down",
			LatencyMS: 812, Error: ptr("connection refused"),
		}),
	}
}

func TestGolden(t *testing.T) {
	cases := golden()
	files, err := filepath.Glob(filepath.Join("testdata", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(cases) {
		t.Fatalf("testdata has %d fixtures, test has %d cases", len(files), len(cases))
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, bytes.TrimRight(want, "\n")) {
				t.Errorf("marshal mismatch\n got: %s\nwant: %s", got, want)
			}
		})
	}
}
