import type React from 'react';
import { getPartProps, mergeClassName, mergeStyle } from '../parts';
import type { ChatWidgetRenderers, PartProps, RenderEntity } from '../types';

type TimelineProps = {
  entities: RenderEntity[];
  errors: Array<{ id: string; scope: string; message: string; detail?: string; extra?: unknown; time: number }>;
  showErrors: boolean;
  errorCount: number;
  onClearErrors: () => void;
  onToggleErrors: () => void;
  renderers: ChatWidgetRenderers;
  bottomRef: React.RefObject<HTMLDivElement>;
  partProps?: PartProps;
  state?: string;
};

function roleFromEntity(e: RenderEntity): string | undefined {
  if (e.kind === 'message') return String(e.props?.role ?? 'assistant');
  if (e.kind === 'tool_call' || e.kind === 'tool_result') return 'tool';
  if (
    e.kind === 'thinking_mode' ||
    e.kind === 'planning' ||
    e.kind === 'disco_dialogue_line' ||
    e.kind === 'disco_dialogue_check' ||
    e.kind === 'disco_dialogue_state'
  ) {
    return 'system';
  }
  return undefined;
}

export function ChatTimeline({
  entities,
  errors,
  showErrors,
  errorCount,
  onClearErrors,
  onToggleErrors,
  renderers,
  bottomRef,
  partProps,
  state,
}: TimelineProps) {
  const timelineProps = getPartProps('timeline', partProps);
  const timelineClassName = mergeClassName(timelineProps.className);
  const timelineStyle = mergeStyle(timelineProps.style);

  const turnProps = getPartProps('turn', partProps);
  const bubbleProps = getPartProps('bubble', partProps);
  const contentProps = getPartProps('content', partProps);

  return (
    <div
      {...timelineProps}
      data-part="timeline"
      data-state={state || undefined}
      className={timelineClassName}
      style={timelineStyle}
    >
      {showErrors && errorCount > 0 ? (
        <div data-part="error-panel">
          <div data-part="error-panel-header">
            <div data-part="row">
              <span data-part="pill" data-variant="danger">
                errors
              </span>
              <span data-part="pill">{errorCount}</span>
            </div>
            <div data-part="row">
              <button type="button" data-part="button" data-variant="ghost" onClick={onClearErrors}>
                Clear
              </button>
              <button type="button" data-part="button" data-variant="ghost" onClick={onToggleErrors}>
                Hide
              </button>
            </div>
          </div>
          <div data-part="error-panel-body">
            {errors.map((err) => (
              <div key={err.id} data-part="error-item">
                <div data-part="row">
                  <span data-part="pill" data-variant="danger">
                    {err.scope}
                  </span>
                  <span data-part="pill" data-mono="true">
                    {new Date(err.time).toLocaleTimeString()}
                  </span>
                </div>
                <div data-part="error-item-message">{err.message}</div>
                {err.detail ? <div data-part="error-item-detail" data-mono="true">{err.detail}</div> : null}
                {err.extra ? (
                  <pre data-part="error-item-detail" data-mono="true">
                    {JSON.stringify(err.extra, null, 2)}
                  </pre>
                ) : null}
              </div>
            ))}
          </div>
        </div>
      ) : null}
      {entities.map((e) => {
        const Renderer = renderers[e.kind] ?? renderers.default;
        const role = roleFromEntity(e);
        return (
          <div
            key={e.id}
            {...turnProps}
            data-part="turn"
            data-role={role}
            className={mergeClassName(turnProps.className)}
            style={mergeStyle(turnProps.style)}
          >
            <div
              {...bubbleProps}
              data-part="bubble"
              className={mergeClassName(bubbleProps.className)}
              style={mergeStyle(bubbleProps.style)}
            >
              <div
                {...contentProps}
                data-part="content"
                className={mergeClassName(contentProps.className)}
                style={mergeStyle(contentProps.style)}
              >
                {Renderer ? <Renderer e={e} /> : null}
              </div>
            </div>
          </div>
        );
      })}
      <div ref={bottomRef} />
    </div>
  );
}
