import type { TimelineEntity } from '../../types';

// Mock timeline entities
export const mockTimelineEntities: TimelineEntity[] = [
  {
    id: 'user-turn_01',
    kind: 'message',
    created_at: 1707229920000,
    version: 1,
    props: { role: 'user', content: 'What is the weather in Paris?', streaming: false },
  },
  {
    id: 'tc_001',
    kind: 'tool_call',
    created_at: 1707229930000,
    version: 1,
    props: { name: 'get_weather', input: { location: 'Paris' }, done: true },
  },
  {
    id: 'tr_001',
    kind: 'tool_result',
    created_at: 1707229931000,
    version: 1,
    props: { result: { temperature: 18, condition: 'cloudy' } },
  },
  {
    id: 'msg-a1b2c3d4',
    kind: 'message',
    created_at: 1707229938000,
    version: 3,
    props: { role: 'assistant', content: 'The weather in Paris is currently 18°C and cloudy.', streaming: false },
  },
];
