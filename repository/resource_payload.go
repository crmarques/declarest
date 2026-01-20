package repository

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/crmarques/declarest/resource"
)

func decodeResourcePayload(data []byte, format ResourceFormat) (resource.Resource, error) {
	switch normalizeResourceFormat(format) {
	case ResourceFormatYAML:
		return resource.NewResourceFromYAML(data)
	default:
		return resource.NewResourceFromJSON(data)
	}
}

func encodeResourcePayload(res resource.Resource, format ResourceFormat) ([]byte, error) {
	switch normalizeResourceFormat(format) {
	case ResourceFormatYAML:
		return res.MarshalYAMLBytes()
	default:
		data, err := res.MarshalJSON()
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		if err := json.Indent(&buf, data, "", "  "); err != nil {
			return nil, fmt.Errorf("failed to format resource payload: %w", err)
		}
		buf.WriteByte('\n')
		return buf.Bytes(), nil
	}
}
