package githook

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

type refUpdate struct {
	oldValue string
	newValue string
	refName  string
}

func readRefUpdates(ctx context.Context, reader io.Reader) ([]refUpdate, error) {
	if reader == nil {
		return nil, nil
	}
	scanner := bufio.NewScanner(reader)
	updates := make([]refUpdate, 0)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		if line == "" {
			continue
		}
		oldValue, rest, found := strings.Cut(line, " ")
		if !found {
			return nil, fmt.Errorf("reference-transaction line %d: missing new value", lineNumber)
		}
		newValue, refName, found := strings.Cut(rest, " ")
		if !found {
			return nil, fmt.Errorf("reference-transaction line %d: missing ref name", lineNumber)
		}
		updates = append(updates, refUpdate{
			oldValue: oldValue,
			newValue: newValue,
			refName:  refName,
		})
	}
	if err := scanner.Err(); err != nil {
		slog.WarnContext(ctx, "reading reference-transaction", "err", err)
		return nil, fmt.Errorf("reading reference-transaction: %w", err)
	}
	return updates, nil
}
