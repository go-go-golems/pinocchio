

export interface NowMarkerProps {
  label?: string;
}

export function NowMarker({ label = 'Live' }: NowMarkerProps) {
  return (
    <div className="now-marker">
      <div className="now-marker-line" />
      <div className="now-marker-label">
        <span className="now-marker-dot" />
        <span className="now-marker-text">{label}</span>
      </div>
    </div>
  );
}

export default NowMarker;
