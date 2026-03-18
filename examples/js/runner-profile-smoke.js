const gp = require("geppetto");

const resolved = gp.profiles.resolve({});

assert(resolved.profileSlug !== "", "expected resolved engine profile slug");
assert(
  typeof resolved.inferenceSettings?.chat?.engine === "string" && resolved.inferenceSettings.chat.engine !== "",
  "expected resolved engine profile to include an engine",
);

const localEngine = gp.engines.fromFunction((turn) => {
  const promptBlock = turn.blocks[turn.blocks.length - 1];
  const model = resolved.inferenceSettings?.chat?.engine || "<missing>";
  return gp.turns.newTurn({
    blocks: [
      gp.turns.newAssistantBlock(`profile=${resolved.profileSlug} model=${model} prompt=${promptBlock.payload.text}`),
    ],
  });
});

const out = gp.runner.run({
  engine: localEngine,
  prompt: "hello from pinocchio js",
});

console.log(out.blocks[0].payload.text);
