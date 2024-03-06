package event

import (
	"context"
	"fmt"

	. "github.com/afeish/hugo/global" //lint:ignore ST1001 ignore
	pb "github.com/afeish/hugo/pb/meta"
	"github.com/afeish/hugo/pkg/store"

	"google.golang.org/protobuf/proto"
)

var _ store.Serializer[Event] = (*Event)(nil)

// Event is the event of meta modification.
// FIXME: event id should be implemented in a better way or rough one, like tso or global unique counter.
// TODO: to implement rollback, we may need to persist the event body.
type Event struct {
	store.UnimplementedSerializer
	pb.Event
}

func (e *Event) FormatKey() string {
	return fmt.Sprintf("%s%d", e.FormatPrefix(), e.Id)
}
func (e *Event) FormatPrefix() string {
	return "evt/"
}
func (e *Event) Serialize() ([]byte, error) {
	return proto.Marshal(e)
}
func (e *Event) Deserialize(bytes []byte) (*Event, error) {
	var tmp Event
	if err := proto.Unmarshal(bytes, &tmp); err != nil {
		return nil, err
	}
	return &tmp, nil
}
func (e *Event) Self() *Event {
	return e
}

func NewEvent(t pb.Event_Type, pi, ci Ino, name string) *Event {
	return &Event{
		Event: pb.Event{
			Id:        NextSnowID(),
			Type:      t,
			ParentIno: pi,
			CurIno:    ci,
			Name:      name,
		},
	}
}

// Service is used for tracing all modifications to the metadata.
type Service struct {
	eventStore store.Store
}

func NewService(s store.Store) (*Service, error) {
	return &Service{
		eventStore: s,
	}, nil
}

func (s *Service) SaveEvent(ctx context.Context, event *Event) error {
	return store.DoSaveInKVPair[Event](ctx, s.eventStore, event)
}

func (s *Service) PullLatest(ctx context.Context, request *pb.PullLatest_Request) (*pb.PullLatest_Response, error) {
	qe := &Event{
		Event: pb.Event{
			Id: request.CurrentEventId,
		},
	}
	entries, err := s.eventStore.Scan(ctx, qe.FormatPrefix(), qe.FormatKey(), "")
	if err != nil {
		return nil, err
	}
	if len(entries) == 1 {
		return &pb.PullLatest_Response{}, nil
	}
	var events []*pb.Event
	for _, e := range entries {
		event, err := qe.Deserialize(e.Value)
		if err != nil {
			return nil, err
		}
		events = append(events, &event.Event)
	}
	return &pb.PullLatest_Response{
		Events: events,
	}, nil
}
