package webchat

import "sync"

type semBufferedFrame struct {
	raw []byte
}

type semFrameBuffer struct {
	mu     sync.Mutex
	max    int
	frames []semBufferedFrame
}

func newSemFrameBuffer(limit int) *semFrameBuffer {
	if limit <= 0 {
		limit = 1000
	}
	return &semFrameBuffer{max: limit, frames: make([]semBufferedFrame, 0, limit)}
}

func (b *semFrameBuffer) Add(frame []byte) {
	if b == nil || len(frame) == 0 {
		return
	}
	cp := make([]byte, len(frame))
	copy(cp, frame)

	b.mu.Lock()
	defer b.mu.Unlock()

	b.frames = append(b.frames, semBufferedFrame{raw: cp})
	if len(b.frames) > b.max {
		drop := len(b.frames) - b.max
		b.frames = append([]semBufferedFrame(nil), b.frames[drop:]...)
	}
}

func (b *semFrameBuffer) Snapshot() [][]byte {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([][]byte, 0, len(b.frames))
	for _, f := range b.frames {
		if len(f.raw) == 0 {
			continue
		}
		cp := make([]byte, len(f.raw))
		copy(cp, f.raw)
		out = append(out, cp)
	}
	return out
}
