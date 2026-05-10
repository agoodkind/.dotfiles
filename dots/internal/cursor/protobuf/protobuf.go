// Package protobuf implements protobuf encoding for Cursor API requests.
package protobuf

import (
	"goodkind.io/.dotfiles/internal/cursor/constants"
	"goodkind.io/.dotfiles/internal/cursor/models"
)

// EncodeVarint encodes an integer as a protobuf varint byte sequence.
func EncodeVarint(value int) []byte {
	encodedBytes := []byte{}
	remainingValue := value
	for remainingValue > constants.VarintDataMask {
		byteVal := min((remainingValue&constants.VarintDataMask)|constants.VarintContinuationBit, 255)
		if byteVal < 0 || byteVal > 255 {
			byteVal = 255
		}
		encodedBytes = append(encodedBytes, byte(byteVal))
		remainingValue >>= 7
	}
	finalByte := min(remainingValue&constants.VarintDataMask, 255)
	encodedBytes = append(encodedBytes, byte(finalByte))
	return encodedBytes
}

// ReadVarint reads a varint from data at position and returns the decoded value and the next byte position.
func ReadVarint(data []byte, position int) (int, int) {
	decodedValue := 0
	shiftCount := 0
	cursorPosition := position
	for cursorPosition < len(data) {
		currentByte := data[cursorPosition]
		cursorPosition++
		decodedValue |= int(currentByte&constants.VarintDataMask) << shiftCount
		if currentByte&constants.VarintContinuationBit == 0 {
			break
		}
		shiftCount += 7
	}
	return decodedValue, cursorPosition
}

// EncodeBytesField encodes a string as a protobuf length-delimited field with the given field number.
func EncodeBytesField(fieldNumber int, value string) []byte {
	payloadBytes := []byte(value)
	fieldTag := (fieldNumber << 3) | constants.WireTypeLength
	return append(append(EncodeVarint(fieldTag), EncodeVarint(len(payloadBytes))...), payloadBytes...)
}

// ParseField parses a single protobuf field from data starting at position.
func ParseField(data []byte, position int) models.ParsedField {
	if position >= len(data) {
		return models.ParsedField{
			FieldNumber: 0,
			WireType:    0,
			Value: models.ParsedValue{
				Kind:    models.ParsedValueInvalid,
				Integer: 0,
				Bytes:   nil,
				List:    nil,
			},
			NextPosition: position,
			Ok:           false,
		}
	}
	tag, nextPosition := ReadVarint(data, position)
	fieldNumber := tag >> 3
	wireType := tag & constants.WireTypeMask

	if wireType == constants.WireTypeVarint {
		integerValue, endPosition := ReadVarint(data, nextPosition)
		return models.ParsedField{
			FieldNumber: fieldNumber,
			WireType:    wireType,
			Value: models.ParsedValue{
				Kind:    models.ParsedValueInteger,
				Integer: integerValue,
				Bytes:   nil,
				List:    nil,
			},
			NextPosition: endPosition,
			Ok:           true,
		}
	}

	if wireType == constants.WireTypeLength {
		payloadLength, endPosition := ReadVarint(data, nextPosition)
		payloadStart := endPosition
		payloadEnd := payloadStart + payloadLength
		payloadEnd = min(payloadEnd, len(data))
		bytesValue := data[payloadStart:payloadEnd]
		return models.ParsedField{
			FieldNumber: fieldNumber,
			WireType:    wireType,
			Value: models.ParsedValue{
				Kind:    models.ParsedValueBytes,
				Integer: 0,
				Bytes:   bytesValue,
				List:    nil,
			},
			NextPosition: payloadEnd,
			Ok:           true,
		}
	}

	return models.ParsedField{
		FieldNumber: 0,
		WireType:    0,
		Value: models.ParsedValue{
			Kind:    models.ParsedValueInvalid,
			Integer: 0,
			Bytes:   nil,
			List:    nil,
		},
		NextPosition: len(data),
		Ok:           false,
	}
}

// AddFieldValue sets or appends value to fields at fieldNumber, promoting to a list on collision.
func AddFieldValue(fields models.ParsedMessage, fieldNumber int, value models.ParsedValue) {
	existingValue, exists := fields[fieldNumber]
	if !exists {
		fields[fieldNumber] = value
		return
	}
	if existingValue.Kind == models.ParsedValueList {
		existingValue.List = append(existingValue.List, value)
		fields[fieldNumber] = existingValue
		return
	}
	fields[fieldNumber] = models.ParsedValue{
		Kind:    models.ParsedValueList,
		Integer: 0,
		Bytes:   nil,
		List:    []models.ParsedValue{existingValue, value},
	}
}

// ParseMessage decodes a protobuf byte slice into a ParsedMessage field map.
func ParseMessage(data []byte) models.ParsedMessage {
	fields := models.ParsedMessage{}
	cursorPosition := 0
	for cursorPosition < len(data) {
		field := ParseField(data, cursorPosition)
		if !field.Ok || field.Value.Kind == models.ParsedValueInvalid {
			break
		}
		AddFieldValue(fields, field.FieldNumber, field.Value)
		cursorPosition = field.NextPosition
	}
	return fields
}

// GetBytesField returns the raw bytes stored at fieldNumber in fields, or nil if absent.
func GetBytesField(fields models.ParsedMessage, fieldNumber int) []byte {
	fieldValue, found := fields[fieldNumber]
	if !found || fieldValue.Kind != models.ParsedValueBytes {
		return nil
	}
	return fieldValue.Bytes
}

// DecodeBytesField converts a bytes field to a string, returning defaultValue when value is nil.
func DecodeBytesField(value []byte, defaultValue string) string {
	if value == nil {
		return defaultValue
	}
	return string(value)
}
