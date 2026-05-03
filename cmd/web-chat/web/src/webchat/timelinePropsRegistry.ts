export type TimelineProps = Record<string, unknown>;
export type TimelinePropsNormalizer = (props: TimelineProps) => TimelineProps;

function asString(v: unknown): string {
  return typeof v === 'string' ? v : '';
}

const builtinNormalizers: Record<string, TimelinePropsNormalizer> = {
  tool_result: (props) => {
    const resultRaw = asString(props.resultRaw);
    return {
      ...props,
      customKind: asString(props.customKind),
      result: resultRaw || props.result || '',
    };
  },
};

const extensionNormalizers = new Map<string, TimelinePropsNormalizer>();

export function registerTimelinePropsNormalizer(kind: string, normalizer: TimelinePropsNormalizer) {
  const key = String(kind || '').trim();
  if (!key) return;
  extensionNormalizers.set(key, normalizer);
}

export function unregisterTimelinePropsNormalizer(kind: string) {
  const key = String(kind || '').trim();
  if (!key) return;
  extensionNormalizers.delete(key);
}

export function clearRegisteredTimelinePropsNormalizers() {
  extensionNormalizers.clear();
}

export function normalizeTimelineProps(kind: string, props: TimelineProps): TimelineProps {
  const key = String(kind || '').trim();
  if (!key) return props;
  const normalizer = extensionNormalizers.get(key) ?? builtinNormalizers[key];
  if (!normalizer) return props;
  return normalizer(props);
}
