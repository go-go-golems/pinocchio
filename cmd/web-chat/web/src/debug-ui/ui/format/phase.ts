const PHASE_LABELS: Record<string, { label: string; short: string }> = {
  draft: { label: 'Draft', short: 'Draft' },
  pre_inference: { label: 'Pre-Inference', short: 'Pre' },
  post_inference: { label: 'Post-Inference', short: 'Post' },
  post_tools: { label: 'Post-Tools', short: 'Tools' },
  final: { label: 'Final', short: 'Final' },
};

export function formatPhaseLabel(phase: string): string {
  return PHASE_LABELS[phase]?.label ?? phase;
}

export function formatPhaseShort(phase: string): string {
  return PHASE_LABELS[phase]?.short ?? phase;
}
