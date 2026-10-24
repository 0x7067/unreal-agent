package operation

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Preparation happens in UpdateRemoteJob, before handlers publish a checkpoint.
// The current checkpoint is also the authority for the artifact configuration:
// handlers cannot accidentally drop it by constructing a fresh state value.
func prepareRemoteJobOutput(current Operation, previous RemoteJobState, state *RemoteJobState) {
	state.OutputDirectory = previous.OutputDirectory
	prepareRemoteJobField(current, state.OutputDirectory, "result", &state.TerminalResult, &state.ResultBytes, &state.ResultTruncated, &state.ResultPath,
		previous.TerminalResult, previous.ResultBytes, previous.ResultTruncated, previous.ResultPath)
	prepareRemoteJobField(current, state.OutputDirectory, "error", &state.TerminalError, &state.ErrorBytes, &state.ErrorTruncated, &state.ErrorPath,
		previous.TerminalError, previous.ErrorBytes, previous.ErrorTruncated, previous.ErrorPath)
}

func prepareRemoteJobField(current Operation, directory, kind string, text *string, size *int, truncated *bool, path *string,
	previous string, previousSize int, previousTruncated bool, previousPath string) {
	if *text != "" && previousTruncated && *text == previous {
		// Even a caller resetting the flag must not archive a checkpoint's preview.
		*size, *truncated, *path = previousSize, true, previousPath
		return
	}
	*path = "" // A different payload must never advertise the previous artifact.
	if *text != "" && *truncated {
		// An already lossy value (e.g. an old session) is not the original output.
		// Never write it to a file advertised as complete.
		*text, _ = BoundOutput(*text, current.MaxOutputLength)
		if directory != "" {
			*text += " [complete output unavailable: original output not retained]"
		}
		return
	}
	*size = len(*text)
	*truncated = utf8.RuneCountInString(*text) > current.MaxOutputLength
	if !*truncated {
		*text, _ = BoundOutput(*text, current.MaxOutputLength)
		return
	}
	if directory != "" {
		var err error
		*path, err = writeRemoteJobArtifact(directory, current.ID, kind, *text)
		if err != nil {
			*path = ""
			*text, _ = BoundOutput(*text, current.MaxOutputLength)
			*text += " [complete output unavailable: artifact could not be saved]"
			return
		}
	}
	*text, _ = boundOutput(*text, "", int64(*size), current.MaxOutputLength, *path)
}

func remoteJobArtifactName(id ID, kind, text string) (string, error) {
	if len(id) == 0 || len(id) > 128 || id == "." || id == ".." {
		return "", fmt.Errorf("invalid remote job operation ID")
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("._-", c)) {
			return "", fmt.Errorf("invalid remote job operation ID")
		}
	}
	if kind != "result" && kind != "error" {
		return "", fmt.Errorf("invalid remote job output kind")
	}
	return fmt.Sprintf("%x-%s-%x.txt", sha256.Sum256([]byte(id)), kind, sha256.Sum256([]byte(text))), nil
}

func writeRemoteJobArtifact(directory string, id ID, kind, text string) (string, error) {
	name, err := remoteJobArtifactName(id, kind, text)
	if err != nil {
		return "", err
	}
	directory, err = filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	directory = filepath.Join(directory, "remote-outputs")
	if err := saveRemoteJobArtifact(directory, name, text); err != nil {
		return "", err
	}
	return filepath.Join(directory, name), nil
}
