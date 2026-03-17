// Package qdrant provides a persistent, semantically-searchable MemoryStore
// backed by a Qdrant vector database.
package qdrant

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	qdrantpb "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/lioarce01/chainforge/pkg/core"
)

// Compile-time guard: Store must satisfy core.MemoryStore.
var _ core.MemoryStore = (*Store)(nil)

// Store is a Qdrant-backed MemoryStore. Safe for concurrent use.
type Store struct {
	cfg      Config
	conn     *grpc.ClientConn
	points   qdrantpb.PointsClient
	collsvc  qdrantpb.CollectionsClient
	initOnce sync.Once
	initErr  error
	seqMu    sync.Mutex
	seqMap   map[string]uint64 // session → next sequence number
}

// New creates and connects a Qdrant-backed Store.
func New(opts ...Option) (*Store, error) {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	var dialOpts []grpc.DialOption
	if cfg.UseTLS {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, "")))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(addr, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("qdrant: connect %s: %w", addr, err)
	}

	return &Store{
		cfg:     cfg,
		conn:    conn,
		points:  qdrantpb.NewPointsClient(conn),
		collsvc: qdrantpb.NewCollectionsClient(conn),
		seqMap:  make(map[string]uint64),
	}, nil
}

// Close releases the underlying gRPC connection.
func (s *Store) Close() error {
	return s.conn.Close()
}

// -- core.MemoryStore --------------------------------------------------------

