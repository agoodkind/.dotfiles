package protobuf

import (
	"fmt"

	"goodkind.io/.dotfiles/internal/cursor/constants"
	"goodkind.io/.dotfiles/internal/cursor/models"
)

func EncodeVarint(value int) []byte {
	encodedBytes := []byte{}
	remainingValue := value
	for remainingValue > constants.VarintDataMask {
		encodedBytes = append(encodedBytes, byte((remainingValue&constants.VarintDataMask)|constants.VarintContinuationBit))
		remainingValue >>= 7
	}
	encodedBytes = append(encodedBytes, byte(remainingValue&constants.VarintDataMask))
	return encodedBytes
}

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

func EncodeBytesField(fieldNumber int, value string) []byte {
	payloadBytes := []byte(value)
	fieldTag := (fieldNumber << 3) | constants.WireTypeLength
	return append(append([]byte{byte(fieldTag)}, EncodeVarint(len(payloadBytes))...), payloadBytes...)
}

func ParseField(data []byte, position int) models.ParsedField {
	if position >= len(data) {
		return models.ParsedField{
			NextPosition: position,
		}
	}
	tag, nextPosition := ReadVarint(data, position)
	fieldNumber := tag >> 3
	wireType := tag & constants.WireTypeMask

	if wireType == constants.WireTypeVarint {
		integerValue, endPosition := ReadVarint(data, nextPosition)
		return models.ParsedField{
			FieldNumber:  fieldNumber,
			WireType:     wireType,
			Value:        integerValue,
			NextPosition: endPosition,
			Ok:           true,
		}
	}

	if wireType == constants.WireTypeLength {
		payloadLength, endPosition := ReadVarint(data, nextPosition)
		payloadStart := endPosition
		payloadEnd := payloadStart + payloadLength
		if payloadEnd > len(data) {
			payloadEnd = len(data)
		}
		bytesValue := data[payloadStart:payloadEnd]
		return models.ParsedField{
			FieldNumber:  fieldNumber,
			WireType:     wireType,
			Value:        bytesValue,
			NextPosition: payloadEnd,
			Ok:           true,
		}
	}

	return models.ParsedField{
		NextPosition: len(data),
	}
}

func AddFieldValue(fields models.ParsedMessage, fieldNumber int, value interface{}) {
	existingValue, exists := fields[fieldNumber]
	if !exists {
		fields[fieldNumber] = value
		return
	}
	if currentSlice, ok := existingValue.([]interface{}); ok {
		fields[fieldNumber] = append(currentSlice, value)
		return
	}
	fields[fieldNumber] = []interface{}{existingValue, value}
}

func ParseMessage(data []byte) models.ParsedMessage {
	fields := models.ParsedMessage{}
	cursorPosition := 0
	for cursorPosition < len(data) {
		field := ParseField(data, cursorPosition)
		if !field.Ok || field.Value == nil {
			break
		}
		AddFieldValue(fields, field.FieldNumber, field.Value)
		cursorPosition = field.NextPosition
	}
	return fields
}

func GetBytesField(fields models.ParsedMessage, fieldNumber int) []byte {
	fieldValue, found := fields[fieldNumber]
	if !found {
		return nil
	}
	bytesValue, ok := fieldValue.([]byte)
	if !ok {
		return nil
	}
	return bytesValue
}

func DecodeBytesField(value []byte, defaultValue string) string {
	if value == nil {
		return defaultValue
	}
	return string(value)
}

func Dump(fields models.ParsedMessage) string {
	return fmt.Sprintf("%v", fields)
}
