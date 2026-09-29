// Package tool defines model-visible tool selection and pure translation
// between model tool calls and durable operations.
package tool

import (
	"encoding/json/jsontext"
	"uuid"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
)

type CallStatus struct {
	Error          string
	ErrorTruncated bool           `json:",omitzero"`
	WaitingFor     []operation.ID `json:",omitzero"`
}

// Context is turn-local and coordinator-owned. Submit allocates an ID and
// records inert data without performing I/O or handing work to another queue.
type Context interface {
	Submit(operation.Spec) operation.ID
}

// Result is a tool-owned value. By convention, a concrete result's Error field
// describes a model-visible failure and may accompany partial output.
type Result interface {
	ToLLMResult() llm.ToolResult
}

type ResultTranslator interface {
	TranslateResult(string, CallStatus, []operation.Operation) (Result, error)
}

type Translator interface {
	ResultTranslator
	Translate(Context, llm.ToolCall) CallStatus
}

type Definition struct {
	Tool     llm.Tool
	Metadata jsontext.Value
}

type RegistrationID = uuid.UUID

type Registry interface {
	StaticDefinitions() []Definition
	Resolve(string) (Translator, bool)
	RegisterSkill(Skill) (RegistrationID, error)
	UnregisterSkill(RegistrationID)
	Skills() []Skill
}

type Skill struct {
	Name        string
	Description string
	Path        string
}
