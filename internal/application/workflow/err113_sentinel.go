package workflow

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrDiskGone         = errors.New("disk gone")
	ErrNatsAckTimeout   = errors.New("nats: ack timeout")
	ErrProcessorTimeout = errors.New("processor timeout")
	ErrUndoExploded     = errors.New("undo exploded")
)
