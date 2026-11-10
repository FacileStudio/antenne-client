/** Every Antenne bus channel the suite emits or listens to. Mirrors go/catalog. */
export const Channels = {
    TaskCreated: "task.created",
    TaskUpdated: "task.updated",
    TaskDeleted: "task.deleted",
    TaskStatusChanged: "task.status_changed",
    TaskActorChanged: "task.actor_changed",
    ProjectCreated: "project.created",
    ProjectUpdated: "project.updated",
    ProjectDeleted: "project.deleted",
    TimeEntryCreated: "time_entry.created",
    TimeEntryUpdated: "time_entry.updated",
    TimeEntryDeleted: "time_entry.deleted",
    CommentCreated: "comment.created",
    WorkspaceCreated: "workspace.created",
    AgentSessionCreated: "agent_session.created",
    AgentSessionUpdated: "agent_session.updated",
    UsageAlertCreated: "usage_alert.created",
    AlertFired: "alert.fired",
    MonitorDown: "monitor.down",
    MonitorUp: "monitor.up",
    GitPush: "git.push",
    GitPullRequest: "git.pull_request",
    GitIssue: "git.issue",
    GitEvent: "git.event",
    FiletCheckFailed: "filet.check.failed",
    FiletCheckPassed: "filet.check.passed",
    CalendarCreated: "calendar.created",
    CalendarUpdated: "calendar.updated",
    CalendarDeleted: "calendar.deleted",
    CalendarEventCreated: "calendar_event.created",
    CalendarEventUpdated: "calendar_event.updated",
    CalendarEventDeleted: "calendar_event.deleted",
};
/** Every channel in the catalog, in declaration order. */
export function allChannels() {
    return Object.values(Channels);
}
