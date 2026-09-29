package viewimage

import (
	"encoding/json/v2"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

const (
	DefaultMaxSize   = 5_000_000 - 1_000
	DefaultMaxWidth  = 2000
	DefaultMaxHeight = 2000
)

type Config struct {
	Directory string
	Limits    operation.ViewImageConfig
}

type translator struct {
	config Config
}

func New(config Config) tool.Translator {
	if config.Limits.MaxSize == 0 {
		config.Limits.MaxSize = DefaultMaxSize
	}
	if config.Limits.MaxWidth == 0 {
		config.Limits.MaxWidth = DefaultMaxWidth
	}
	if config.Limits.MaxHeight == 0 {
		config.Limits.MaxHeight = DefaultMaxHeight
	}
	return &translator{config: config}
}

func (translator *translator) Translate(ctx tool.Context, call llm.ToolCall) tool.CallStatus {
	var arguments struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(call.Arguments), &arguments); err != nil {
		return tool.ErrorStatus(fmt.Sprintf("decode ViewImage arguments: %v", err), operation.DefaultMaxOutputLength)
	}
	if strings.TrimSpace(arguments.Path) == "" {
		return tool.CallStatus{Error: `ViewImage argument "path" must be set`}
	}
	if offset := strings.IndexByte(arguments.Path, 0); offset >= 0 {
		return tool.CallStatus{Error: fmt.Sprintf(`ViewImage argument "path" contains a NUL byte at offset %d`, offset)}
	}
	path := arguments.Path
	if translator.config.Directory != "" && !filepath.IsAbs(path) {
		// Preserve .. for filesystem resolution across symlinks; filepath.Join would clean it.
		path = translator.config.Directory + string(filepath.Separator) + path
	}
	spec, err := operation.NewViewImageSpec(path, translator.config.Limits)
	if err != nil {
		return tool.CallStatus{Error: fmt.Sprintf("build ViewImage operation: %v", err)}
	}
	return tool.CallStatus{WaitingFor: []operation.ID{ctx.Submit(spec)}}
}

type Result struct {
	CallID  string
	Running bool
	Image   *operation.ViewImageResult
	Error   string
}

func (result Result) ToLLMResult() llm.ToolResult {
	output := llm.ToolResult{CallID: result.CallID, Running: result.Running}
	var details []string
	if result.Running {
		details = append(details, "Image is still loading.")
	} else {
		failed := result.Error != ""
		if failed {
			details = append(details, "Error: "+result.Error)
		}
		if image := result.Image; image != nil {
			if !failed {
				output.Output = append(output.Output, llm.ToolResultOutput{
					Kind: llm.ToolResultImage, Value: "data:" + image.EncodedMIMEType + ";base64," + image.Content,
				})
			}
			if image.OriginalMIMEType != "" && (failed || image.OriginalMIMEType != image.EncodedMIMEType) {
				details = append(details, "original MIME type: "+image.OriginalMIMEType)
			}
			if image.OriginalWidth > 0 && image.OriginalHeight > 0 && (failed || image.ScaleRatio < 1) {
				details = append(details, fmt.Sprintf("original dimensions: %dx%d", image.OriginalWidth, image.OriginalHeight))
			}
			if !failed && image.ScaleRatio < 1 {
				details = append(details, fmt.Sprintf("multiply coordinates by %.2f to approximate original", 1/image.ScaleRatio))
			}
		}
	}
	if len(details) != 0 {
		output.Output = append(output.Output, llm.ToolResultOutput{Kind: llm.ToolResultText, Value: strings.Join(details, "; ")})
	}
	return output
}

func (translator *translator) TranslateResult(
	callID string,
	status tool.CallStatus,
	operations []operation.Operation,
) (tool.Result, error) {
	if status.Error != "" {
		if len(operations) != 0 {
			return nil, fmt.Errorf("ViewImage call %q has both a validation error and operations", callID)
		}
		return Result{CallID: callID, Error: status.Error}, nil
	}
	if len(operations) != 1 {
		return nil, fmt.Errorf("ViewImage call %q has %d operations, want 1", callID, len(operations))
	}
	current := operations[0]
	state, err := operation.DecodeViewImageState(current)
	if err != nil {
		return nil, fmt.Errorf("decode ViewImage call %q result: %w", callID, err)
	}
	result := Result{CallID: callID, Image: state.Result}
	switch current.Status {
	case operation.StatusReady, operation.StatusAwaiting, operation.StatusCanceling:
		result.Running = true
		return result, nil
	case operation.StatusFailed, operation.StatusCanceled:
		result.Error = "view-image operation " + string(current.Status)
		if state.Result != nil && state.Result.Error != "" {
			result.Error = state.Result.Error
		}
	case operation.StatusCompleted:
		if state.Result == nil || state.Result.Content == "" || state.Result.EncodedMIMEType == "" ||
			state.Result.ScaleRatio <= 0 || state.Result.ScaleRatio > 1 || state.Result.Error != "" {
			return nil, fmt.Errorf("ViewImage call %q completed operation %q has an invalid image result", callID, current.ID)
		}
	default:
		return nil, fmt.Errorf("ViewImage call %q operation %q has invalid status %q", callID, current.ID, current.Status)
	}
	return result, nil
}
