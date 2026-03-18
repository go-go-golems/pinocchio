const gp = require("geppetto");
const pinocchio = require("pinocchio");

const runtime = gp.runner.resolveRuntime({
  profile: { profileSlug: "assistant" },
});

assert(runtime.runtimeKey === "assistant", "expected runtime key from profile");

const defaultsEngine = pinocchio.engines.fromDefaults({
  model: "gpt-4o-mini",
  apiType: "openai",
});

const prepared = gp.runner.prepare({
  engine: defaultsEngine,
  runtime,
  prompt: "Prepare a turn with runtime metadata only.",
});

assert(prepared.turn.metadata.runtime.runtime_key === "assistant", "expected stamped runtime key");
assert(prepared.turn.metadata.runtime["profile.slug"] === "assistant", "expected stamped profile slug");

const localEngine = gp.engines.fromFunction((turn) => {
  const promptBlock = turn.blocks[turn.blocks.length - 1];
  return gp.turns.newTurn({
    blocks: [
      gp.turns.newAssistantBlock(`profile=${runtime.runtimeKey} prompt=${promptBlock.payload.text}`),
    ],
  });
});

const out = gp.runner.run({
  engine: localEngine,
  runtime,
  prompt: "hello from pinocchio js",
});

console.log(out.blocks[0].payload.text);
