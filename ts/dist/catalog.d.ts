/** Every Antenne bus channel the suite emits or listens to. Mirrors go/catalog. */
export declare const Channels: {
    readonly TaskCreated: "task.created";
    readonly TaskUpdated: "task.updated";
    readonly TaskDeleted: "task.deleted";
    readonly TaskStatusChanged: "task.status_changed";
    readonly TaskActorChanged: "task.actor_changed";
    readonly ProjectCreated: "project.created";
    readonly ProjectUpdated: "project.updated";
    readonly ProjectDeleted: "project.deleted";
    readonly TimeEntryCreated: "time_entry.created";
    readonly TimeEntryUpdated: "time_entry.updated";
    readonly TimeEntryDeleted: "time_entry.deleted";
    readonly CommentCreated: "comment.created";
    readonly WorkspaceCreated: "workspace.created";
    readonly AgentSessionCreated: "agent_session.created";
    readonly AgentSessionUpdated: "agent_session.updated";
    readonly UsageAlertCreated: "usage_alert.created";
    readonly AlertFired: "alert.fired";
    readonly MonitorDown: "monitor.down";
    readonly MonitorUp: "monitor.up";
    readonly GitPush: "git.push";
    readonly GitPullRequest: "git.pull_request";
    readonly GitIssue: "git.issue";
    readonly GitEvent: "git.event";
    readonly FiletCheckFailed: "filet.check.failed";
    readonly FiletCheckPassed: "filet.check.passed";
    readonly CalendarCreated: "calendar.created";
    readonly CalendarUpdated: "calendar.updated";
    readonly CalendarDeleted: "calendar.deleted";
    readonly CalendarEventCreated: "calendar_event.created";
    readonly CalendarEventUpdated: "calendar_event.updated";
    readonly CalendarEventDeleted: "calendar_event.deleted";
};
/** The name of one Antenne bus channel, as in "task.created". */
export type Channel = (typeof Channels)[keyof typeof Channels];
/** Every channel in the catalog, in declaration order. */
export declare function allChannels(): Channel[];
