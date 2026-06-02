const gp = require("geppetto");

const settings = gp.inferenceProfiles.resolve();
const snapshot = settings.toJSON();

console.log("resolved engine profile");
console.log(JSON.stringify({
  registrySlug: snapshot.provenance?.registrySlug || null,
  profileSlug: snapshot.provenance?.profileSlug || null,
  model: snapshot.chat?.engine || null,
  apiType: snapshot.chat?.api_type || null,
}, null, 2));

const agent = gp.agent()
  .name("pinocchio-js-demo")
  .inference(settings)
  .build();
const session = agent.session().id("pinocchio-js-demo").build();

console.log("running live inference");

const result = session.next()
  .system("Answer in one short sentence.")
  .user("Say hello in one short sentence.")
  .run({ timeoutMs: 120000 });

console.log("finished run");

const text = String(result.text() || "").trim();
assert(text !== "", "expected final assistant text");
console.log(text);
