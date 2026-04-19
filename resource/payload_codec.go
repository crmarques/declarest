// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resource

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/crmarques/declarest/faults"
	"go.yaml.in/yaml/v3"
)

const (
	PayloadTypeJSON        = "json"
	PayloadTypeYAML        = "yaml"
	PayloadTypeXML         = "xml"
	PayloadTypeHCL         = "hcl"
	PayloadTypeINI         = "ini"
	PayloadTypeProperties  = "properties"
	PayloadTypeText        = "text"
	PayloadTypeOctetStream = "octet-stream"
	PayloadTypeBinary      = "binary"
)

type BinaryValue struct {
	Bytes []byte
}

type PayloadCodec struct {
	Type       string
	Extension  string
	MediaType  string
	Structured bool
	Text       bool
	Binary     bool
}

var payloadCodecs = []PayloadCodec{
	{Type: PayloadTypeJSON, Extension: ".json", MediaType: "application/json", Structured: true},
	{Type: PayloadTypeYAML, Extension: ".yaml", MediaType: "application/yaml", Structured: true},
	{Type: PayloadTypeINI, Extension: ".ini", MediaType: "application/ini", Structured: true},
	{Type: PayloadTypeProperties, Extension: ".properties", MediaType: "text/x-java-properties", Structured: true},
	{Type: PayloadTypeXML, Extension: ".xml", MediaType: "application/xml", Text: true},
	{Type: PayloadTypeHCL, Extension: ".hcl", MediaType: "application/hcl", Text: true},
	{Type: PayloadTypeText, Extension: ".txt", MediaType: "text/plain", Text: true},
	{Type: PayloadTypeOctetStream, Extension: ".bin", MediaType: "application/octet-stream", Binary: true},
}

var payloadCodecByType = buildPayloadCodecByType()

func NormalizePayloadType(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "", PayloadTypeJSON:
		return PayloadTypeJSON
	case PayloadTypeBinary:
		return PayloadTypeOctetStream
	default:
		return normalized
	}
}

func ValidatePayloadType(value string) (string, error) {
	normalized := NormalizePayloadType(value)
	if _, ok := payloadCodecByType[normalized]; ok {
		return normalized, nil
	}

	return "", faults.Invalid(
		fmt.Sprintf("unsupported payload type %q", strings.TrimSpace(value)),
		nil,
	)
}

func SupportedPayloadTypes() []string {
	values := make([]string, 0, len(payloadCodecs))
	for _, codec := range payloadCodecs {
		values = append(values, codec.Type)
	}
	return values
}

func PayloadCodecForType(value string) (PayloadCodec, error) {
	normalized, err := ValidatePayloadType(value)
	if err != nil {
		return PayloadCodec{}, err
	}
	return payloadCodecByType[normalized], nil
}

func PayloadTypeForExtension(extension string) (string, bool) {
	descriptor, ok := PayloadDescriptorForExtension(extension)
	if !ok {
		return "", false
	}
	return descriptor.PayloadType, true
}

func PayloadTypeForMediaType(value string) (string, bool) {
	descriptor, ok := PayloadDescriptorForContentType(value)
	if !ok {
		return "", false
	}
	return descriptor.PayloadType, true
}

func PayloadMediaType(value string) (string, error) {
	codec, err := PayloadCodecForType(value)
	if err != nil {
		return "", err
	}
	return codec.MediaType, nil
}

func PayloadExtension(value string) (string, error) {
	codec, err := PayloadCodecForType(value)
	if err != nil {
		return "", err
	}
	return codec.Extension, nil
}

func IsStructuredPayloadType(value string) bool {
	codec, err := PayloadCodecForType(value)
	if err != nil {
		return false
	}
	return codec.Structured
}

func IsTextPayloadType(value string) bool {
	codec, err := PayloadCodecForType(value)
	if err != nil {
		return false
	}
	return codec.Text
}

func IsBinaryPayloadType(value string) bool {
	codec, err := PayloadCodecForType(value)
	if err != nil {
		return false
	}
	return codec.Binary
}

func EncodePayload(value Value, payloadType string) ([]byte, error) {
	return encodePayload(value, payloadType, false)
}

func EncodePayloadPretty(value Value, payloadType string) ([]byte, error) {
	return encodePayload(value, payloadType, true)
}

func DecodePayload(data []byte, payloadType string) (Value, error) {
	codec, err := PayloadCodecForType(payloadType)
	if err != nil {
		return nil, err
	}

	switch {
	case codec.Structured:
		return decodeStructuredPayload(data, codec.Type)
	case codec.Text:
		return string(data), nil
	case codec.Binary:
		return BinaryValue{Bytes: append([]byte(nil), data...)}, nil
	default:
		return nil, faults.Invalid(
			fmt.Sprintf("unsupported payload type %q", payloadType),
			nil,
		)
	}
}

