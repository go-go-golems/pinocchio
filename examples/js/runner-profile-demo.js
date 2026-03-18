const gp = require("geppetto");
const pinocchio = require("pinocchio");

const runtime = gp.runner.resolveRuntime({});

console.log("resolved runtime");
console.log(JSON.stringify({
  runtimeKey: runtime.runtimeKey,
  runtimeFingerprint: runtime.runtimeFingerprint,
  profileVersion: runtime.profileVersion,
  toolNames: runtime.toolNames || [],
}, null, 2));

const engineInfo = pinocchio.engines.inspectDefaults({
  model: "gpt-4o-mini",
  apiType: "openai",
});

console.log("engine bootstrap");
console.log(JSON.stringify(engineInfo, null, 2));

const engine = pinocchio.engines.fromDefaults({
  model: "gpt-4o-mini",
  apiType: "openai",
});

const prepared = gp.runner.prepare({
  engine,
  runtime,
  prompt: "Prepare a turn with runtime metadata only.",
});

assert(prepared.turn.metadata.runtime.runtime_key === runtime.runtimeKey, "expected stamped runtime key");
assert(prepared.turn.metadata.runtime["profile.slug"] === runtime.runtimeKey, "expected stamped profile slug");

console.log("running live inference");

const out = gp.runner.run({
  engine,
  runtime,
  prompt: "Say hello in one short sentence and mention the active profile if you know it.",
});

console.log("finished run");

const last = out.blocks[out.blocks.length - 1];
assert(last && typeof last.payload?.text === "string", "expected final assistant text block");
console.log(last.payload.text);
