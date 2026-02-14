import React from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useGetTurnDetailQuery } from '../api/debugApi';
import { TurnInspector } from '../components/TurnInspector';
import { useAppSelector } from '../store/hooks';

export function TurnDetailPage() {
  const { sessionId, turnId } = useParams();
  const navigate = useNavigate();
  const selectedConvId = useAppSelector((state) => state.ui.selectedConvId);

  const { data: turnDetail, isLoading, error } = useGetTurnDetailQuery(
    { 
      convId: selectedConvId ?? '', 
      sessionId: sessionId ?? '', 
      turnId: turnId ?? '' 
    },
    { skip: !selectedConvId || !sessionId || !turnId }
  );

  if (!selectedConvId || !sessionId || !turnId) {
    return (
      <div className="turn-detail-empty-state">
        <h2>Turn Not Found</h2>
        <p>Missing conversation, session, or turn ID.</p>
        <button className="btn" onClick={() => navigate('/')}>
          Go Back
        </button>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="turn-detail-loading-state">
        <p>Loading turn...</p>
      </div>
    );
  }

  if (error || !turnDetail) {
    return (
      <div className="turn-detail-empty-state">
        <h2>Failed to load turn</h2>
        <p>Could not load turn details.</p>
        <button className="btn" onClick={() => navigate(-1)}>
          Go Back
        </button>
      </div>
    );
  }

  return (
    <div className="turn-detail-page">
      <div className="turn-detail-page-header">
        <button className="btn btn-ghost" onClick={() => navigate(-1)}>
          ← Back
        </button>
        <h2>Turn: {turnId}</h2>
      </div>

      <TurnInspector turnDetail={turnDetail} />
    </div>
  );
}

export default TurnDetailPage;
