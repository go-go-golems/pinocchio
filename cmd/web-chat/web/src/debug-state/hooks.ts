import { type TypedUseSelectorHook, useDispatch, useSelector } from 'react-redux';
import type { DebugDispatch, DebugRootState } from './store';

export const useDebugDispatch = () => useDispatch<DebugDispatch>();
export const useDebugSelector: TypedUseSelectorHook<DebugRootState> = useSelector;