func EncodeContent(content Content) ([]byte, error) {
	descriptor := NormalizePayloadDescriptor(content.Descriptor)
	return EncodePayload(content.Value, descriptor.PayloadType)
}

func EncodeContentPretty(content Content) ([]byte, error) {
	descriptor := NormalizePayloadDescriptor(content.Descriptor)
	return EncodePayloadPretty(content.Value, descriptor.PayloadType)
}

func DecodeContent(data []byte, descriptor PayloadDescriptor) (Content, error) {
	resolved := NormalizePayloadDescriptor(descriptor)
	value, err := DecodePayload(data, resolved.PayloadType)
	if err != nil {
		return Content{}, err
	}
	return Content{Value: value, Descriptor: resolved}, nil
}

func encodePayload(value Value, payloadType string, pretty bool) ([]byte, error) {
	codec, err := PayloadCodecForType(payloadType)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}

	switch {
	case codec.Structured:
		return encodeStructuredPayload(value, codec.Type, pretty)
	case codec.Text:
		return encodeTextPayload(value, codec.Type)
	case codec.Binary:
		return encodeBinaryPayload(value)
	default:
		return nil, faults.Invalid(
			fmt.Sprintf("unsupported payload type %q", payloadType),
			nil,
		)
	}
}

func IsBinaryValue(value any) bool {
	switch typed := value.(type) {
	case BinaryValue:
		return true
	case *BinaryValue:
		return typed != nil
	default:
		return false
	}
}

func CloneBinaryValue(value BinaryValue) BinaryValue {
	return BinaryValue{Bytes: append([]byte(nil), value.Bytes...)}
}

func BinaryBytes(value any) ([]byte, bool) {
	switch typed := value.(type) {
	case BinaryValue:
		return append([]byte(nil), typed.Bytes...), true
	case *BinaryValue:
		if typed == nil {
			return nil, false
		}
		return append([]byte(nil), typed.Bytes...), true
	case []byte:
		return append([]byte(nil), typed...), true
	default:
		return nil, false
	}
}

func decodeStructuredPayload(data []byte, payloadType string) (Value, error) {
	switch payloadType {
	case PayloadTypeYAML:
		var decoded any
		if err := yaml.Unmarshal(data, &decoded); err != nil {
			return nil, faults.Invalid("invalid yaml payload", err)
		}
		return Normalize(decoded)
	case PayloadTypeINI:
		decoded, err := decodeINIPayload(data)
		if err != nil {
			return nil, err
		}
		return Normalize(decoded)
	case PayloadTypeProperties:
		decoded, err := decodePropertiesPayload(data)
		if err != nil {
			return nil, err
		}
		return Normalize(decoded)
	default:
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()

		var decoded any
		if err := decoder.Decode(&decoded); err != nil {
			return nil, faults.Invalid("invalid json payload", err)
		}
		return Normalize(decoded)
	}
}

func encodeStructuredPayload(value Value, payloadType string, pretty bool) ([]byte, error) {
	normalized, err := Normalize(value)
	if err != nil {
		return nil, err
	}

	switch payloadType {
	case PayloadTypeYAML:
		encoded, err := yaml.Marshal(normalized)
		if err != nil {
			return nil, faults.Invalid("failed to encode yaml payload", err)
		}
		return encoded, nil
	case PayloadTypeINI:
		return encodeINIPayload(normalized)
	case PayloadTypeProperties:
		return encodePropertiesPayload(normalized)
	default:
		if pretty {
			encoded, err := json.MarshalIndent(normalized, "", "  ")
			if err != nil {
				return nil, faults.Invalid("failed to encode json payload", err)
			}
			return encoded, nil
		}
		encoded, err := json.Marshal(normalized)
		if err != nil {
			return nil, faults.Invalid("failed to encode json payload", err)
		}
		return encoded, nil
	}
}

func encodeTextPayload(value Value, payloadType string) ([]byte, error) {
	switch typed := value.(type) {
	case string:
		return []byte(typed), nil
	case []byte:
		return append([]byte(nil), typed...), nil
	default:
		return nil, faults.Invalid(
			fmt.Sprintf("payload type %q requires string input", payloadType),
			nil,
		)
	}
}

func encodeBinaryPayload(value Value) ([]byte, error) {
	bytesValue, ok := BinaryBytes(value)
	if !ok {
		return nil, faults.Invalid("payload type \"octet-stream\" requires resource.BinaryValue input", nil)
	}
	return bytesValue, nil
}

func buildPayloadCodecByType() map[string]PayloadCodec {
	result := make(map[string]PayloadCodec, len(payloadCodecs))
	for _, codec := range payloadCodecs {
		result[codec.Type] = codec
	}
	return result
}
