const gp = require("geppetto");

const settings = gp.inferenceProfiles.resolve();
const snapshot = settings.toJSON();
const agent = gp.agent()
  .name("pinocchio-js-smoke")
  .inference(settings)
  .build();
const session = agent.session().id("pinocchio-js-smoke").build();

assert(typeof agent.session === "function", "expected session-capable agent");
assert(typeof session.next === "function", "expected session.next()");
assert(typeof snapshot.chat?.engine === "string" && snapshot.chat.engine !== "", "expected resolved engine profile to include an engine");

console.log(JSON.stringify({
  profile: snapshot.provenance?.profileSlug || "",
  registry: snapshot.provenance?.registrySlug || "",
  model: snapshot.chat.engine,
  session: session.id(),
}, null, 2));
