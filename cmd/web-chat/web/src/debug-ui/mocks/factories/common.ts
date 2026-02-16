export function cloneMock<T>(value: T): T {
  if (typeof structuredClone === 'function') {
    return structuredClone(value);
  }

  return JSON.parse(JSON.stringify(value)) as T;
}

export function pickByIndex<T>(items: readonly T[], index = 0): T {
  if (items.length === 0) {
    throw new Error('Cannot pick from an empty fixture list');
  }

  const normalizedIndex = ((Math.trunc(index) % items.length) + items.length) % items.length;
  return cloneMock(items[normalizedIndex]);
}
