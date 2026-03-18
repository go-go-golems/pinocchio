const gp = require("geppetto");

const resolved = gp.profiles.resolve({});

console.log("resolved engine profile");
console.log(JSON.stringify({
  registrySlug: resolved.registrySlug,
  profileSlug: resolved.profileSlug,
  model: resolved.inferenceSettings?.chat?.engine || null,
  apiType: resolved.inferenceSettings?.chat?.api_type || null,
}, null, 2));

const engine = gp.engines.fromResolvedProfile(resolved);

console.log("running live inference");

const out = gp.runner.run({
  engine,
  prompt: "Say hello in one short sentence.",
});

console.log("finished run");

const last = out.blocks[out.blocks.length - 1];
assert(last && typeof last.payload?.text === "string", "expected final assistant text block");
console.log(last.payload.text);