// Get retrieves the most recent TopK messages for a session, ordered oldest→newest.
func (s *Store) Get(ctx context.Context, sessionID string) ([]core.Message, error) {
	if err := s.ensureCollection(ctx); err != nil {
		return nil, err
	}
	ctx = s.withAPIKey(ctx)

	limit := uint32(s.cfg.TopK)
	resp, err := s.points.Scroll(ctx, &qdrantpb.ScrollPoints{
		CollectionName: s.cfg.CollectionName,
		Filter:         sessionFilter(sessionID),
		Limit:          &limit,
		WithPayload:    &qdrantpb.WithPayloadSelector{SelectorOptions: &qdrantpb.WithPayloadSelector_Enable{Enable: true}},
		OrderBy: &qdrantpb.OrderBy{
			Key:       "sequence_num",
			Direction: qdrantpb.Direction_Desc.Enum(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant: scroll: %w", err)
	}

	msgs := make([]core.Message, 0, len(resp.Result))
	for _, pt := range resp.Result {
		msg, err := payloadToMessage(pt.Payload)
		if err != nil {
			continue // skip corrupt points
		}
		msgs = append(msgs, msg)
	}

	// Reverse so the caller receives oldest→newest.
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

// Append embeds and upserts one or more messages into Qdrant.
func (s *Store) Append(ctx context.Context, sessionID string, msgs ...core.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	if err := s.ensureCollection(ctx); err != nil {
		return err
	}
	ctx = s.withAPIKey(ctx)

	points := make([]*qdrantpb.PointStruct, 0, len(msgs))
	for _, msg := range msgs {
		text := messageText(msg)
		if text == "" {
			continue // nothing to embed
		}

		vec, err := s.cfg.Embedder.Embed(ctx, text)
		if err != nil {
			return fmt.Errorf("qdrant: embed message: %w", err)
		}

		seq := s.nextSeq(ctx, sessionID)
		id := pointID(sessionID, seq)

		msgJSON, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("qdrant: marshal message: %w", err)
		}

		payload := map[string]*qdrantpb.Value{
			"session_id":    strVal(sessionID),
			"sequence_num":  intVal(int64(seq)),
			"role":          strVal(string(msg.Role)),
			"content":       strVal(msg.Content),
			"tool_call_id":  strVal(msg.ToolCallID),
			"name":          strVal(msg.Name),
			"message_json":  strVal(string(msgJSON)),
			"timestamp_unix": intVal(time.Now().UnixNano()),
		}

		points = append(points, &qdrantpb.PointStruct{
			Id:      &qdrantpb.PointId{PointIdOptions: &qdrantpb.PointId_Num{Num: id}},
			Vectors: &qdrantpb.Vectors{VectorsOptions: &qdrantpb.Vectors_Vector{Vector: &qdrantpb.Vector{Data: vec}}},
			Payload: payload,
		})
	}

	if len(points) == 0 {
		return nil
	}

	wait := true
	_, err := s.points.Upsert(ctx, &qdrantpb.UpsertPoints{
		CollectionName: s.cfg.CollectionName,
		Points:         points,
		Wait:           &wait,
	})
	if err != nil {
		return fmt.Errorf("qdrant: upsert: %w", err)
	}
	return nil
}

// Clear deletes all points for a session.
func (s *Store) Clear(ctx context.Context, sessionID string) error {
	if err := s.ensureCollection(ctx); err != nil {
		return err
	}
	ctx = s.withAPIKey(ctx)

	wait := true
	_, err := s.points.Delete(ctx, &qdrantpb.DeletePoints{
		CollectionName: s.cfg.CollectionName,
		Points:         &qdrantpb.PointsSelector{PointsSelectorOneOf: &qdrantpb.PointsSelector_Filter{Filter: sessionFilter(sessionID)}},
		Wait:           &wait,
	})
	if err != nil {
		return fmt.Errorf("qdrant: clear session %q: %w", sessionID, err)
	}

	s.seqMu.Lock()
	delete(s.seqMap, sessionID)
	s.seqMu.Unlock()
	return nil
}

// Search performs semantic similarity search within a session.
// This is only available on the concrete *Store type (not on core.MemoryStore).
func (s *Store) Search(ctx context.Context, sessionID, query string, topK uint64) ([]core.Message, error) {
	if err := s.ensureCollection(ctx); err != nil {
		return nil, err
	}
	ctx = s.withAPIKey(ctx)

	vec, err := s.cfg.Embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("qdrant: embed query: %w", err)
	}

	resp, err := s.points.Search(ctx, &qdrantpb.SearchPoints{
		CollectionName: s.cfg.CollectionName,
		Vector:         vec,
		Filter:         sessionFilter(sessionID),
		Limit:          topK,
		WithPayload:    &qdrantpb.WithPayloadSelector{SelectorOptions: &qdrantpb.WithPayloadSelector_Enable{Enable: true}},
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant: search: %w", err)
	}

	msgs := make([]core.Message, 0, len(resp.Result))
	for _, pt := range resp.Result {
		msg, err := payloadToMessage(pt.Payload)
		if err != nil {
			continue
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}

// -- Collection init ---------------------------------------------------------

func (s *Store) ensureCollection(ctx context.Context) error {
	s.initOnce.Do(func() {
		s.initErr = s.createCollectionIfMissing(ctx)
	})
	return s.initErr
}

func (s *Store) createCollectionIfMissing(ctx context.Context) error {
	ctx = s.withAPIKey(ctx)
	_, err := s.collsvc.Get(ctx, &qdrantpb.GetCollectionInfoRequest{
		CollectionName: s.cfg.CollectionName,
	})
	if err == nil {
		return nil // already exists
	}

	// Create the collection.
	_, err = s.collsvc.Create(ctx, &qdrantpb.CreateCollection{
		CollectionName: s.cfg.CollectionName,
		VectorsConfig: &qdrantpb.VectorsConfig{
			Config: &qdrantpb.VectorsConfig_Params{
				Params: &qdrantpb.VectorParams{
					Size:     s.cfg.Embedder.Dims(),
					Distance: qdrantpb.Distance_Cosine,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCollectionInit, err)
	}
	return nil
}

// -- Sequence numbers --------------------------------------------------------

// nextSeq returns the next monotone sequence number for sessionID.
// On first access it lazily queries Qdrant to resume from where it left off.
func (s *Store) nextSeq(ctx context.Context, sessionID string) uint64 {
	s.seqMu.Lock()
	defer s.seqMu.Unlock()

	if _, known := s.seqMap[sessionID]; !known {
		s.seqMap[sessionID] = s.loadMaxSeq(ctx, sessionID) + 1
	}
	n := s.seqMap[sessionID]
	s.seqMap[sessionID]++
	return n
}

// loadMaxSeq queries Qdrant for the maximum sequence_num stored for sessionID.
// Returns 0 if no points exist yet.
func (s *Store) loadMaxSeq(ctx context.Context, sessionID string) uint64 {
	ctx = s.withAPIKey(ctx)
	limit := uint32(1)
	resp, err := s.points.Scroll(ctx, &qdrantpb.ScrollPoints{
		CollectionName: s.cfg.CollectionName,
		Filter:         sessionFilter(sessionID),
		Limit:          &limit,
		WithPayload:    &qdrantpb.WithPayloadSelector{SelectorOptions: &qdrantpb.WithPayloadSelector_Enable{Enable: true}},
		OrderBy: &qdrantpb.OrderBy{
			Key:       "sequence_num",
			Direction: qdrantpb.Direction_Desc.Enum(),
		},
	})
	if err != nil || len(resp.Result) == 0 {
		return 0
	}
	v, ok := resp.Result[0].Payload["sequence_num"]
	if !ok {
		return 0
	}
	if iv, ok := v.Kind.(*qdrantpb.Value_IntegerValue); ok {
		return uint64(iv.IntegerValue)
	}
	return 0
}

// -- Helpers -----------------------------------------------------------------

// pointID computes a deterministic FNV-64a hash for (sessionID, seqNum).
func pointID(sessionID string, seqNum uint64) uint64 {
	h := fnv.New64a()
	h.Write([]byte(sessionID))
	h.Write([]byte(":"))
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], seqNum)
	h.Write(buf[:])
	return h.Sum64()
}

// messageText synthesises an embeddable string from a message.
func messageText(msg core.Message) string {
	if msg.Content != "" {
		return string(msg.Role) + ": " + msg.Content
	}
	// Assistant messages may have only ToolCalls and no content.
	if len(msg.ToolCalls) > 0 {
		var sb strings.Builder
		sb.WriteString("assistant: [tool_call]")
		for i, tc := range msg.ToolCalls {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(" ")
			sb.WriteString(tc.Name)
			sb.WriteString("(")
			sb.WriteString(tc.Input)
			sb.WriteString(")")
		}
		return sb.String()
	}
	return ""
}

// payloadToMessage reconstructs a core.Message from a Qdrant payload.
// It uses message_json as the authoritative source.
func payloadToMessage(payload map[string]*qdrantpb.Value) (core.Message, error) {
	v, ok := payload["message_json"]
	if !ok {
		return core.Message{}, fmt.Errorf("missing message_json")
	}
	sv, ok := v.Kind.(*qdrantpb.Value_StringValue)
	if !ok {
		return core.Message{}, fmt.Errorf("message_json is not a string")
	}
	var msg core.Message
	if err := json.Unmarshal([]byte(sv.StringValue), &msg); err != nil {
		return core.Message{}, fmt.Errorf("unmarshal message_json: %w", err)
	}
	return msg, nil
}

// sessionFilter builds a Qdrant filter that matches a single session_id.
func sessionFilter(sessionID string) *qdrantpb.Filter {
	return &qdrantpb.Filter{
		Must: []*qdrantpb.Condition{
			{
				ConditionOneOf: &qdrantpb.Condition_Field{
					Field: &qdrantpb.FieldCondition{
						Key: "session_id",
						Match: &qdrantpb.Match{
							MatchValue: &qdrantpb.Match_Keyword{Keyword: sessionID},
						},
					},
				},
			},
		},
	}
}

// withAPIKey injects the API key into outgoing gRPC metadata if set.
func (s *Store) withAPIKey(ctx context.Context) context.Context {
	if s.cfg.APIKey == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "api-key", s.cfg.APIKey)
}

// strVal wraps a string in a Qdrant Value.
func strVal(s string) *qdrantpb.Value {
	return &qdrantpb.Value{Kind: &qdrantpb.Value_StringValue{StringValue: s}}
}

// intVal wraps an int64 in a Qdrant Value.
func intVal(n int64) *qdrantpb.Value {
	return &qdrantpb.Value{Kind: &qdrantpb.Value_IntegerValue{IntegerValue: n}}
}
