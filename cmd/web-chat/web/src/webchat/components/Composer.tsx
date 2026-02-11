import { getPartProps, mergeClassName, mergeStyle } from '../parts';
import type { ComposerSlotProps } from '../types';

export function DefaultComposer({
  text,
  disabled,
  onChangeText,
  onSubmit,
  onNewConversation,
  onKeyDown,
  partProps,
}: ComposerSlotProps) {
  const composerProps = getPartProps('composer', partProps);
  const composerClassName = mergeClassName(composerProps.className);
  const composerStyle = mergeStyle(composerProps.style);

  const actionsProps = getPartProps('composer-actions', partProps);
  const actionsClassName = mergeClassName(actionsProps.className);
  const actionsStyle = mergeStyle(actionsProps.style);

  const inputProps = getPartProps('composer-input', partProps);
  const inputClassName = mergeClassName(inputProps.className);
  const inputStyle = mergeStyle(inputProps.style);

  return (
    <form
      {...composerProps}
      data-part="composer"
      className={composerClassName}
      style={composerStyle}
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
    >
      <div data-part="composer-inner">
        <div>
          <textarea
            {...inputProps}
            data-part="composer-input"
            className={inputClassName}
            style={inputStyle}
            value={text}
            onChange={(e) => onChangeText(e.target.value)}
            onKeyDown={onKeyDown}
            placeholder="Ask something…"
          />
          <div data-part="composer-hint">Enter to send · Shift+Enter for newline</div>
        </div>
        <div
          {...actionsProps}
          data-part="composer-actions"
          className={actionsClassName}
          style={actionsStyle}
        >
          <button type="submit" data-part="send-button" data-disabled={disabled || undefined} disabled={disabled}>
            Send
          </button>
          <button type="button" data-part="button" data-variant="ghost" onClick={onNewConversation}>
            New conv
          </button>
        </div>
      </div>
    </form>
  );
}
