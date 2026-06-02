#!/usr/bin/env node
import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(here, '../../../..');
const statePath = resolve(repoRoot, '.devctl/state.json');

function main() {
  let state;
  try {
    state = JSON.parse(readFileSync(statePath, 'utf8'));
  } catch (err) {
    console.error(`Could not read devctl state at ${statePath}.`);
    console.error('Run `cd ../../../.. && devctl up --force` first, or start devctl from the Pinocchio repo root.');
    process.exit(1);
  }

  const services = Array.isArray(state.services) ? state.services : [];
  const vite = services.find((service) => service?.name === 'vite');
  const backend = services.find((service) => service?.name === 'backend');

  if (!vite?.health_url) {
    console.error('No Vite service URL found in devctl state.');
    process.exit(1);
  }

  console.log(`web-chat: ${vite.health_url}`);
  if (backend?.health_url) console.log(`backend:  ${backend.health_url}`);
}

main();
