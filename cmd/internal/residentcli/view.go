package residentcli

import "github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"

// CLI and TUI share the same pure public-protocol integrity checks.
type view struct{ state *attachproto.View }

func newView(hello attachproto.Event) *view          { return &view{state: attachproto.NewView(hello)} }
func (v *view) accept(event attachproto.Event) error { return v.state.Accept(event) }
