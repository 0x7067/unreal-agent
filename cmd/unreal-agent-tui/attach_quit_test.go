package main

import (
	tea "charm.land/bubbletea/v2"
	"testing"
)

func TestAttachQuitFromExpandedToolWithDraft(t *testing.T) {
	m := toolsModel(t, defaultTheme, 120, 40, 1)
	m.attached = true
	m.attachConnected = true
	m.composer.SetValue("draft")
	m.openTool(0)
	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("no quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("not quit: %T", cmd())
	}
	if updated.(model).composer.Value() != "draft" {
		t.Fatal("draft lost")
	}
}
