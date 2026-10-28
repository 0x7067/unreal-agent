package localfile

import (
	"context"
	"errors"
	"io"

	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

// ReadLifecycle decodes existing canonical bytes without opening paths, creating
// directories, acquiring leases, changing metadata, or caching execution state.
// The caller owns a safely opened regular-file reader and its lifetime. The same
// versioned canonical codec as Store is used, including committed-line handling.
// This bounded diagnostic operation never authorizes execution or writer takeover.
func ReadLifecycle(ctx context.Context, reader io.Reader, id session.ID) (sessionstore.LifecycleHistory, error) {
	if err := validateSessionID(id); err != nil {
		return sessionstore.LifecycleHistory{}, err
	}
	if err := context.Cause(ctx); err != nil {
		return sessionstore.LifecycleHistory{}, err
	}
	const maxBytes = 64 << 20
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return sessionstore.LifecycleHistory{}, err
	}
	if len(data) > maxBytes {
		return sessionstore.LifecycleHistory{}, errors.New("read-only lifecycle history exceeds 64MiB diagnostic bound")
	}
	if err := context.Cause(ctx); err != nil {
		return sessionstore.LifecycleHistory{}, err
	}
	state, _, err := decodeLog(data)
	if err != nil {
		return sessionstore.LifecycleHistory{}, err
	}
	if state.Snapshot.Session.ID != id {
		return sessionstore.LifecycleHistory{}, errors.New("read-only lifecycle session identity mismatch")
	}
	return sessionstore.ReadLifecycle(ctx, readOnlyItems{items: state.Items}, id)
}

type readOnlyItems struct {
	sessionstore.Store
	items []sessionstore.Item
}

func (s readOnlyItems) Items(ctx context.Context, _ session.ID, after sessionstore.Sequence, limit int) (sessionstore.Page, error) {
	if err := context.Cause(ctx); err != nil {
		return sessionstore.Page{}, err
	}
	page := sessionstore.Page{}
	for _, item := range s.items {
		if item.Sequence <= after {
			continue
		}
		if len(page.Items) == limit {
			page.More = true
			break
		}
		page.Items = append(page.Items, item)
		page.NextAfter = item.Sequence
	}
	return page, nil
}
