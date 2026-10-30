package guardian

import "errors"

type State string

const (
	Open     State = "OPEN"
	Grace    State = "GRACE"
	Safe     State = "SAFE"
	Unknown  State = "UNKNOWN"
	Distress State = "DISTRESS"
)

func (s State) Closed() bool { return s == Safe || s == Unknown || s == Distress }

var ErrClosed = errors.New("interval already closed")

type Alert struct {
	Kind State
	At   int64
}

type Interval struct {
	ID        string
	State     State
	BoundAt   int64
	ExpiresAt int64
	GraceSecs int64
	ClosedAt  int64
	Extended  int
}

func New(id string, now, windowSecs, graceSecs int64) *Interval {
	return &Interval{
		ID: id, State: Open, BoundAt: now,
		ExpiresAt: now + windowSecs, GraceSecs: graceSecs,
	}
}

func (i *Interval) Extend(now, extraSecs int64) error {
	if i.State.Closed() {
		return ErrClosed
	}
	i.ExpiresAt = max(i.ExpiresAt, now) + extraSecs
	i.State = Open
	i.Extended++
	return nil
}

func (i *Interval) Release(now int64) error {
	if i.State.Closed() {
		return ErrClosed
	}
	i.State, i.ClosedAt = Safe, now
	return nil
}

func (i *Interval) Distress(now int64) (*Alert, error) {
	if i.State.Closed() {
		return nil, ErrClosed
	}
	i.State, i.ClosedAt = Distress, now
	return &Alert{Kind: Distress, At: now}, nil
}

func (i *Interval) Tick(now int64) *Alert {
	switch i.State {
	case Open:
		if now >= i.ExpiresAt {
			i.State = Grace
		}
		fallthrough
	case Grace:
		if i.State == Grace && now >= i.ExpiresAt+i.GraceSecs {
			i.State, i.ClosedAt = Unknown, now
			return &Alert{Kind: Unknown, At: now}
		}
	}
	return nil
}
