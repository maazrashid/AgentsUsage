package parser

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
)

const maxJSONLLineBytes = 16 * 1024 * 1024
const streamAnchorBytes = 4 * 1024

type streamCursor struct {
	offset     int64
	pending    []byte
	discarding bool
	anchor     []byte
}

type tailScanResult struct {
	cursor         streamCursor
	bytesRead      int64
	oversizedLines int
}

// scanJSONLTail reads from the cursor offset and emits complete JSONL records.
// An incomplete final record stays in memory until a later append completes it.
func scanJSONLTail(ctx context.Context, path string, cursor streamCursor, visit func([]byte)) (tailScanResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return tailScanResult{}, err
	}
	defer file.Close()
	if _, err := file.Seek(cursor.offset, io.SeekStart); err != nil {
		return tailScanResult{}, err
	}

	pending := append([]byte(nil), cursor.pending...)
	discarding := cursor.discarding
	result := tailScanResult{cursor: streamCursor{offset: cursor.offset}}
	reader := bufio.NewReaderSize(file, 64*1024)
	for {
		if err := ctx.Err(); err != nil {
			return tailScanResult{}, err
		}
		fragment, readErr := reader.ReadSlice('\n')
		result.bytesRead += int64(len(fragment))
		hasNewline := len(fragment) > 0 && fragment[len(fragment)-1] == '\n'

		if discarding {
			if hasNewline {
				discarding = false
			}
		} else if len(fragment) > 0 {
			content := fragment
			if hasNewline {
				content = content[:len(content)-1]
			}
			if len(pending)+len(content) > maxJSONLLineBytes {
				result.oversizedLines++
				pending = nil
				discarding = !hasNewline
			} else {
				pending = append(pending, content...)
				if hasNewline {
					pending = trimCarriageReturn(pending)
					if len(pending) > 0 {
						visit(pending)
					}
					pending = pending[:0]
				}
			}
		}

		switch {
		case readErr == nil:
			continue
		case errors.Is(readErr, bufio.ErrBufferFull):
			continue
		case errors.Is(readErr, io.EOF):
			if !discarding && len(pending) > 0 && json.Valid(pending) {
				visit(pending)
				pending = nil
			}
			result.cursor.offset = cursor.offset + result.bytesRead
			result.cursor.pending = append([]byte(nil), pending...)
			result.cursor.discarding = discarding
			result.cursor.anchor, err = readStreamAnchor(file, result.cursor.offset)
			if err != nil {
				return tailScanResult{}, err
			}
			return result, nil
		default:
			return tailScanResult{}, readErr
		}
	}
}

// cursorMatchesFile detects in-place rewrites before treating growth as an
// append. Reading a short window at the old EOF keeps the check independent of
// total log size while covering the region most likely to change on rewrite.
func cursorMatchesFile(path string, cursor streamCursor) (bool, error) {
	if cursor.offset == 0 {
		return true, nil
	}
	if len(cursor.anchor) == 0 {
		return false, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	current, err := readStreamAnchor(file, cursor.offset)
	if err != nil {
		return false, err
	}
	return bytes.Equal(current, cursor.anchor), nil
}

func readStreamAnchor(file *os.File, offset int64) ([]byte, error) {
	length := int64(streamAnchorBytes)
	if offset < length {
		length = offset
	}
	if length == 0 {
		return nil, nil
	}
	anchor := make([]byte, int(length))
	if _, err := file.ReadAt(anchor, offset-length); err != nil {
		return nil, err
	}
	return anchor, nil
}

func trimCarriageReturn(value []byte) []byte {
	if len(value) > 0 && value[len(value)-1] == '\r' {
		return value[:len(value)-1]
	}
	return value
}
