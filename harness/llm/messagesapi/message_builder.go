package messagesapi

import (
	"encoding/json/jsontext"
	"fmt"
	"maps"
	"slices"

	"github.com/unreallabsai/unreal-agent/internal/apijson"
)

type messageBuilder struct {
	fields map[string]any
	blocks map[int]*blockContentBuilder
}

func newMessageBuilder(raw jsontext.Value) (*messageBuilder, error) {
	var fields map[string]any
	if err := apijson.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("decode message_start: %w", err)
	}
	if fields == nil {
		return nil, fmt.Errorf("message_start requires a message object")
	}
	return &messageBuilder{fields: fields, blocks: make(map[int]*blockContentBuilder)}, nil
}

func (message *messageBuilder) update(raw, usage jsontext.Value) error {
	if len(raw) != 0 {
		var delta map[string]any
		if err := apijson.Unmarshal(raw, &delta); err != nil {
			return fmt.Errorf("decode message_delta: %w", err)
		}
		for name, value := range delta {
			if value != nil {
				message.fields[name] = value
			}
		}
	}
	if len(usage) != 0 && usage.Kind() != jsontext.KindNull {
		var delta map[string]any
		if err := apijson.Unmarshal(usage, &delta); err != nil {
			return fmt.Errorf("decode usage delta: %w", err)
		}
		message.fields["usage"] = mergeUsage(message.fields["usage"], delta)
	}
	return nil
}

func (message *messageBuilder) startBlock(index int, raw jsontext.Value) error {
	if _, exists := message.blocks[index]; exists {
		return fmt.Errorf("content block %d already exists", index)
	}
	block, err := newBlockContentBuilder(raw)
	if err != nil {
		return fmt.Errorf("content block %d: %w", index, err)
	}
	message.blocks[index] = block
	return nil
}

func (message *messageBuilder) build() ([]byte, error) {
	content := make([]map[string]any, 0, len(message.blocks))
	for _, index := range slices.Sorted(maps.Keys(message.blocks)) {
		builder := message.blocks[index]
		if !builder.finalized {
			continue
		}
		block, err := builder.build()
		if err != nil {
			return nil, fmt.Errorf("content block %d: %w", index, err)
		}
		content = append(content, block)
	}
	message.fields["content"] = content
	return apijson.Marshal(message.fields)
}

func mergeUsage(previous any, update map[string]any) map[string]any {
	fields, _ := previous.(map[string]any)
	if fields == nil {
		fields = make(map[string]any)
	}
	for name, value := range update {
		if value == nil {
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			fields[name] = mergeUsage(fields[name], nested)
		} else {
			fields[name] = value
		}
	}
	return fields
}
