// @go-go-golems/chatapp-client: the browser client of pinocchio's chatapp HTTP
// API (pkg/chatapp/httpapi). Its built file is embedded in the Go module and
// served at {prefix}/client.js, so a page always runs the client of the server
// it talks to.
//
//	const { Session } = await import("/v1/client.js");
//	const s = await Session.create("/v1", { labels: { preset: "coder" } });
//	s.on(() => render(s.timeline.answers()));
//	const id = await s.send("hello");
//	const answer = await s.wait(id);
export { ChatError, readSSE, Session } from "./session.ts";
export type { BrowserTool, Options, SessionInfo } from "./session.ts";
export { parentOf, TERMINAL, Timeline } from "./timeline.ts";
export type { Answer, Entity, Part, Payload, Question, Snapshot, SnapshotEntity, TextPart, ToolPart, Turn, UIEvent, Usage } from "./timeline.ts";
